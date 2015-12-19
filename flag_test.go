package gluaflag

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/yuin/gopher-lua"
)

func captureStdout() func() string {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	return func() string {
		w.Close()
		os.Stdout = old // restoring the real stdout
		return <-outC
	}
}

func captureStderr() func() string {
	old := os.Stderr // keep backup of the real Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	return func() string {
		w.Close()
		os.Stderr = old // restoring the real stdout
		return <-outC
	}
}

func doString(src string, t *testing.T) (stdout, stderr string) {
	L := lua.NewState()
	defer L.Close()
	L.PreloadModule("flag", Loader)

	restoreStdout := captureStdout()
	restoreStderr := captureStderr()
	err := L.DoString(src)
	stdout = restoreStdout()
	stderr = restoreStderr()
	if err != nil {
		t.Errorf("runtime error: %v", err)
	}

	if len(stdout) > 0 {
		stdout = stdout[0 : len(stdout)-1]
	}

	if len(stderr) > 0 {
		stderr = stderr[0 : len(stderr)-1]
	}

	return stdout, stderr
}

func TestUsage(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-foo"}
  fs = flag.new("subcommand")
  fs:number("times", 1, "Number help string")
  function fail()
    flags = fs:parse(arg)
  end
  ok, err = pcall(fail)

  print(ok)
  print(err)
  `

	expected := strings.Join([]string{
		"false",
		"<string>:7: flag provided but not defined: -foo",
	}, "\n")
	expectedStderr := strings.Join([]string{
		"flag provided but not defined: -foo",
		"Usage of subcommand:",
		"  -times float",
		"    	Number help string (default 1)",
	}, "\n")
	got, stderr := doString(src, t)

	if got != expected || stderr != expectedStderr {
		t.Errorf("expected stdout: `%v`\ngot: `%v`\nexpected stderr: `%v`\ngot: `%v`\nsrc: `%v`", expected, got, expectedStderr, stderr, src)
	}
}

func TestNumberFlag(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-times", "2"}
  fs = flag.new()
  fs:number("times", 1, "Number help string")
  flags = fs:parse(arg)

  print(flags.times)
  print(type(flags.times))
  `

	expected := "2\nnumber"
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestIntFlag(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-times", "2"}
  fs = flag.new()
  fs:int("times", 1, "Number help string")
  flags = fs:parse(arg)

  print(flags.times)
  print(type(flags.times))
  `

	expected := "2\nnumber"
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestIntSliceFlag(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-times", "2", "-times", "4"}
  fs = flag.new()
  fs:ints("times", "Number help string")
  flags = fs:parse(arg)

  print(type(flags.times))
  print(table.concat(flags.times, ","))
  `

	expected := "table\n2,4"
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestNumberFlagCompgen(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-times"}
  arg[0] = "subcommand"
  fs = flag.new()
  local function compgen()
    return "1 2 3"
  end
  fs:number("times", 1, "Number help string", compgen)
  flags = fs:compgen(2, arg)

  print(flags)
  `

	expected := strings.Join([]string{
		"1",
		"2",
		"3",
	}, " ")
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestNumberFlagAndArgs(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-times", "2", "foo"}
  fs = flag.new()
  fs:number("times", 1, "Number help string")
  flags = fs:parse(arg)

  print(flags.times)
  print(type(flags.times))
  for i, v in ipairs(flags) do
    print(i .. "=" .. v)
  end
  print(flags[1])
  `

	expected := strings.Join([]string{
		"2",
		"number",
		"1=foo",
		"foo",
	}, "\n")
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestNumberSliceFlag(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-times", "2.4", "-times", "4.3"}
  fs = flag.new()
  fs:numbers("times", "Number help string")
  flags = fs:parse(arg)

  print(type(flags.times))
  print(table.concat(flags.times, ","))
  `

	expected := "table\n2.4,4.3"
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestStringFlag(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-name", "bar"}
  fs = flag.new()
  fs:string("name", "foo", "String help string")
  flags = fs:parse(arg)

  print(flags.name)
  print(type(flags.name))
  `

	expected := strings.Join([]string{
		"bar",
		"string",
	}, "\n")
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestStringFlagCompgen(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-name"}
  arg[0] = "subcommand"
  fs = flag.new()
  local function compgen()
    return "fii foo fum"
  end
  fs:string("name", "foo", "String help string", compgen)
  flags = fs:compgen(2, arg)

  print(flags)
  `

	expected := strings.Join([]string{
		"fii",
		"foo",
		"fum",
	}, " ")
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestStringSliceFlag(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-times", "foo", "-times", "bar"}
  fs = flag.new()
  fs:strings("times", "Number help string")
  flags = fs:parse(arg)

  print(type(flags.times))
  print(table.concat(flags.times, ","))
  `

	expected := "table\nfoo,bar"
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestStringSliceNoValues(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {}
  fs = flag.new()
  fs:strings("times", "Number help string")
  flags = fs:parse(arg)

  print(type(flags.times))
  print(table.concat(flags.times, ","))
  `

	expected := "table\n"
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestBoolFlag(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-q"}
  fs = flag.new()
  fs:bool("q", false, "Bool help string")
  flags = fs:parse(arg)

  print(flags.q)
  print(type(flags.q))
  `

	expected := strings.Join([]string{
		"true",
		"boolean",
	}, "\n")
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}

func TestArgumentCompgen(t *testing.T) {
	src := `
  local flag = require('flag')
  arg = {"-name", "foo"}
  arg[0] = "subcommand"
  fs = flag.new()
  local function compgen()
    return "fii foo fum"
  end
  fs:string("name", "foo", "String help string", compgen)
  fs:argument("mr", 1, "Title", function()
    return "mr miss mrs"
  end)
  flags = fs:compgen(3, arg)

  print(flags)
  `

	expected := strings.Join([]string{
		"mr",
		"miss",
		"mrs",
	}, " ")
	got, _ := doString(src, t)

	if got != expected {
		t.Errorf("expected: `%v`, got: `%v`\nsrc: `%v`", expected, got, src)
	}
}
