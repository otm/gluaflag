package gluaflag

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yuin/gopher-lua"
)

var exports = map[string]lua.LGFunction{
	"new": new,
}

const luaFlagTypeName = "flag"

// Gluaflag is the background userdata component
type Gluaflag struct {
	fs        *flag.FlagSet
	flags     map[string]interface{}
	compFn    map[string]*lua.LFunction
	arguments arguments
}

type flg struct {
	name   string
	value  interface{}
	usage  string
	compFn *lua.LFunction
}

type flgs []*flg

type argument struct {
	name   string
	times  int
	usage  string
	compFn *lua.LFunction
}

type arguments []*argument

func (gf *Gluaflag) printFlags() string {
	var s []string
	gf.fs.VisitAll(func(fl *flag.Flag) {
		s = append(s, "-"+fl.Name)
	})
	return strings.Join(s, " ")
}

// Compgen returns a string with possible options for the flag
func (gf *Gluaflag) Compgen(L *lua.LState, compCWords int, compWords []string) string {
	if compCWords == 1 && len(compWords) == 1 {
		return gf.printFlags()
	}

	if compCWords <= len(compWords) {
		prev := compWords[compCWords-1]
		if string(prev[0]) == "-" {
			fl := gf.fs.Lookup(prev[1:len(prev)])
			v, ok := gf.flags[fl.Name]
			if !ok {
				return ""
			}
			switch value := v.(type) {
			case *bool:
				if string(compWords[len(compWords)-1][0]) == "-" {
					return gf.printFlags()
				}
				return ""
			case *string, *float64:
				if fn, ok := gf.compFn[fl.Name]; ok {
					if err := L.CallByParam(lua.P{
						Fn:      fn,
						NRet:    1,
						Protect: true,
					}); err != nil {
						fmt.Fprintf(os.Stderr, "%v\n", err)
						os.Exit(1)
					}
					res := L.Get(-1)
					L.Pop(1)
					return res.String()
				}
			default:
				L.RaiseError("not implemented type: %T", value)
				return ""
			}
		} else if string(compWords[len(compWords)-1][0]) == "-" {
			return gf.printFlags()
		} else { // argument
			return ""
		}

	}
	return ""
}

func toStringSlice(t *lua.LTable) []string {
	args := make([]string, 0, t.Len())
	if zv := t.RawGet(lua.LNumber(0)); zv.Type() != lua.LTNil {
		args = append(args, zv.String())
	}

	t.ForEach(func(k, v lua.LValue) {
		if key, ok := k.(lua.LNumber); !ok || int(key) < 1 {
			return
		}
		args = append(args, v.String())
	})
	return args
}

func compgen(L *lua.LState) int {
	ud := L.CheckUserData(1)
	compCWords := L.CheckInt(2)
	compWords := L.CheckTable(3)

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	comp := gf.Compgen(L, compCWords, toStringSlice(compWords))

	L.Push(lua.LString(comp))
	return 1
}

var flagFuncs = map[string]lua.LGFunction{
	"number":  number,
	"numbers": numbers,
	"int":     integer,
	"ints":    integers,
	"string":  str,
	"strings": strs,
	"bool":    boolean,
	"parse":   parse,
	"compgen": compgen,
}

// Loader is used for preloading the module
func Loader(L *lua.LState) int {

	// register functions to the table
	mod := L.SetFuncs(L.NewTable(), exports)
	// set up meta table
	// mt := L.NewTable()
	// L.SetField(mt, "__index", L.NewClosure(moduleIndex))
	// L.SetField(mt, "__call", L.NewClosure(moduleCall))
	// L.SetMetatable(mod, mt)

	flagMetaTable := L.NewTypeMetatable(luaFlagTypeName)
	L.SetField(flagMetaTable, "__index", L.SetFuncs(L.NewTable(), flagFuncs))

	// returns the module
	L.Push(mod)
	return 1
}

// New returns a new flagset userdata
func New(L *lua.LState) *lua.LUserData {
	// TODO: refactor to function
	var d lua.LValue = lua.LString("")
	larg := L.GetGlobal("arg")
	targ, ok := larg.(*lua.LTable)
	if ok {
		d = targ.RawGetInt(0)
	}

	name := L.OptString(1, d.String())
	flags := &Gluaflag{
		fs:     flag.NewFlagSet(name, flag.ContinueOnError),
		flags:  make(map[string]interface{}),
		compFn: make(map[string]*lua.LFunction),
	}

	ud := L.NewUserData()
	ud.Value = flags
	L.SetMetatable(ud, L.GetTypeMetatable(luaFlagTypeName))
	return ud
}

func new(L *lua.LState) int {
	L.Push(New(L))

	return 1
}

func number(L *lua.LState) int {
	ud := L.CheckUserData(1)
	name := L.CheckString(2)
	value := L.CheckNumber(3)
	usage := L.CheckString(4)
	cf := L.OptFunction(5, L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	f := gf.fs.Float64(name, float64(value), usage)
	gf.flags[name] = f
	gf.compFn[name] = cf

	return 0
}

func numbers(L *lua.LState) int {
	ud := L.CheckUserData(1)
	name := L.CheckString(2)
	usage := L.CheckString(3)
	cf := L.OptFunction(4, L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	var numbers numberslice
	gf.fs.Var(&numbers, name, usage)
	gf.flags[name] = &numbers
	gf.compFn[name] = cf

	return 0
}

func integer(L *lua.LState) int {
	ud := L.CheckUserData(1)
	name := L.CheckString(2)
	value := L.CheckInt(3)
	usage := L.CheckString(4)
	cf := L.OptFunction(5, L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	f := gf.fs.Int(name, int(value), usage)
	gf.flags[name] = f
	gf.compFn[name] = cf

	return 0
}

func integers(L *lua.LState) int {
	ud := L.CheckUserData(1)
	name := L.CheckString(2)
	usage := L.CheckString(3)
	cf := L.OptFunction(4, L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	var ints intslice
	gf.fs.Var(&ints, name, usage)
	gf.flags[name] = &ints
	gf.compFn[name] = cf

	return 0
}

func str(L *lua.LState) int {
	ud := L.CheckUserData(1)
	name := L.CheckString(2)
	value := L.CheckString(3)
	usage := L.CheckString(4)
	cf := L.OptFunction(5, L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	f := gf.fs.String(name, value, usage)
	gf.flags[name] = f
	gf.compFn[name] = cf

	return 0
}

func arg(L *lua.LState) int {
	ud := L.CheckUserData(1)
	name := L.CheckString(2)
	times := L.CheckInt(3)
	usage := L.CheckString(4)
	cf := L.OptFunction(5, L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	arg := &argument{
		name:   name,
		times:  times,
		usage:  usage,
		compFn: cf,
	}

	gf.arguments = append(gf.arguments, arg)

	return 0
}

func strs(L *lua.LState) int {
	ud := L.CheckUserData(1)
	name := L.CheckString(2)
	usage := L.CheckString(3)
	cf := L.OptFunction(4, L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(""))
		return 1
	}))

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	var strs stringslice
	gf.fs.Var(&strs, name, usage)
	gf.flags[name] = &strs
	gf.compFn[name] = cf

	return 0
}

func boolean(L *lua.LState) int {
	ud := L.CheckUserData(1)
	name := L.CheckString(2)
	value := L.CheckBool(3)
	usage := L.CheckString(4)

	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
	}

	f := gf.fs.Bool(name, value, usage)
	gf.flags[name] = f

	return 0
}

// UserDataTypeError is returned when it is not a flag userdata received
var ErrUserDataType = fmt.Errorf("Expected gluaflag userdata")

// Parse the command line parameters
func Parse(L *lua.LState, ud *lua.LUserData, args []string) (*lua.LTable, error) {
	gf, ok := ud.Value.(*Gluaflag)
	if !ok {
		L.RaiseError("Expected gluaflag userdata, got `%T`", ud.Value)
		return nil, ErrUserDataType
	}

	err := gf.fs.Parse(args)
	if err != nil {
		// L.RaiseError("%v", err)
		return nil, err
	}

	t := L.NewTable()
	for f, v := range gf.flags {
		switch value := v.(type) {
		case *float64:
			t.RawSetString(f, lua.LNumber(*value))
		case *string:
			t.RawSetString(f, lua.LString(*value))
		case *bool:
			t.RawSetString(f, lua.LBool(*value))
		case *int:
			t.RawSetString(f, lua.LNumber(*value))
		case *intslice:
			t.RawSetString(f, value.Table(L))
		case *numberslice:
			t.RawSetString(f, value.Table(L))
		case *stringslice:
			t.RawSetString(f, value.Table(L))
		default:
			L.RaiseError("unknown type: `%T`", v)
		}
	}

	for _, v := range gf.fs.Args() {
		t.Append(lua.LString(v))
	}

	return t, nil
}

func parse(L *lua.LState) int {
	ud := L.CheckUserData(1)
	args := L.CheckTable(2)
	a := make([]string, 0, args.Len())

	args.ForEach(func(k, v lua.LValue) {
		if v.Type() != lua.LTString {
			L.RaiseError("Expected string type, got: `%v`", v.Type())
		}
		a = append(a, v.String())
	})

	t, err := Parse(L, ud, a)
	if err != nil {
		L.RaiseError("%v", err)
	}

	L.Push(t)
	return 1
}
