package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type H map[string]interface{}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type fileType int

const (
	ftFile fileType = iota
	ftDir
	ftLink
	ftNotExists
	ftOther
)

func fileStat(path string) fileType {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ftNotExists
		}
		return ftOther
	}

	if (fi.Mode() & os.ModeSymlink) == os.ModeSymlink {
		return ftLink
	}
	if fi.IsDir() {
		return ftDir
	}
	return ftFile
}

func isDir(path string) bool { return fileStat(path) == ftDir }

func isFile(path string) bool { return fileStat(path) == ftFile }

func pathExists(path string) bool { return fileStat(path) != ftNotExists }

func ensureDirExists(path string) error {
	dir := filepath.Dir(path)
	if !pathExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFile(data string, path string) error {
	if err := ensureDirExists(path); err != nil {
		return err
	}
	if err := ioutil.WriteFile(path, ([]byte)(data), 0755); err != nil {
		return err
	}
	return nil
}

const popenError = 500

func popenArgs(arg string) (string, []string) {
	cmd := "/bin/sh"
	args := []string{"-c"}
	if runtime.GOOS == "windows" {
		cmd = "C:\\Windows\\system32\\cmd.exe"
		args = []string{"/c"}
	}
	args = append(args, arg)
	return cmd, args
}

func exitStatus(err error) int {
	if err != nil {
		if e2, ok := err.(*exec.ExitError); ok {
			if s, ok := e2.Sys().(syscall.WaitStatus); ok {
				return s.ExitStatus()
			}
			return 1
		}
	}
	return 0
}

func shellStdout(cmd string) (int, string) {
	c, args := popenArgs(cmd)
	pp := exec.Command(c, args...)
	var out bytes.Buffer
	pp.Stdout = &out
	err := pp.Start()
	if err != nil {
		return popenError, err.Error()
	}
	err = pp.Wait()
	if err != nil {
		return exitStatus(err), err.Error()
	}
	return 0, out.String()
}

func parseNumber(number string) (float64, error) {
	number = strings.Trim(number, " \t\n")
	if v, err := strconv.ParseInt(number, 0, 64); err != nil {
		if v2, err2 := strconv.ParseFloat(number, 64); err2 != nil {
			return 0, err2
		} else {
			return float64(v2), nil
		}
	} else {
		return float64(v), nil
	}
}
