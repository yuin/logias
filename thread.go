package main

import (
	"fmt"
	"github.com/yuin/gopher-lua"
	"os"
	"sync"
)

type shared struct {
	logger *logger
	quitc  chan *sync.WaitGroup
}

type thread struct {
	L            *lua.LState
	config       *config
	shared       *shared
	luaUd        *lua.LUserData
	isInDowntime bool
}

func mustNewThread(path string, s *shared) *thread {
	th := &thread{lua.NewState(), nil, s, nil, false}
	th.luaUd = th.L.NewUserData()
	th.beforeLoadConfig()
	cfg, err := loadConfig(th.L, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can not load %s:\n\n%s", path, err.Error())
		os.Exit(1)
	}
	th.config = cfg
	th.afterLoadConfig()
	return th
}

func (th *thread) beforeLoadConfig() {
	if err := th.L.DoString(luaConstants); err != nil {
		panic(err)
	}
	registerNqueueType(th.L)
	th.L.SetFuncs(th.L.Get(lua.GlobalsIndex).(*lua.LTable), luaFunctions, th.luaUd)
}

func (th *thread) afterLoadConfig() {
	th.luaUd.Value = th
}

func (th *thread) callLua(fn *lua.LFunction, nret int, args ...lua.LValue) error {
	return th.L.CallByParam(lua.P{Fn: fn, NRet: nret, Protect: true}, args...)
}

func (th *thread) popLuaRet() lua.LValue {
	ret := th.L.Get(-1)
	th.L.Pop(1)
	return ret
}

func (th *thread) checkDowntime() {
	fn := th.config.Downtime
	if lua.LVIsFalse(fn) {
		th.isInDowntime = false
		return
	}
	if err := th.callLua(fn, 1); err != nil {
		th.systemError(logLevelError.String(), "error while calling the downtime function: %s", err.Error())
		th.isInDowntime = false
	}
	th.isInDowntime = lua.LVAsBool(th.popLuaRet())
}

func (th *thread) systemError(level, format string, args ...interface{}) {
	fn := th.config.Downtime
	if !lua.LVIsFalse(fn) {
		if err := th.callLua(fn, 1); err == nil {
			if lua.LVAsBool(th.popLuaRet()) {
				return
			}
		}
		// ignore errors while calling the downtime function
	}

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	th.callLua(th.config.OnSystemError, 1, lua.LString(level), lua.LString(msg))
}
