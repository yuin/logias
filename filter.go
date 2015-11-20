package main

import (
	"github.com/yuin/gopher-lua"
	"regexp"
)

type filter struct {
	Type    string
	Pattern string
	State   string
	Test    *lua.LFunction
	Fn      *lua.LFunction
	Level   string
	Code    string
	Message string

	regexp *regexp.Regexp
}

func (fil *filter) init() {
	if fil.Type == "match" || fil.Type == "notmatch" {
		fil.regexp = regexp.MustCompile(fil.Pattern)
	}
}
