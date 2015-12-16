package main

import (
	"bufio"
	"fmt"
	"github.com/yuin/gopher-lua"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type worker struct {
	*thread
	target *target
}

func newWorker(path string, fpath string, s *shared) *worker {
	wk := &worker{
		thread: mustNewThread(path, s),
	}
	wk.target = wk.config.Targets[fpath]
	wk.target.init(wk.L)
	return wk
}

func (wk *worker) run() {
	for {
		select {
		case wg := <-wk.shared.quitc:
			wk.shared.logger.info("worker stopped.")
			wg.Done()
			return
		case <-time.After(time.Duration(wk.target.Interval) * time.Second):
			olddt := wk.isInDowntime
			wk.checkDowntime()
			// clear state when downtime is closed
			if olddt && !wk.isInDowntime {
				wk.target.initState(wk.L)
			}

			switch wk.target.Type {
			case "FILE":
				wk.processFile()
			case "CMD":
				wk.processCmd()
			case "LUA":
				wk.processLua()
			}
		}
	}
}

func (wk *worker) processFile() {
	fd := wk.readFileData()
	if fileStat(wk.target.Path) == ftNotExists {
		fd.header = ""
		fd.position = 0
		wk.writeFileData(fd)
		return
	}

	fp, err := os.Open(wk.target.Path)
	if err != nil {
		wk.systemError(logLevelError.String(), "can not open %s: %s", wk.target.Path, err.Error())
		return
	}
	defer fp.Close()

	reader := bufio.NewReaderSize(fp, 4096)
	header, err := reader.ReadString('\n')

	if err == io.EOF {
		return
	}

	if err != nil {
		wk.systemError(logLevelError.String(), "can not read %s: %s", wk.target.Path, err.Error())
		return
	}

	header = strings.Trim(header, "\n")
	pos := fd.position

	fi, err := fp.Stat()
	if err != nil {
		wk.systemError(logLevelError.String(), "can not stat %s: %s", wk.target.Path, err.Error())
		return
	}

	if header != fd.header || pos > fi.Size() {
		wk.shared.logger.info("%s was truncated", wk.target.Path)
		pos = 0
	}

	if _, err := fp.Seek(pos, 0); err != nil {
		wk.systemError(logLevelError.String(), "can not seek %s: %s", wk.target.Path, err.Error())
		return
	}

	const maxread = 100
	for i := 0; i < maxread; i++ {
		line, err := readFileLine(fp)
		iseof := err == io.EOF
		if err != nil && !iseof {
			wk.systemError(logLevelError.String(), "can not read %s: %s", wk.target.Path, err.Error())
			return
		}
		line = strings.Trim(line, "\n")
		if len(line) > 0 {
			obj, ok := wk.applyParser(line)
			if !ok {
				break
			}
			for _, group := range wk.target.FilterGroups {
				wk.applyFilterGroup(line, group, obj)
			}
		}
		if iseof {
			break
		}
	}

	where, err := fp.Seek(0, 1)
	if err != nil {
		wk.systemError(logLevelError.String(), "can not seek %s: %s", wk.target.Path, err.Error())
		return
	}
	if fd.header == header && fd.position == where {
		return
	}

	fd.header = header
	fd.position = where
	wk.writeFileData(fd)
}

func (wk *worker) processCmd() {
	status, output := shellStdout(wk.target.Path)
	if status != 0 {
		wk.systemError(logLevelError.String(), "command failed %s: %s", wk.target.Path, output)
		return
	}
	output = strings.Trim(output, " \t\n")

	obj, ok := wk.applyParser(output)
	if !ok {
		return
	}

	for _, group := range wk.target.FilterGroups {
		wk.applyFilterGroup(output, group, obj)
	}
}

func (wk *worker) processLua() {
	if err := wk.callLua(wk.target.Fn, 1); err != nil {
		wk.systemError(logLevelError.String(), "error while calling the function %s: %s", wk.target.Path, err.Error())
		return
	}
	obj := wk.popLuaRet()
	output := luaToString(wk.L, obj)
	for _, group := range wk.target.FilterGroups {
		wk.applyFilterGroup(output, group, obj)
	}
}

func (wk *worker) applyParser(line string) (lua.LValue, bool) {
	obj := lua.LNil
	if !lua.LVIsFalse(wk.target.Parser) && wk.target.Parser != nil {
		if err := wk.callLua(wk.target.Parser, 1, lua.LString(line)); err != nil {
			wk.systemError(logLevelError.String(), "error while calling the parser function %s: %s", wk.target.Path, err.Error())
			return lua.LNil, false
		}
		obj = wk.popLuaRet()
	}
	return obj, true
}

func (wk *worker) applyFilterGroup(line string, filterGroup []*filter, obj lua.LValue) {
	for _, filter := range filterGroup {
		switch filter.Type {
		case "match":
			if !filter.regexp.MatchString(line) {
				return
			}
		case "notmatch":
			if filter.regexp.MatchString(line) {
				return
			}
		case "test":
			if err := wk.callLua(filter.Test, 1, wk.target.State, lua.LString(line), obj); err != nil {
				wk.systemError(logLevelError.String(), "error while calling the test function %s: %s", wk.target.Path, err.Error())
			}
			ret := wk.popLuaRet()
			if !lua.LVAsBool(ret) {
				return
			}
		case "action":
			if err := wk.callLua(filter.Fn, 0, wk.target.State, lua.LString(line), obj); err != nil {
				wk.systemError(logLevelError.String(), "error while calling the action function %s: %s", wk.target.Path, err.Error())
			}
		case "notify":
			message := filter.Message
			if len(message) == 0 {
				message = line
			}
			wk.notify(message, obj, filter.Level, filter.Code)
		}
	}
}

func (wk *worker) notify(message string, obj lua.LValue, level, code string) {
	if wk.isInDowntime {
		return
	}

	var fn *lua.LFunction
	if len(code) != 0 {
		if f, ok := wk.config.Notifiers.Code[code]; ok {
			fn = f
		}
	}
	if fn == nil && len(level) != 0 {
		if f, ok := wk.config.Notifiers.Level[level]; ok {
			fn = f
		}
	}
	if fn == nil {
		fn = wk.config.Notifiers.Default
	}
	if err := wk.callLua(fn, 0, wk.target.State, obj, lua.LString(message), lua.LString(level), lua.LString(code)); err != nil {
		wk.systemError(logLevelError.String(), "error while calling the notify function %s: %s", wk.target.Path, err.Error())
	}
}

func (wk *worker) writeFileData(fd *fileData) {
	path := filepath.Join(wk.config.StatDir, wk.target.dataPath())
	err := writeFile(fmt.Sprintf("%s\n%s\n%d", wk.target.Path, fd.header, fd.position), path)
	if err != nil {
		wk.systemError(logLevelError.String(), "failed to write the stat file %s: %s", path, err.Error())
	}
}

func (wk *worker) readFileData() *fileData {
	path := filepath.Join(wk.config.StatDir, wk.target.dataPath())
	switch fileStat(path) {
	case ftDir:
		wk.systemError(logLevelError.String(), "stat file %s is must be a file, not be a directory", path)
	case ftNotExists:
		return &fileData{
			header:   "",
			position: 0,
		}
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		wk.systemError(logLevelError.String(), "failed to read the stat file %s: %s", path, err.Error())
		return &fileData{
			header:   "",
			position: 0,
		}
	}

	lines := strings.Split(string(data), "\n")
	fd := &fileData{
		header:   lines[1],
		position: 0,
	}
	i, err := strconv.ParseInt(lines[2], 10, 64)
	if err == nil {
		fd.position = i
	}
	return fd
}
