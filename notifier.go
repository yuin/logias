package main

import (
	"github.com/yuin/gopher-lua"
)

type notifiers struct {
	Default *lua.LFunction
	Level   map[string]*lua.LFunction
	Code    map[string]*lua.LFunction
}
