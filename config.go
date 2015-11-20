package main

import (
	"github.com/yuin/gluamapper"
	"github.com/yuin/gopher-lua"
)

type config struct {
	StatDir       string
	LogFile       string
	LogLevel      string
	OnSystemError *lua.LFunction
	Downtime      *lua.LFunction

	Targets   map[string]*target
	Notifiers *notifiers
}

func loadConfig(L *lua.LState, path string) (*config, error) {
	cfg := config{}
	err := L.DoFile(path)
	if err != nil {
		return nil, err
	}
	lcfg := L.GetGlobal(appName).(*lua.LTable)
	err = gluamapper.Map(lcfg, &cfg)
	if err != nil {
		return nil, err
	}

	mapper := gluamapper.NewMapper(gluamapper.Option{NameFunc: gluamapper.Id})
	targets := luaMustGetTableAttr(lcfg, "targets")
	cfg.Targets = make(map[string]*target, len(cfg.Targets))
	targets.ForEach(func(key, value lua.LValue) {
		t := target{}
		gluamapper.Map(value.(*lua.LTable), &t)
		cfg.Targets[key.String()] = &t
		t.Path = key.String()
	})
	lnotifiers := luaMustGetTableAttr(lcfg, "notifiers")
	cfg.Notifiers.Default = lnotifiers.RawGetString("default").(*lua.LFunction)
	mapper.Map(luaMustGetTableAttr(lnotifiers, "code"), &cfg.Notifiers.Code)
	mapper.Map(luaMustGetTableAttr(lnotifiers, "level"), &cfg.Notifiers.Level)

	return &cfg, nil
}
