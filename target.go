package main

import (
	"crypto/sha1"
	"fmt"
	"github.com/yuin/gopher-lua"
	"path/filepath"
)

type target struct {
	Type         string
	Path         string
	Interval     int
	InitialState *lua.LFunction
	State        *lua.LTable
	Parser       *lua.LFunction
	Fn           *lua.LFunction
	FilterGroups [][]*filter

	_dataPath string
}

func (t *target) init(L *lua.LState) {
	for _, group := range t.FilterGroups {
		for _, filter := range group {
			filter.init()
		}
	}
	t.initState(L)
}

func (t *target) initState(L *lua.LState) {
	if err := L.CallByParam(lua.P{Fn: t.InitialState, NRet: 1, Protect: true}); err != nil {
		panic(err)
	}
	t.State = L.Get(-1).(*lua.LTable)
	L.Pop(1)
}

func (t *target) dataPath() string {
	if len(t._dataPath) == 0 {
		t._dataPath = fmt.Sprintf("%x_%s.txt", sha1.Sum([]byte(t.Path)), filepath.Base(t.Path))
	}
	return t._dataPath
}

type fileData struct {
	header   string
	position int64
}
