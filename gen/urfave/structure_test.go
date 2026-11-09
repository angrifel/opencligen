package urfave_test

import (
	"encoding/json"
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/angrifel/opencligen/internal/golden"
	"github.com/angrifel/opencligen/internal/opencli/ast"
	"github.com/angrifel/opencligen/internal/opencli/parser"
)

// The structure test checks what the generated code builds, not how its
// source is laid out. For each description it compiles the generated code
// with a dump program that walks the urfave/cli command tree returned by
// New and runs every runnable command once with valid arguments, recording
// which Handlers method it reaches. The resulting tree is then
//
//   - checked against the description: every command and option must be
//     modeled with the properties the description gives it, and
//   - for golden cases, compared with specs/golden/<case>/structure.json.

// Command and Flag mirror the JSON written by dumpProgram.
type Command struct {
	Name              string    `json:"name"`
	Aliases           []string  `json:"aliases,omitempty"`
	Usage             string    `json:"usage,omitempty"`
	UsageText         string    `json:"usageText,omitempty"`
	Description       string    `json:"description,omitempty"`
	Category          string    `json:"category,omitempty"`
	Version           string    `json:"version,omitempty"`
	ArgsUsage         string    `json:"argsUsage,omitempty"`
	Hidden            bool      `json:"hidden,omitempty"`
	GroupShortOptions bool      `json:"groupShortOptions,omitempty"`
	Runnable          bool      `json:"runnable,omitempty"`
	Handler           string    `json:"handler,omitempty"`
	Flags             []Flag    `json:"flags,omitempty"`
	Commands          []Command `json:"commands,omitempty"`
}

type Flag struct {
	Type      string   `json:"type"`
	Names     []string `json:"names"`
	Usage     string   `json:"usage,omitempty"`
	Category  string   `json:"category,omitempty"`
	Required  bool     `json:"required,omitempty"`
	Hidden    bool     `json:"hidden,omitempty"`
	Inherited bool     `json:"inherited,omitempty"` // applies to subcommands too (not Local)
	Default   string   `json:"default,omitempty"`
	Validated bool     `json:"validated,omitempty"`
}

type dumpResult struct {
	Tree   Command  `json:"tree"`
	Errors []string `json:"errors"`
}

// invocation runs one command; Path identifies it in the dump. The first
// successful invocation of a path records its handler.
type invocation struct {
	Path      string   `json:"path"`
	Args      []string `json:"args"`
	WantError string   `json:"wantError,omitempty"` // expected error substring; empty when it must succeed
}

type structureCase struct {
	name, pkg, path string
	opts            parser.Options
	golden          bool // compare with specs/golden/<name>/structure.json
}

func TestStructure(t *testing.T) {
	var cases []structureCase
	for _, c := range golden.Cases(t, "../..") {
		cases = append(cases, structureCase{name: c.Name, pkg: c.Package, path: c.Spec, golden: true})
	}
	cases = append(cases,
		structureCase{name: "upstream-git", pkg: "upstreamgit", path: "../../testdata/upstream/git.json"},
		structureCase{name: "upstream-curl", pkg: "upstreamcurl", path: "../../testdata/upstream/curl.json"},
		structureCase{name: "upstream-ffmpeg", pkg: "upstreamffmpeg", path: "../../testdata/upstream/ffmpeg.json",
			opts: parser.Options{Lenient: true, AllowCLimits: true}},
	)

	s := newScratch(t)
	models := map[string]*ast.Model{}
	var imports, registrations strings.Builder
	for i, c := range cases {
		m, _, err := parser.LoadFile(c.path, c.opts)
		if err != nil {
			t.Fatal(err)
		}
		models[c.pkg] = m
		src := s.generate(m, c.pkg, c.opts.Lenient)
		s.write(c.pkg+"/recorder.go", recorder(t, c.pkg, src))
		invs := invocations(m)
		t.Logf("%s: %d command lines", c.name, len(invs))
		data, err := json.Marshal(invs)
		if err != nil {
			t.Fatal(err)
		}
		s.write("data/"+c.pkg+".json", string(data))
		fmt.Fprintf(&imports, "\tp%d %q\n", i, "example.com/scratch/"+c.pkg)
		fmt.Fprintf(&registrations, "\t\t%q: func(got *string) *cli.Command { return p%d.New(p%d.Recorder{Got: got}) },\n", c.pkg, i, i)
	}
	s.write("main.go", strings.Replace(strings.Replace(dumpProgram, "\t// IMPORTS\n", imports.String(), 1), "\t\t// CASES\n", registrations.String(), 1))
	bin := s.build()

	out, err := exec.Command(bin, filepath.Join(s.dir, "data")).Output()
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	var results map[string]dumpResult
	if err := json.Unmarshal(out, &results); err != nil {
		t.Fatal(err)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := results[c.pkg]
			for _, e := range r.Errors {
				t.Errorf("command line: %s", e)
			}
			m := models[c.pkg]
			checkCommand(t, m, m.Root, r.Tree, map[string]string{})
			if c.golden {
				want, err := json.MarshalIndent(r.Tree, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				golden.Assert(t, "../..", c.name+"/structure.json", append(want, '\n'))
			}
		})
	}
}

// recorder generates a Handlers implementation for a generated package: each
// method stores its own name. The method list is read from the generated
// Handlers interface.
func recorder(t *testing.T, pkg string, src []byte) string {
	t.Helper()
	f, err := goparser.ParseFile(token.NewFileSet(), "gen.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\nimport (\n\t\"context\"\n\n\t\"github.com/urfave/cli/v3\"\n)\n\n", pkg)
	b.WriteString("// Recorder implements Handlers by storing the name of the method called.\ntype Recorder struct{ Got *string }\n\n")
	goast.Inspect(f, func(n goast.Node) bool {
		ts, ok := n.(*goast.TypeSpec)
		if !ok || ts.Name.Name != "Handlers" {
			return true
		}
		for _, meth := range ts.Type.(*goast.InterfaceType).Methods.List {
			name := meth.Names[0].Name
			fmt.Fprintf(&b, "func (r Recorder) %s(context.Context, *cli.Command) error { *r.Got = %q; return nil }\n", name, name)
		}
		return false
	})
	return b.String()
}

// invocations builds, for every runnable command:
//
//   - a command line that should pass all checks: values for required
//     options and the minimum number of positional values;
//   - the same with every recursive option inherited from any ancestor, which
//     must be accepted at any depth;
//   - for each ancestor, the same with one of its non-recursive options,
//     which must be rejected: it belongs to the ancestor only.
func invocations(m *ast.Model) []invocation {
	var out []invocation
	m.Root.Walk(func(c *ast.Command) {
		if !c.Runnable() {
			return
		}
		base := slices.Clone(c.Path()[1:])
		known := map[string]bool{} // names valid in c
		for _, o := range append(slices.Clone(c.Options), c.InheritedOptions()...) {
			for _, n := range o.Names {
				known[n] = true
			}
			if o.Required && !builtin(m, c, o) {
				base = append(base, optionArgs(o)...)
			}
		}
		var pos []string
		for _, a := range c.Arguments {
			for range a.MinCount {
				pos = append(pos, sample(a.Value))
			}
		}
		if slices.ContainsFunc(pos, func(s string) bool { return strings.HasPrefix(s, "-") }) {
			pos = append([]string{"--"}, pos...)
		}
		line := func(extra ...string) []string {
			return append(append(slices.Clone(base), extra...), pos...)
		}
		out = append(out, invocation{Path: c.FullName(), Args: line()})

		var inherited []string
		for _, o := range c.InheritedOptions() {
			if !builtin(m, c, o) && !o.Required {
				inherited = append(inherited, optionArgs(o)...)
			}
		}
		if len(inherited) > 0 {
			out = append(out, invocation{Path: c.FullName(), Args: line(inherited...)})
		}

		for p := c.Parent; p != nil; p = p.Parent {
			for _, o := range p.Options {
				if o.Recursive || builtin(m, p, o) || slices.ContainsFunc(o.Names, func(n string) bool { return known[n] }) {
					continue
				}
				out = append(out, invocation{Path: c.FullName(), Args: line(optionArgs(o)...),
					WantError: "flag provided but not defined: -" + o.Names[0]})
				break // one per ancestor is enough
			}
		}
	})
	return out
}

// optionArgs renders an option with a valid value, if it takes one.
func optionArgs(o *ast.Option) []string {
	args := []string{"--" + o.Names[0]}
	if o.Value.TakesValue {
		args = append(args, sample(o.Value))
	}
	return args
}

// sample returns a value that satisfies v's type, accepted values and range.
func sample(v ast.Value) string {
	if len(v.Accepted) > 0 {
		return v.Accepted[0]
	}
	switch v.Type {
	case ast.Bool:
		return "true"
	case ast.Duration:
		return "1s"
	case ast.Int, ast.Uint, ast.Float:
		if v.Min != nil {
			return ast.FormatValue(v.Min)
		}
		if v.Max != nil {
			return ast.FormatValue(v.Max)
		}
		return "1"
	}
	return "value"
}

// builtin reports options urfave/cli provides itself, which are not generated.
func builtin(m *ast.Model, c *ast.Command, o *ast.Option) bool {
	return slices.Contains(o.Names, "help") ||
		(c.IsRoot() && m.Info.Version != "" && slices.Contains(o.Names, "version"))
}

// checkCommand checks that the dumped command d models the described command
// c. handlers maps handler names to the command that uses them.
func checkCommand(t *testing.T, m *ast.Model, c *ast.Command, d Command, handlers map[string]string) {
	t.Helper()
	where := c.FullName()
	errorf := func(format string, args ...any) {
		t.Helper()
		t.Errorf("%s: %s", where, fmt.Sprintf(format, args...))
	}
	eq := func(what string, got, want any) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			errorf("%s = %q, want %q", what, fmt.Sprint(got), fmt.Sprint(want))
		}
	}

	eq("name", d.Name, c.Name)
	eq("aliases", d.Aliases, nonNil(c.Aliases))
	eq("usage", d.Usage, c.Summary)
	eq("usageText", d.UsageText, strings.Join(c.Usage, "\n"))
	eq("category", d.Category, c.Category)
	eq("hidden", d.Hidden, c.Hidden)
	eq("argsUsage", d.ArgsUsage, c.ArgsUsage())
	for _, ex := range c.Examples {
		if !strings.Contains(d.Description, ex) {
			errorf("description lacks example %q", ex)
		}
	}
	for _, e := range c.ExitCodes {
		if !strings.Contains(d.Description, fmt.Sprintf("  %d", e.Code)) {
			errorf("description lacks exit code %d", e.Code)
		}
	}
	if c.Documentation != "" && !strings.Contains(d.Description, c.Documentation) {
		errorf("description lacks documentation link %q", c.Documentation)
	}
	if c.IsRoot() {
		eq("version", d.Version, m.Info.Version)
		eq("groupShortOptions", d.GroupShortOptions, m.Conventions.GroupOptions)
	} else {
		eq("version", d.Version, "")
		eq("groupShortOptions", d.GroupShortOptions, false)
	}

	// A runnable command reaches exactly one handler, named by x-go-name or
	// by the command path; handler names are unique.
	eq("runnable", d.Runnable, c.Runnable())
	if c.Runnable() {
		eq("handler", d.Handler, wantHandler(c))
		if prev, dup := handlers[d.Handler]; dup && d.Handler != "" {
			errorf("handler %s is also used by %s", d.Handler, prev)
		}
		handlers[d.Handler] = where
	} else {
		eq("handler", d.Handler, "")
	}

	var opts []*ast.Option
	for _, o := range c.Options {
		if !builtin(m, c, o) {
			opts = append(opts, o)
		}
	}
	if len(d.Flags) != len(opts) {
		errorf("%d flags, want %d", len(d.Flags), len(opts))
	} else {
		for i, o := range opts {
			f := d.Flags[i]
			what := "flag " + o.Name + " "
			eq(what+"names", f.Names, o.Names)
			eq(what+"type", f.Type, wantFlagType(o.Value))
			eq(what+"usage", f.Usage, o.Description)
			eq(what+"category", f.Category, o.Group)
			eq(what+"required", f.Required, o.Required)
			eq(what+"hidden", f.Hidden, o.Hidden)
			eq(what+"inherited", f.Inherited, o.Recursive)
			eq(what+"default", f.Default, wantDefault(o.Value))
			eq(what+"validated", f.Validated, len(o.Value.Accepted) > 0 || o.Value.HasRange() || (o.Value.Type == ast.Bool && o.Value.TakesValue))
		}
	}

	if len(d.Commands) != len(c.Commands) {
		errorf("%d subcommands, want %d", len(d.Commands), len(c.Commands))
		return
	}
	for i, s := range c.Commands {
		checkCommand(t, m, s, d.Commands[i], handlers)
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// wantHandler is the naming rule of specs/extensions.md: x-go-name, else the
// command path without the root in CamelCase, "Root" for the root.
func wantHandler(c *ast.Command) string {
	if raw, ok := c.Metadata.Get("x-go-name"); ok {
		var name string
		_ = json.Unmarshal(raw, &name)
		return name
	}
	path := c.Path()
	if len(path) == 1 {
		return "Root"
	}
	var b strings.Builder
	for _, seg := range path[1:] {
		for _, word := range strings.FieldsFunc(seg, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			b.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	return b.String()
}

// wantFlagType is the urfave/cli flag type a value maps to.
func wantFlagType(v ast.Value) string {
	if v.Type == ast.Bool {
		if v.TakesValue {
			return "StringFlag" // "-flag true", validated
		}
		return "BoolFlag"
	}
	base := map[ast.Type]string{ast.String: "String", ast.Int: "Int", ast.Uint: "Uint", ast.Float: "Float", ast.Duration: "Duration"}[v.Type]
	if v.Multi {
		return base + "SliceFlag"
	}
	return base + "Flag"
}

// wantDefault renders the default as the dump program prints flag values
// (fmt.Sprint of the urfave/cli Value field), or "" when there is none.
func wantDefault(v ast.Value) string {
	if !v.HasDefault() || len(v.Default) == 0 {
		return ""
	}
	parts := make([]string, len(v.Default))
	for i, d := range v.Default {
		parts[i] = ast.FormatValue(d)
	}
	if v.Multi {
		return "[" + strings.Join(parts, " ") + "]"
	}
	if v.Type == ast.Bool && v.TakesValue {
		return parts[0] // stored as the string "true" or "false"
	}
	if parts[0] == "false" || parts[0] == "0" || parts[0] == "0s" || parts[0] == "" {
		return "" // zero values are indistinguishable from no default
	}
	return parts[0]
}

// dumpProgram walks each generated command tree and runs each invocation.
// The test replaces the IMPORTS and CASES markers.
const dumpProgram = `package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/urfave/cli/v3"

	// IMPORTS
)

type command struct {
	Name              string    ` + "`json:\"name\"`" + `
	Aliases           []string  ` + "`json:\"aliases,omitempty\"`" + `
	Usage             string    ` + "`json:\"usage,omitempty\"`" + `
	UsageText         string    ` + "`json:\"usageText,omitempty\"`" + `
	Description       string    ` + "`json:\"description,omitempty\"`" + `
	Category          string    ` + "`json:\"category,omitempty\"`" + `
	Version           string    ` + "`json:\"version,omitempty\"`" + `
	ArgsUsage         string    ` + "`json:\"argsUsage,omitempty\"`" + `
	Hidden            bool      ` + "`json:\"hidden,omitempty\"`" + `
	GroupShortOptions bool      ` + "`json:\"groupShortOptions,omitempty\"`" + `
	Runnable          bool      ` + "`json:\"runnable,omitempty\"`" + `
	Handler           string    ` + "`json:\"handler,omitempty\"`" + `
	Flags             []flag    ` + "`json:\"flags,omitempty\"`" + `
	Commands          []command ` + "`json:\"commands,omitempty\"`" + `
}

type flag struct {
	Type      string   ` + "`json:\"type\"`" + `
	Names     []string ` + "`json:\"names\"`" + `
	Usage     string   ` + "`json:\"usage,omitempty\"`" + `
	Category  string   ` + "`json:\"category,omitempty\"`" + `
	Required  bool     ` + "`json:\"required,omitempty\"`" + `
	Hidden    bool     ` + "`json:\"hidden,omitempty\"`" + `
	Inherited bool     ` + "`json:\"inherited,omitempty\"`" + `
	Default   string   ` + "`json:\"default,omitempty\"`" + `
	Validated bool     ` + "`json:\"validated,omitempty\"`" + `
}

type result struct {
	Tree   command  ` + "`json:\"tree\"`" + `
	Errors []string ` + "`json:\"errors\"`" + `
}

type invocation struct {
	Path      string   ` + "`json:\"path\"`" + `
	Args      []string ` + "`json:\"args\"`" + `
	WantError string   ` + "`json:\"wantError\"`" + `
}

func flagType(f cli.Flag) string {
	switch f.(type) {
	case *cli.BoolFlag:
		return "BoolFlag"
	case *cli.StringFlag:
		return "StringFlag"
	case *cli.StringSliceFlag:
		return "StringSliceFlag"
	case *cli.IntFlag:
		return "IntFlag"
	case *cli.IntSliceFlag:
		return "IntSliceFlag"
	case *cli.UintFlag:
		return "UintFlag"
	case *cli.UintSliceFlag:
		return "UintSliceFlag"
	case *cli.FloatFlag:
		return "FloatFlag"
	case *cli.FloatSliceFlag:
		return "FloatSliceFlag"
	case *cli.DurationFlag:
		return "DurationFlag"
	}
	return fmt.Sprintf("%T", f)
}

func dumpFlag(f cli.Flag) flag {
	v := reflect.ValueOf(f).Elem()
	field := func(name string) reflect.Value { return v.FieldByName(name) }
	d := flag{
		Type:      flagType(f),
		Names:     f.Names(),
		Usage:     field("Usage").String(),
		Category:  field("Category").String(),
		Required:  field("Required").Bool(),
		Hidden:    field("Hidden").Bool(),
		Inherited: !field("Local").Bool(),
		Validated: !field("Validator").IsNil(),
	}
	if val := field("Value"); !val.IsZero() {
		d.Default = fmt.Sprint(val.Interface())
	}
	return d
}

func dump(c *cli.Command, path string, handlers map[string]string) command {
	d := command{
		Name: c.Name, Aliases: c.Aliases, Usage: c.Usage, UsageText: c.UsageText,
		Description: c.Description, Category: c.Category, Version: c.Version,
		ArgsUsage: c.ArgsUsage, Hidden: c.Hidden, GroupShortOptions: c.UseShortOptionHandling,
		Runnable: c.Action != nil, Handler: handlers[path],
	}
	if d.Aliases == nil {
		d.Aliases = []string{}
	}
	for _, f := range c.Flags {
		d.Flags = append(d.Flags, dumpFlag(f))
	}
	for _, s := range c.Commands {
		d.Commands = append(d.Commands, dump(s, path+" "+s.Name, handlers))
	}
	return d
}

func run(newCmd func(*string) *cli.Command, data string) result {
	var r result
	var invs []invocation
	raw, err := os.ReadFile(data)
	if err == nil {
		err = json.Unmarshal(raw, &invs)
	}
	if err != nil {
		r.Errors = append(r.Errors, err.Error())
	}
	handlers := map[string]string{}
	for _, inv := range invs {
		var got string
		cmd := newCmd(&got)
		cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
		cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
		err := cmd.Run(context.Background(), append([]string{cmd.Name}, inv.Args...))
		switch {
		case inv.WantError == "" && err != nil:
			r.Errors = append(r.Errors, fmt.Sprintf("%s %v: %v", cmd.Name, inv.Args, err))
		case inv.WantError != "" && (err == nil || !strings.Contains(err.Error(), inv.WantError)):
			r.Errors = append(r.Errors, fmt.Sprintf("%s %v: error %v, want %q", cmd.Name, inv.Args, err, inv.WantError))
		}
		if _, seen := handlers[inv.Path]; !seen && inv.WantError == "" {
			handlers[inv.Path] = got
		}
	}
	var unused string
	root := newCmd(&unused)
	r.Tree = dump(root, root.Name, handlers)
	return r
}

func main() {
	cases := map[string]func(*string) *cli.Command{
		// CASES
	}
	out := map[string]result{}
	for pkg, newCmd := range cases {
		out[pkg] = run(newCmd, filepath.Join(os.Args[1], pkg+".json"))
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		panic(err)
	}
}
`
