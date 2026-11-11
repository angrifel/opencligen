package ast_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/angrifel/opencligen/internal/opencli/ast"
)

// tree builds, by hand:
//
//	tool [--verbose (recursive)]
//	  remote [--dry-run (recursive)]
//	    add <name> <url>
//	  config [<key>] [<value>...] (runnable group)
//	    edit
//	  version
func tree() (root, remote, add, config, edit, version *ast.Command) {
	root = &ast.Command{Name: "tool", Options: []*ast.Option{
		{Name: "--verbose", Names: []string{"verbose", "v"}, Recursive: true},
	}}
	remote = &ast.Command{Name: "remote", Parent: root, Options: []*ast.Option{
		{Name: "--dry-run", Names: []string{"dry-run"}, Recursive: true},
	}}
	add = &ast.Command{Name: "add", Parent: remote, Arguments: []*ast.Argument{
		{Name: "name", Required: true, MinCount: 1, MaxCount: 1},
		{Name: "url", Required: true, MinCount: 1, MaxCount: 1},
	}}
	remote.Commands = []*ast.Command{add}
	config = &ast.Command{Name: "config", Parent: root, Arguments: []*ast.Argument{
		{Name: "key", MaxCount: 1},
		{Name: "value", MaxCount: 3},
	}}
	edit = &ast.Command{Name: "edit", Parent: config}
	config.Commands = []*ast.Command{edit}
	version = &ast.Command{Name: "version", Parent: root}
	root.Commands = []*ast.Command{remote, config, version}
	return
}

func TestCommandPaths(t *testing.T) {
	root, _, add, _, _, _ := tree()
	if got := add.FullName(); got != "tool remote add" {
		t.Errorf("FullName = %q", got)
	}
	if !root.IsRoot() || add.IsRoot() {
		t.Error("IsRoot is wrong")
	}
	var walked []string
	root.Walk(func(c *ast.Command) { walked = append(walked, c.Name) })
	if got := strings.Join(walked, " "); got != "tool remote add config edit version" {
		t.Errorf("Walk order = %q", got)
	}
}

func TestRunnable(t *testing.T) {
	root, remote, add, config, edit, _ := tree()
	for _, c := range []struct {
		cmd  *ast.Command
		want bool
	}{
		{root, false}, // group without arguments
		{remote, false},
		{add, true},    // leaf
		{config, true}, // group that takes arguments
		{edit, true},
	} {
		if got := c.cmd.Runnable(); got != c.want {
			t.Errorf("%s: Runnable = %t, want %t", c.cmd.FullName(), got, c.want)
		}
	}
}

func TestSynopsisAndArgsUsage(t *testing.T) {
	root, remote, add, config, edit, version := tree()
	for _, c := range []struct {
		cmd  *ast.Command
		want string
	}{
		{root, "tool [options] <command>"},
		{remote, "tool remote [options] <command>"},
		{add, "tool remote add [options] <name> <url>"},
		{config, "tool config [options] [<command>] [<key>] [<value>...]"},
		{edit, "tool config edit [options]"},
		{version, "tool version [options]"},
	} {
		if got := c.cmd.Synopsis(); got != c.want {
			t.Errorf("%s: Synopsis = %q, want %q", c.cmd.FullName(), got, c.want)
		}
	}
	// Without any options in reach, there is no "[options]".
	bare := &ast.Command{Name: "hello"}
	if got := bare.Synopsis(); got != "hello" {
		t.Errorf("Synopsis = %q, want %q", got, "hello")
	}
	// Hidden arguments are left out of the usage.
	config.Arguments[1].Hidden = true
	if got := config.ArgsUsage(); got != "[<key>]" {
		t.Errorf("ArgsUsage = %q, want %q", got, "[<key>]")
	}
}

func TestInheritedOptions(t *testing.T) {
	root, remote, add, _, _, version := tree()
	names := func(opts []*ast.Option) string {
		var s []string
		for _, o := range opts {
			s = append(s, o.Name)
		}
		return strings.Join(s, " ")
	}
	if got := names(root.InheritedOptions()); got != "" {
		t.Errorf("root inherits %q", got)
	}
	if got := names(add.InheritedOptions()); got != "--dry-run --verbose" {
		t.Errorf("add inherits %q, want nearest ancestor first", got)
	}
	if got := names(version.InheritedOptions()); got != "--verbose" {
		t.Errorf("version inherits %q; --dry-run belongs to the remote subtree", got)
	}
	// A redeclaration with the same names replaces the farther option.
	again := &ast.Option{Name: "--verbose", Names: []string{"verbose", "v"}, Recursive: true}
	remote.Options = append(remote.Options, again)
	if got := add.InheritedOptions(); len(got) != 2 || got[1] != again {
		t.Errorf("add inherits %q; want the redeclared --verbose from remote", names(got))
	}
	// So does the command's own option.
	add.Options = []*ast.Option{{Name: "--dry-run", Names: []string{"dry-run"}}}
	if got := names(add.InheritedOptions()); got != "--verbose" {
		t.Errorf("add inherits %q after declaring --dry-run itself", got)
	}
	// Non-recursive options are never inherited.
	root.Options = append(root.Options, &ast.Option{Name: "--color", Names: []string{"color"}})
	if strings.Contains(names(version.InheritedOptions()), "--color") {
		t.Error("a non-recursive option was inherited")
	}
}

func TestValueUsage(t *testing.T) {
	for _, c := range []struct {
		opt  ast.Option
		want string
	}{
		{ast.Option{Name: "--flag"}, ""},
		{ast.Option{Name: "-k", ValueNames: []string{"n"}}, "<n>"},
		{ast.Option{Name: "-I", ValueNames: []string{"dir"}, Value: ast.Value{Multi: true}}, "<dir>..."},
		{ast.Option{Name: "--pair", ValueNames: []string{"k", "v"}, Value: ast.Value{Multi: true}}, "<k> <v>"},
	} {
		if got := c.opt.ValueUsage(); got != c.want {
			t.Errorf("%s: ValueUsage = %q, want %q", c.opt.Name, got, c.want)
		}
	}
}

func TestValueTexts(t *testing.T) {
	for _, c := range []struct {
		v                  ast.Value
		rangeText, deflt   string
		hasRange, hasDeflt bool
	}{
		{ast.Value{Type: ast.Int, Min: int64(1), Max: int64(9), Default: []any{int64(5)}}, "1..9", "5", true, true},
		{ast.Value{Type: ast.Int, Min: int64(1)}, ">= 1", "", true, false},
		{ast.Value{Type: ast.Float, Max: 0.5}, "<= 0.5", "", true, false},
		{ast.Value{Type: ast.Float, Multi: true, Default: []any{0.5, 2.0}}, "", "0.5, 2", false, true},
		{ast.Value{Type: ast.Duration, Default: []any{90 * time.Second}}, "", "1m30s", false, true},
		{ast.Value{Type: ast.String, Default: []any{}}, "", "", false, true},
	} {
		if got := c.v.RangeText(); got != c.rangeText {
			t.Errorf("%+v: RangeText = %q, want %q", c.v, got, c.rangeText)
		}
		if got := c.v.DefaultText(); got != c.deflt {
			t.Errorf("%+v: DefaultText = %q, want %q", c.v, got, c.deflt)
		}
		if c.v.HasRange() != c.hasRange || c.v.HasDefault() != c.hasDeflt {
			t.Errorf("%+v: HasRange/HasDefault = %t/%t", c.v, c.v.HasRange(), c.v.HasDefault())
		}
	}
	if !(ast.Value{Type: ast.Uint}).Numeric() || (ast.Value{Type: ast.Duration}).Numeric() {
		t.Error("Numeric is wrong")
	}
}

func TestMetadataGet(t *testing.T) {
	m := ast.Metadata{{Name: "x-a", Value: json.RawMessage(`1`)}, {Name: "x-a", Value: json.RawMessage(`2`)}}
	if v, ok := m.Get("x-a"); !ok || string(v) != "1" {
		t.Errorf("Get = %s, %t; want the first entry", v, ok)
	}
	if _, ok := m.Get("x-b"); ok {
		t.Error("Get found a missing entry")
	}
}

func TestSourcePosition(t *testing.T) {
	src := &ast.Source{Name: "t.json", Data: []byte("{\n" +
		`  "a": {"b": [10, {"c/d": true, "e~f": null}]},` + "\n" +
		`  "s": "x"` + "\n" +
		"}")}
	for _, c := range []struct {
		ptr       string
		line, col int
	}{
		{"", 1, 1},
		{"/a", 2, 8},
		{"/a/b/0", 2, 15},
		{"/a/b/1", 2, 19},
		{"/a/b/1/" + ast.EscapePointer("c/d"), 2, 27},
		{"/a/b/1/" + ast.EscapePointer("e~f"), 2, 40},
		{"/s", 3, 8},
		{"/a/b/9", 2, 14},      // missing element: the enclosing array
		{"/nope/deeper", 1, 1}, // missing everything: the document
	} {
		line, col := src.Position(c.ptr)
		if line != c.line || col != c.col {
			t.Errorf("Position(%q) = %d:%d, want %d:%d", c.ptr, line, col, c.line, c.col)
		}
	}
	if got := ast.EscapePointer("a/b~c"); got != "a~1b~0c" {
		t.Errorf("EscapePointer = %q", got)
	}
}

func TestLineCol(t *testing.T) {
	data := []byte("ab\ncd")
	for off, want := range map[int][2]int{0: {1, 1}, 1: {1, 2}, 3: {2, 1}, 4: {2, 2}, 99: {2, 3}} {
		if line, col := ast.LineCol(data, off); line != want[0] || col != want[1] {
			t.Errorf("LineCol(%d) = %d:%d, want %d:%d", off, line, col, want[0], want[1])
		}
	}
}

func TestDiagnostics(t *testing.T) {
	m := &ast.Model{}
	d := m.Diagnostic(ast.SeverityWarning, "/x", "something %s", "odd")
	if got := d.String(); got != ": warning: something odd" {
		t.Errorf("without a source: %q", got)
	}
	m.Source = &ast.Source{Name: "t.json", Data: []byte(`{"x": 1}`)}
	d = m.Diagnostic(ast.SeverityError, "/x", "bad")
	if got := d.String(); got != "t.json:1:7: error: bad" {
		t.Errorf("with a source: %q", got)
	}
	err := &ast.Error{Diagnostics: []ast.Diagnostic{d, d}}
	if got := err.Error(); got != "t.json:1:7: error: bad\nt.json:1:7: error: bad" {
		t.Errorf("Error() = %q", got)
	}
}
