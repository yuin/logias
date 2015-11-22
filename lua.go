package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/yuin/gluamapper"
	"github.com/yuin/gopher-lua"
	"io/ioutil"
	"net/smtp"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

var luaConstants = `
	  target = {
		FILE = "FILE",
		CMD  = "CMD",
		LUA  = "LUA"
	  }
	  loglevel = {
		DEBUG = "DEBUG",
		INFO  = "INFO",
		WARN  = "WARN",
		ERROR = "ERROR",
		CRIT  = "CRIT"
	  }

	  function match(tbl)
	    return {type="match", pattern=tostring(tbl.pattern or tbl[1])}
	  end

	  function notmatch(tbl)
	    return {type="notmatch", pattern=tostring(tbl.pattern or tbl[1])}
	  end

	  function test(tbl)
	    return {type="test", test=(tbl.test or tbl[1])}
	  end

	  function action(tbl)
	    return {type="action", fn=(tbl.fn or tbl[1])}
	  end

	  function notify(tbl)
	    return {type="notify", level=tostring(tbl.level), code=tostring(tbl.code), message=tostring(tbl.message)}
	  end
`

func callLFunc0(L *lua.LState, fn lua.LValue, args ...lua.LValue) {
	top := L.GetTop()
	L.Push(fn)
	for _, lv := range args {
		L.Push(lv)
	}
	L.Call(len(args), 0)
	L.SetTop(top)
}

func callLFunc1(L *lua.LState, fn lua.LValue, args ...lua.LValue) lua.LValue {
	top := L.GetTop()
	L.Push(fn)
	for _, lv := range args {
		L.Push(lv)
	}
	L.Call(len(args), 1)
	ret := L.Get(-1)
	L.SetTop(top)
	return ret
}

func callLFunc2(L *lua.LState, fn lua.LValue, args ...lua.LValue) (lua.LValue, lua.LValue) {
	top := L.GetTop()
	L.Push(fn)
	for _, lv := range args {
		L.Push(lv)
	}
	L.Call(len(args), 2)
	ret1 := L.Get(-2)
	ret2 := L.Get(-1)
	L.SetTop(top)
	return ret1, ret2
}

type nqueue struct {
	capa int
	d    []lua.LNumber
}

const nqueueName = "NQUEUE*"

func registerNqueueType(L *lua.LState) {
	mt := L.NewTypeMetatable(nqueueName)
	L.SetGlobal("nqueue", mt)
	L.SetField(mt, "new", L.NewFunction(newNqueue))
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), nqueueMethods))
	L.SetField(mt, "__len", L.NewFunction(nqueueLen))
}

func newNqueue(L *lua.LState) int {
	q := &nqueue{L.OptInt(1, 0), []lua.LNumber{}}
	ud := L.NewUserData()
	ud.Value = q
	L.SetMetatable(ud, L.GetTypeMetatable(nqueueName))
	L.Push(ud)
	return 1
}

func checkNqueue(L *lua.LState) (*nqueue, *lua.LUserData) {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*nqueue); ok {
		return v, ud
	}
	L.ArgError(1, "nqueue expected")
	return nil, nil
}

var nqueueMethods = map[string]lua.LGFunction{
	"put": nqueuePut,
	"at":  nqueueAt,
	"pop": nqueuePop,
	"max": nqueueMax,
	"min": nqueueMin,
}

func nqueuePut(L *lua.LState) int {
	q, _ := checkNqueue(L)
	v := L.CheckNumber(2)
	q.d = append(q.d, v)
	if q.capa == 0 || len(q.d) <= q.capa {
		return 0
	}
	q.d = q.d[len(q.d)-q.capa : len(q.d)]
	return 0
}

func nqueueLen(L *lua.LState) int {
	q, _ := checkNqueue(L)
	L.Push(lua.LNumber(len(q.d)))
	return 1
}

func nqueueAt(L *lua.LState) int {
	q, _ := checkNqueue(L)
	v := L.CheckInt(2)
	if v < 0 {
		v = len(q.d) + v + 1
	}
	if v < 1 || v > len(q.d) {
		L.Push(lua.LNil)
	} else {
		L.Push(q.d[v-1])
	}
	return 1
}

func nqueuePop(L *lua.LState) int {
	q, _ := checkNqueue(L)
	if len(q.d) == 0 {
		L.Push(lua.LNil)
	} else {
		L.Push(q.d[len(q.d)-1])
		q.d = q.d[0 : len(q.d)-1]
	}
	return 1
}

func nqueueMax(L *lua.LState) int {
	q, _ := checkNqueue(L)
	if len(q.d) == 0 {
		L.Push(lua.LNil)
	} else if len(q.d) == 1 {
		L.Push(q.d[0])
	} else {
		max := q.d[0]
		for i := 1; i < len(q.d); i++ {
			if q.d[i] > max {
				max = q.d[i]
			}
		}
		L.Push(max)
	}
	return 1
}

func nqueueMin(L *lua.LState) int {
	q, _ := checkNqueue(L)
	if len(q.d) == 0 {
		L.Push(lua.LNil)
	} else if len(q.d) == 1 {
		L.Push(q.d[0])
	} else {
		max := q.d[0]
		for i := 1; i < len(q.d); i++ {
			if q.d[i] < max {
				max = q.d[i]
			}
		}
		L.Push(max)
	}
	return 1
}

var luaFunctions = map[string]lua.LGFunction{
	"log":          luaLog,
	"template":     luaTemplate,
	"parseltsv":    luaParseLtsv,
	"threshold":    luaThreshold,
	"downtimefile": luaDowntimeFile,
	"isindowntime": luaIsInDowntime,
	"mail":         luaMail,
}

func luaGetAttr(obj lua.LValue, names ...string) lua.LValue {
	cur := obj
	for _, name := range names {
		cur = cur.(*lua.LTable).RawGetString(name)
	}
	return cur
}

func luaMustGetTableAttr(obj lua.LValue, names ...string) *lua.LTable {
	return luaGetAttr(obj, names...).(*lua.LTable)
}

func luaToString(L *lua.LState, obj lua.LValue) string {
	if ret := L.CallMeta(obj, "__tostring"); ret != lua.LNil {
		return ret.String()
	}
	tbl, ok := obj.(*lua.LTable)
	if !ok {
		return obj.String()
	}
	buf := []string{"{"}
	tbl.ForEach(func(key, value lua.LValue) {
		buf = append(buf, key.String())
		buf = append(buf, ":")
		buf = append(buf, luaToString(L, value))
		buf = append(buf, ", ")
	})
	buf = buf[0 : len(buf)-1]
	buf = append(buf, "}")
	return strings.Join(buf, "")
}

func goThread(L *lua.LState) *thread {
	return L.Get(lua.UpvalueIndex(1)).(*lua.LUserData).Value.(*thread)
}

func luaLog(L *lua.LState) int {
	th := goThread(L)
	th.shared.logger.log(logLevelOf(L.CheckString(1)), L.CheckString(2))
	return 0
}

func luaTemplate(L *lua.LState) int {
	tpl, err := template.New("").Parse(L.CheckString(1))
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}
	gomap := gluamapper.ToGoValue(L.CheckTable(2), gluamapper.Option{NameFunc: gluamapper.Id})
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, gomap); err != nil {
		L.RaiseError(err.Error())
		return 0
	}
	L.Push(lua.LString(buf.String()))
	return 1
}

func luaParseLtsv(L *lua.LState) int {
	ltsv := L.CheckString(1)
	ret := L.NewTable()
	for _, pair := range strings.Split(ltsv, "\t") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		num, err := parseNumber(parts[1])
		if err != nil {
			ret.RawSetString(parts[0], lua.LString(parts[1]))
		} else {
			ret.RawSetString(parts[0], lua.LNumber(num))
		}
	}
	L.Push(ret)
	return 1
}

func _luaThreshold(L *lua.LState) int {
	tbl := L.Get(lua.UpvalueIndex(1)).(*lua.LTable)
	state := L.CheckTable(1)
	// ignore arg #2
	obj := L.CheckTable(3)
	attrname := lua.LVAsString(tbl.RawGetString("name"))
	statename := lua.LVAsString(tbl.RawGetString("state"))
	if len(statename) == 0 {
		statename = attrname
	}

	st := state.RawGetString(statename)
	var nq *lua.LUserData
	if st == lua.LNil {
		nq = callLFunc1(L, L.GetField(L.GetGlobal("nqueue"), "new"), lua.LNumber(32)).(*lua.LUserData)
		state.RawSetString(statename, nq)
	} else {
		nq = st.(*lua.LUserData)
	}

	if !lua.LVAsBool(obj.RawGetString(attrname + "__thput__")) {
		callLFunc0(L, L.GetField(nq, "put"), nq, lua.LVAsNumber(obj.RawGetString(attrname)))
		obj.RawSetString(attrname+"__thput__", lua.LTrue)
	}

	val := tbl.RawGetString("val")
	count := int(float64(lua.LVAsNumber(tbl.RawGetString("count"))))
	op := tbl.RawGetString("op").(lua.LString).String()
	var f func(n lua.LNumber) bool
	switch op {
	case "gt":
		f = func(n lua.LNumber) bool {
			return n > lua.LVAsNumber(val)
		}
	case "ge":
		f = func(n lua.LNumber) bool {
			return n >= lua.LVAsNumber(val)
		}
	case "lt":
		f = func(n lua.LNumber) bool {
			return n < lua.LVAsNumber(val)
		}
	case "le":
		f = func(n lua.LNumber) bool {
			return n <= lua.LVAsNumber(val)
		}
	case "ne":
		f = func(n lua.LNumber) bool {
			return n != lua.LVAsNumber(val)
		}
	case "eq":
		f = func(n lua.LNumber) bool {
			return n == lua.LVAsNumber(val)
		}
	case "range":
		ns := strings.Split(lua.LVAsString(val), ",")
		min, _ := parseNumber(ns[0])
		max, _ := parseNumber(ns[1])
		f = func(n lua.LNumber) bool {
			fval := float64(n)
			return min < fval && fval < max
		}
	}

	ret := lua.LTrue
	for i := 0; i < count+1; i++ {
		index := 0 - i - 1
		lcval := callLFunc1(L, L.GetField(nq, "at"), nq, lua.LNumber(index))
		if i == count {
			if lua.LVAsBool(tbl.RawGetString("recover")) {
				if lcval == lua.LNil || f(lua.LVAsNumber(lcval)) {
					ret = lua.LFalse
					goto finally
				}
			} else {
				if lcval != lua.LNil && f(lua.LVAsNumber(lcval)) {
					ret = lua.LFalse
					goto finally
				}
			}
		} else {
			if lcval == lua.LNil {
				ret = lua.LFalse
				goto finally
			}
			if !f(lua.LVAsNumber(lcval)) {
				ret = lua.LFalse
				goto finally
			}
		}
	}
finally:
	L.Push(ret)
	return 1
}

func luaThreshold(L *lua.LState) int {
	L.Push(L.NewClosure(_luaThreshold, L.CheckTable(1)))
	return 1
}

func _luaDowntimeFile(L *lua.LState) int {
	fglob := L.Get(lua.UpvalueIndex(1)).String()
	files, err := filepath.Glob(fglob)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}
	if len(files) == 0 {
		L.Push(lua.LFalse)
		return 1
	}
	sort.Strings(files)
	file := files[len(files)-1]
	bs, err := ioutil.ReadFile(file)
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}
	content := string(bs)
	dstart := time.Unix(0, 0)
	dend := time.Unix(0, 0)
	for _, line := range strings.Split(content, "\n") {
		var err error
		line1 := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(line1, "START:") {
			dstart, err = time.Parse(time.RFC3339, strings.TrimSpace(line1[6:len(line1)]))
			if err != nil {
				L.RaiseError("invalid downtime '%s', downtime must be written in RFC3339 format.", line1[6:len(line1)])
				return 0
			}
		}
		if strings.HasPrefix(line1, "END:") {
			dend, err = time.Parse(time.RFC3339, strings.TrimSpace(line1[4:len(line1)]))
			if err != nil {
				L.RaiseError("invalid downtime '%s', downtime must be written in RFC3339 format.", line1[4:len(line1)])
				return 0
			}
		}
	}
	now := time.Now()
	if now.After(dstart) && now.Before(dend) {
		L.Push(lua.LTrue)
	} else {
		L.Push(lua.LFalse)
	}
	return 1
}

func luaDowntimeFile(L *lua.LState) int {
	L.CheckString(1)
	L.Push(L.NewClosure(_luaDowntimeFile, L.Get(1)))
	return 1
}

func luaIsInDowntime(L *lua.LState) int {
	th := goThread(L)
	if th.isInDowntime {
		L.Push(lua.LTrue)
	} else {
		L.Push(lua.LFalse)
	}
	return 1
}

func luaMail(L *lua.LState) int {
	tbl := L.CheckTable(1)
	luser := tbl.RawGetString("user")
	if luser == lua.LNil {
		L.Push(lua.LFalse)
		L.Push(lua.LString("user can not be nil"))
		return 2
	}
	user := lua.LVAsString(luser)
	password := lua.LVAsString(tbl.RawGetString("password"))

	lauthhost := tbl.RawGetString("authhost")
	if lauthhost == lua.LNil {
		L.Push(lua.LFalse)
		L.Push(lua.LString("authhost can not be nil"))
		return 2
	}
	authhost := lua.LVAsString(lauthhost)

	lhost := tbl.RawGetString("host")
	if lhost == lua.LNil {
		L.Push(lua.LFalse)
		L.Push(lua.LString("host can not be nil"))
		return 2
	}
	host := lua.LVAsString(lhost)

	lfrom := tbl.RawGetString("from")
	if lfrom == lua.LNil {
		L.Push(lua.LFalse)
		L.Push(lua.LString("from can not be nil"))
		return 2
	}
	from := lua.LVAsString(lfrom)

	ltos := tbl.RawGetString("to")
	if ltos == lua.LNil {
		L.Push(lua.LFalse)
		L.Push(lua.LString("to can not be nil"))
		return 2
	}
	tos := []string{}
	if ltos.Type() == lua.LTString {
		tos = append(tos, lua.LVAsString(ltos))
	} else {
		ltos.(*lua.LTable).ForEach(func(key, value lua.LValue) {
			tos = append(tos, lua.LVAsString(value))
		})
	}

	subject := lua.LVAsString(tbl.RawGetString("subject"))
	body := lua.LVAsString(tbl.RawGetString("body"))

	auth := smtp.PlainAuth("", user, password, authhost)
	message := []string{}
	for k, v := range map[string]string{
		"From":                      from,
		"To":                        strings.Join(tos, ", "),
		"MIME-Version":              "1.0",
		"Content-Type":              "text/plain; charset=\"utf-8\"",
		"Content-Transfer-Encoding": "base64",
	} {
		message = append(message, fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message = append(message, encodeSubject(subject))
	message = append(message, "\r\n")
	message = append(message, add76crlf(base64.StdEncoding.EncodeToString([]byte(body))))
	if err := smtp.SendMail(host, auth, from, tos, []byte(strings.Join(message, ""))); err != nil {
		L.Push(lua.LFalse)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}
