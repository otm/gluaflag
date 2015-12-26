package gluaflag

import "github.com/yuin/gopher-lua"

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

func toTable(s []string, L *lua.LState) *lua.LTable {
	table := L.NewTable()
	for _, str := range s {
		table.Append(lua.LString(str))
	}
	return table
}
