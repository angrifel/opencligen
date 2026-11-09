package urfave_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/angrifel/opencligen/gen/urfave"
	"github.com/angrifel/opencligen/internal/golden"
	"github.com/angrifel/opencligen/spec"
)

func load(t *testing.T, src string) *spec.Model {
	t.Helper()
	m, _, err := spec.Load("test.json", []byte(src), spec.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGolden(t *testing.T) {
	m, _, err := spec.LoadFile("../../testdata/kitchen.json", spec.Options{})
	if err != nil {
		t.Fatal(err)
	}
	src, warns, err := urfave.Generate(m, urfave.Options{Package: "kitchen", Source: "testdata/kitchen.json"})
	if err != nil || len(warns) > 0 {
		t.Fatalf("err = %v, warnings = %v", err, warns)
	}
	golden.Assert(t, "../..", "kitchen.go.golden", src)
}

func TestUnsupported(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    string
		lenient bool // also generates, with a warning, in lenient mode
	}{
		{
			name:    "separator",
			src:     `{"opencli": "0.1", "info": {"title": "t", "version": "1"}, "conventions": {"optionSeparator": ":"}, "command": {"name": "t"}}`,
			want:    `1:95: error: option separator ":" is not supported by urfave/cli`,
			lenient: true,
		},
		{
			name:    "multi-argument option",
			src:     `{"opencli": "0.1", "info": {"title": "t", "version": "1"}, "command": {"name": "t", "options": [{"name": "--pair", "arguments": [{"name": "k"}, {"name": "v"}]}]}}`,
			want:    `option "--pair" takes 2 arguments per use`,
			lenient: true,
		},
		{
			name: "shadowing",
			src: `{"opencli": "0.1", "info": {"title": "t", "version": "1"}, "command": {"name": "t",
				"options": [{"name": "--verbose", "aliases": ["-x"], "recursive": true}],
				"commands": [{"name": "sub", "options": [{"name": "-x", "arguments": [{"name": "v"}]}]}]}}`,
			want:    `t sub: option "-x" reuses the name "x" of the recursive option "--verbose"; urfave/cli would drop "--verbose" entirely in this subcommand`,
			lenient: true,
		},
		{
			name: "method collision",
			src: `{"opencli": "0.1", "info": {"title": "t", "version": "1"}, "command": {"name": "t",
				"commands": [{"name": "foo-bar"}, {"name": "foo", "commands": [{"name": "bar"}]}]}}`,
			want: `commands "t foo-bar" and "t foo bar" both map to handler method FooBar; set x-go-name on one of them`,
		},
		{
			name: "x-go-name",
			src: `{"opencli": "0.1", "info": {"title": "t", "version": "1"}, "command": {"name": "t",
				"commands": [{"name": "a", "metadata": [{"name": "x-go-name", "value": "lower"}]}]}}`,
			want: `t a: x-go-name must be an exported Go identifier, got "lower"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := load(t, c.src)
			_, _, err := urfave.Generate(m, urfave.Options{})
			var se *spec.Error
			if !errors.As(err, &se) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error = %v, want it to contain %q", err, c.want)
			}

			src, warns, err := urfave.Generate(m, urfave.Options{Lenient: true})
			if !c.lenient {
				if err == nil {
					t.Fatal("lenient mode: want an error")
				}
				return
			}
			if err != nil || len(warns) != 1 || warns[0].Severity != spec.SeverityWarning || len(src) == 0 {
				t.Fatalf("lenient mode: err = %v, warnings = %v", err, warns)
			}
		})
	}
}

func TestBuiltinFlags(t *testing.T) {
	// --help, and the root --version when info.version is set, come from
	// urfave/cli itself.
	m := load(t, `{"opencli": "0.1", "info": {"title": "t", "version": "1"}, "command": {"name": "t",
		"options": [{"name": "--help", "aliases": ["-h"]}, {"name": "--version"}, {"name": "--host", "aliases": ["-H"]}]}}`)
	src, _, err := urfave.Generate(m, urfave.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if strings.Contains(s, `Name: "help"`) || strings.Contains(s, `"version"`) || !strings.Contains(s, `"host"`) {
		t.Errorf("unexpected flags in:\n%s", s)
	}
}

func TestPackageName(t *testing.T) {
	m := load(t, `{"opencli": "0.1", "info": {"title": "t", "version": "1"}, "command": {"name": "t"}}`)
	if _, _, err := urfave.Generate(m, urfave.Options{Package: "not-valid"}); err == nil {
		t.Fatal("want an error for an invalid package name")
	}
}
