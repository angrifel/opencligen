package parser_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/angrifel/opencligen/internal/opencli/ast"
	"github.com/angrifel/opencligen/internal/opencli/parser"
)

// doc builds a minimal document around a root command body.
func doc(command string) string {
	return `{"opencli": "0.1", "info": {"title": "t", "version": "1"}, "command": {"name": "t"` + command + `}}`
}

// opt builds a root command with one value-taking option carrying metadata.
func opt(metadata string) string {
	return doc(`, "options": [{"name": "--a", "arguments": [{"name": "v", "required": true}], "metadata": [` + metadata + `]}]`)
}

func load(t *testing.T, src string, opts parser.Options) (*ast.Model, []ast.Diagnostic, error) {
	t.Helper()
	return parser.Load("test.json", []byte(src), opts)
}

func TestLoadUpstream(t *testing.T) {
	cases := []struct {
		file     string
		opts     parser.Options
		commands int    // expected command count when the load succeeds
		errWant  string // expected substring of the error otherwise
		warnings int
	}{
		{file: "git.json", commands: 251},
		{file: "curl.json", commands: 1},
		{file: "dotnet.json", errWant: `dotnet test: options "--list-tests" and "--target" both declare the name "t"`},
		{file: "ffmpeg.json", errWant: `"INT_MAX" is a C limit constant`},
		{file: "ffmpeg.json", opts: parser.Options{AllowCLimits: true}, errWant: `unsupported x-type "flags"`},
		{file: "ffmpeg.json", opts: parser.Options{Lenient: true}, commands: 1, warnings: 268},
		{file: "ffmpeg.json", opts: parser.Options{Lenient: true, AllowCLimits: true}, commands: 1, warnings: 138},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/%+v", c.file, c.opts), func(t *testing.T) {
			m, warns, err := parser.LoadFile("../../../testdata/upstream/"+c.file, c.opts)
			if c.errWant != "" {
				if err == nil || !strings.Contains(err.Error(), c.errWant) {
					t.Fatalf("error = %v, want it to contain %q", err, c.errWant)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			m.Root.Walk(func(*ast.Command) { n++ })
			if n != c.commands {
				t.Errorf("commands = %d, want %d", n, c.commands)
			}
			if len(warns) != c.warnings {
				t.Errorf("warnings = %d, want %d", len(warns), c.warnings)
			}
		})
	}
}

func TestDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		src  string
		opts parser.Options
		want string // "line:col: message substring"
	}{
		{"syntax", `{"opencli": "0.1",` + "\n" + ` "info": }`, parser.Options{}, `2:10: invalid JSON`},
		{"version", `{"opencli": "9.9", "info": {"title": "t", "version": "1"}, "command": {"name": "t"}}`, parser.Options{}, `1:13: unsupported OpenCLI version "9.9"`},
		{"schema", `{"opencli": "0.1", "info": {"title": "t"}, "command": {"name": "t"}}`, parser.Options{}, `1:28: does not conform to the OpenCLI schema: missing property 'version'`},
		{"email", `{"opencli": "0.1", "info": {"title": "t", "version": "1", "contact": {"email": "nope"}}, "command": {"name": "t"}}`, parser.Options{}, `1:80: does not conform to the OpenCLI schema: 'nope' does not match pattern`},
		{"empty name", doc(`, "commands": [{"name": " "}]`), parser.Options{}, `command name is empty`},
		{"dup option", doc(`, "options": [{"name": "--a"}, {"name": "-x", "aliases": ["--a"]}]`), parser.Options{}, `options "--a" and "-x" both declare the name "a"`},
		{"dup command", doc(`, "commands": [{"name": "a"}, {"name": "b", "aliases": ["a"]}]`), parser.Options{}, `subcommand name "a" is declared more than once`},
		{"arity", doc(`, "arguments": [{"name": "f", "arity": {"minimum": 3, "maximum": 1}}]`), parser.Options{}, `argument "f" has arity minimum 3 greater than maximum 1`},
		{"option arity", doc(`, "options": [{"name": "--a", "arguments": [{"name": "v", "arity": {"minimum": 2, "maximum": 1}}]}]`), parser.Options{}, `option "--a" argument "v" has arity minimum 2 greater than maximum 1`},
		{"unknown type", opt(`{"name": "x-type", "value": "bigint"}`), parser.Options{}, `unsupported x-type "bigint"`},
		{"type needs arg", doc(`, "options": [{"name": "--a", "metadata": [{"name": "x-type", "value": "int"}]}]`), parser.Options{}, `x-type "int" needs the option to take an argument`},
		{"string default for int", opt(`{"name": "x-type", "value": "int"}, {"name": "x-default", "value": "x"}`), parser.Options{}, `x-default: "x" is not a valid int`},
		{"fractional int", opt(`{"name": "x-type", "value": "int"}, {"name": "x-default", "value": 1.5}`), parser.Options{}, `x-default: "1.5" is not a valid int`},
		{"negative uint", opt(`{"name": "x-type", "value": "uint"}, {"name": "x-default", "value": -1}`), parser.Options{}, `x-default: "-1" is not a valid uint`},
		{"range order", opt(`{"name": "x-type", "value": "int"}, {"name": "x-range", "value": {"minimum": 3, "maximum": 1}}`), parser.Options{}, `x-range minimum 3 is greater than maximum 1`},
		{"default below range", opt(`{"name": "x-type", "value": "int"}, {"name": "x-range", "value": {"minimum": 1}}, {"name": "x-default", "value": 0}`), parser.Options{}, `x-default: 0 is below the x-range minimum 1`},
		{"range on string", opt(`{"name": "x-range", "value": {"minimum": 1}}`), parser.Options{}, `x-range can only be used with x-type int, uint or float`},
		{"range field", opt(`{"name": "x-type", "value": "int"}, {"name": "x-range", "value": {"min": 1}}`), parser.Options{}, `unknown field "min"`},
		{"dup metadata", opt(`{"name": "x-type", "value": "int"}, {"name": "x-type", "value": "int"}`), parser.Options{}, `x-type is declared more than once`},
		{"c limit", opt(`{"name": "x-type", "value": "int"}, {"name": "x-range", "value": {"maximum": "INT_MAX"}}`), parser.Options{}, `"INT_MAX" is a C limit constant`},
		{"c limit too big", opt(`{"name": "x-type", "value": "int"}, {"name": "x-range", "value": {"maximum": "UINT64_MAX"}}`), parser.Options{AllowCLimits: true}, `"UINT64_MAX" is not a valid int`},
		{"accepted not in", doc(`, "options": [{"name": "--a", "arguments": [{"name": "v", "acceptedValues": ["x"]}], "metadata": [{"name": "x-default", "value": "y"}]}]`), parser.Options{}, `"y" is not one of the accepted values (x)`},
		{"accepted on int", doc(`, "options": [{"name": "--a", "arguments": [{"name": "v", "acceptedValues": ["1"]}], "metadata": [{"name": "x-type", "value": "int"}]}]`), parser.Options{}, `acceptedValues can only be used with x-type string`},
		{"multi duration", doc(`, "options": [{"name": "--a", "arguments": [{"name": "v", "arity": {"minimum": 1, "maximum": 2}}], "metadata": [{"name": "x-type", "value": "duration"}]}]`), parser.Options{}, `x-type "duration" cannot be used for several values`},
		{"multi default", doc(`, "options": [{"name": "--a", "arguments": [{"name": "v", "arity": {"minimum": 1, "maximum": 2}}], "metadata": [{"name": "x-default", "value": "x"}]}]`), parser.Options{}, `x-default must be an array for several values`},
		{"x-usage", doc(`, "metadata": [{"name": "x-usage", "value": 3}]`), parser.Options{}, `x-usage must be a string or an array of strings`},
		{"argument type", doc(`, "arguments": [{"name": "n", "metadata": [{"name": "x-type", "value": "int"}, {"name": "x-default", "value": "x"}]}]`), parser.Options{}, `argument "n": x-default: "x" is not a valid int`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := load(t, c.src, c.opts)
			var se *ast.Error
			if !errors.As(err, &se) {
				t.Fatalf("error = %v, want a *ast.Error", err)
			}
			pos, msg, _ := strings.Cut(c.want, ": ")
			if !strings.Contains(pos, ":") {
				pos, msg = "", c.want
			}
			for _, d := range se.Diagnostics {
				if strings.Contains(d.Message, msg) && (pos == "" || fmt.Sprintf("%d:%d", d.Line, d.Column) == pos) {
					return
				}
			}
			t.Errorf("no diagnostic matching %q in:\n%v", c.want, err)
		})
	}
}

func TestDiagnosticPointsAtMetadataValue(t *testing.T) {
	src := "{\n" +
		`  "opencli": "0.1", "info": {"title": "t", "version": "1"},` + "\n" +
		`  "command": {"name": "t", "options": [{"name": "--a", "arguments": [{"name": "v"}],` + "\n" +
		`    "metadata": [` + "\n" +
		`      {"name": "x-type", "value": "bigint"}]}]}}`
	_, _, err := load(t, src, parser.Options{})
	var se *ast.Error
	if !errors.As(err, &se) || len(se.Diagnostics) != 1 {
		t.Fatalf("error = %v", err)
	}
	d := se.Diagnostics[0]
	if d.Line != 5 || d.Column != 35 || d.Pointer != "/command/options/0/metadata/0/value" {
		t.Errorf("diagnostic at %d:%d %s, want 5:35 /command/options/0/metadata/0/value", d.Line, d.Column, d.Pointer)
	}
	if got := d.String(); !strings.HasPrefix(got, "test.json:5:35: error: t: option \"--a\": unsupported x-type") {
		t.Errorf("String() = %q", got)
	}
}

func TestLenient(t *testing.T) {
	m, warns, err := load(t, opt(`{"name": "x-type", "value": "flags"}`), parser.Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || warns[0].Severity != ast.SeverityWarning || !strings.Contains(warns[0].Message, "treating it as untyped") {
		t.Fatalf("warnings = %v", warns)
	}
	if v := m.Root.Options[0].Value; v.Type != ast.String || v.Explicit {
		t.Errorf("value = %+v, want untyped string", v)
	}
}

func TestValues(t *testing.T) {
	cases := []struct {
		name     string
		metadata string
		args     string // option arguments; default: one required value
		opts     parser.Options
		check    func(t *testing.T, v ast.Value)
	}{
		{"untyped", ``, "", parser.Options{}, func(t *testing.T, v ast.Value) {
			want(t, v.Type == ast.String && !v.Explicit && v.TakesValue && !v.Multi, "untyped string, got %+v", v)
		}},
		{"int", `{"name": "x-type", "value": "int"}, {"name": "x-default", "value": 3}, {"name": "x-range", "value": {"minimum": 1, "maximum": 9}}`, "", parser.Options{}, func(t *testing.T, v ast.Value) {
			want(t, v.Type == ast.Int && v.Default[0] == int64(3) && v.Min == int64(1) && v.Max == int64(9), "got %+v", v)
			want(t, v.RangeText() == "1..9" && v.DefaultText() == "3", "texts %q %q", v.RangeText(), v.DefaultText())
		}},
		{"string numbers", `{"name": "x-type", "value": "int64"}, {"name": "x-default", "value": "12"}, {"name": "x-range", "value": {"maximum": "2.14748e+09"}}`, "", parser.Options{}, func(t *testing.T, v ast.Value) {
			want(t, v.Type == ast.Int && v.Default[0] == int64(12) && v.Max == int64(2147480000), "got %+v", v)
		}},
		{"c limits", `{"name": "x-type", "value": "unsigned"}, {"name": "x-range", "value": {"minimum": "0", "maximum": "UINT32_MAX"}}, {"name": "x-default", "value": "UINT32_MAX"}`, "", parser.Options{AllowCLimits: true}, func(t *testing.T, v ast.Value) {
			want(t, v.Type == ast.Uint && v.Max == uint64(4294967295) && v.Default[0] == uint64(4294967295), "got %+v", v)
		}},
		{"negated c limit", `{"name": "x-type", "value": "double"}, {"name": "x-range", "value": {"minimum": "-FLT_MAX", "maximum": "DBL_MAX"}}`, "", parser.Options{AllowCLimits: true}, func(t *testing.T, v ast.Value) {
			want(t, v.Type == ast.Float && v.Min.(float64) < -3.4e38 && v.Max.(float64) > 1e308, "got %+v", v)
		}},
		{"int64 limits", `{"name": "x-type", "value": "int"}, {"name": "x-range", "value": {"minimum": "I64_MIN", "maximum": "I64_MAX"}}`, "", parser.Options{AllowCLimits: true}, func(t *testing.T, v ast.Value) {
			want(t, v.Min == int64(-9223372036854775808) && v.Max == int64(9223372036854775807), "got %+v", v)
		}},
		{"duration", `{"name": "x-type", "value": "duration"}, {"name": "x-default", "value": "1m30s"}`, "", parser.Options{}, func(t *testing.T, v ast.Value) {
			want(t, v.Default[0] == 90*time.Second && v.DefaultText() == "1m30s", "got %+v", v)
		}},
		{"value boolean", `{"name": "x-type", "value": "bool"}, {"name": "x-default", "value": "false"}`, "", parser.Options{}, func(t *testing.T, v ast.Value) {
			want(t, v.Type == ast.Bool && v.TakesValue && v.Default[0] == false, "got %+v", v)
		}},
		{"multi", `{"name": "x-type", "value": "float"}, {"name": "x-default", "value": [0.5, "2"]}`, `[{"name": "v", "arity": {"minimum": 1, "maximum": 3}}]`, parser.Options{}, func(t *testing.T, v ast.Value) {
			want(t, v.Multi && len(v.Default) == 2 && v.Default[1] == 2.0 && v.DefaultText() == "0.5, 2", "got %+v", v)
		}},
		{"presence flag", ``, `[]`, parser.Options{}, func(t *testing.T, v ast.Value) {
			want(t, v.Type == ast.Bool && !v.TakesValue, "got %+v", v)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := c.args
			if args == "" {
				args = `[{"name": "v", "required": true}]`
			}
			src := doc(`, "options": [{"name": "--a", "arguments": ` + args + `, "metadata": [` + c.metadata + `]}]`)
			m, _, err := load(t, src, c.opts)
			if err != nil {
				t.Fatal(err)
			}
			c.check(t, m.Root.Options[0].Value)
		})
	}
}

func TestMultiDefaultDoesNotCorruptDocument(t *testing.T) {
	// Regression: decoding an array default into a slice that aliased the
	// document's bytes overwrote them, so a second resolution failed.
	src := doc(`, "options": [{"name": "--a", "arguments": [{"name": "v", "arity": {"minimum": 1, "maximum": 3}}], "metadata": [{"name": "x-type", "value": "int"}, {"name": "x-default", "value": [1, 2]}]}]`)
	data := []byte(src)
	for i := 0; i < 2; i++ {
		m, _, err := parser.Load("test.json", data, parser.Options{})
		if err != nil {
			t.Fatalf("load %d: %v", i, err)
		}
		if d := m.Root.Options[0].Value.Default; len(d) != 2 || d[1] != int64(2) {
			t.Fatalf("load %d: default = %v", i, d)
		}
	}
	if string(data) != src {
		t.Fatal("document bytes were modified")
	}
}

func TestModel(t *testing.T) {
	m, _, err := parser.LoadFile("../../../specs/golden/all-features/spec.json", parser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	root := m.Root
	want(t, root.Summary == "Exercises every feature opencligen supports", "root summary %q", root.Summary)
	want(t, m.Conventions.GroupOptions && m.Conventions.OptionSeparator == " ", "conventions %+v", m.Conventions)
	want(t, root.Documentation == "https://example.com/docs", "documentation %q", root.Documentation)

	cook := root.Commands[0]
	want(t, cook.FullName() == "kitchen cook" && cook.Category == "Meals" && len(cook.Usage) == 1, "cook %+v", cook)
	want(t, cook.Summary == "Cook a dish", "cook summary %q", cook.Summary)
	want(t, cook.ArgsUsage() == "<dish> [<servings>]", "args usage %q", cook.ArgsUsage())
	want(t, cook.Synopsis() == "kitchen cook [options] <dish> [<servings>]", "synopsis %q", cook.Synopsis())
	inh := cook.InheritedOptions()
	want(t, len(inh) == 2 && inh[0].Name == "--verbose", "inherited %v", inh)
	w := cook.Options[3]
	want(t, w.Name == "--weights" && w.ValueUsage() == "<kg>..." && w.Value.Multi, "weights %+v", w)

	pantry := root.Commands[1]
	want(t, pantry.Runnable() && pantry.Synopsis() == "kitchen pantry [options] [<command>] [<shelf>...]", "pantry synopsis %q", pantry.Synopsis())
	want(t, pantry.Arguments[0].MinCount == 0 && pantry.Arguments[0].MaxCount == 3, "shelf %+v", pantry.Arguments[0])
}

func want(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Errorf(format, args...)
	}
}

func TestInheritedOptionsRedeclared(t *testing.T) {
	m, _, err := parser.LoadFile("../../../specs/golden/nested-commands/spec.json", parser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// repo remote branch list inherits --dry-run from both "remote branch"
	// and "remote"; the nearer declaration replaces the farther one.
	list := m.Root.Commands[0].Commands[3].Commands[0]
	var got []string
	for _, o := range list.InheritedOptions() {
		got = append(got, o.Name)
	}
	if want := []string{"--dry-run", "--verbose", "--config"}; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("%s inherits %v, want %v", list.FullName(), got, want)
	}
	if got := list.InheritedOptions()[0]; got != m.Root.Commands[0].Commands[3].Options[0] {
		t.Errorf("--dry-run should come from %q", "repo remote branch")
	}
}
