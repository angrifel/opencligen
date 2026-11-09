package man_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/angrifel/opencligen/gen/man"
	"github.com/angrifel/opencligen/internal/golden"
	"github.com/angrifel/opencligen/internal/opencli/ast"
	"github.com/angrifel/opencligen/internal/opencli/parser"
)

// TestStructure reads the generated man page back into sections and blocks
// and checks it against the description, so the choice of roff requests and
// line breaks never fails a test. For golden cases the parsed page is also
// compared with specs/golden/<case>/man.json.
func TestStructure(t *testing.T) {
	type tc struct {
		name, path string
		golden     bool
	}
	var cases []tc
	for _, c := range golden.Cases(t, "../..") {
		cases = append(cases, tc{c.Name, c.Spec, true})
	}
	cases = append(cases,
		tc{"upstream-git", "../../testdata/upstream/git.json", false},
		tc{"upstream-curl", "../../testdata/upstream/curl.json", false})

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, _, err := parser.LoadFile(c.path, parser.Options{})
			if err != nil {
				t.Fatal(err)
			}
			p, err := readRoff(string(man.Generate(m, man.Options{})))
			if err != nil {
				t.Fatal(err)
			}
			checkPage(t, m, p)
			if c.golden {
				data, err := json.MarshalIndent(p, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				golden.Assert(t, "../..", c.name+"/man.json", append(data, '\n'))
			}
		})
	}
}

func visible(c *ast.Command) bool {
	for ; c != nil; c = c.Parent {
		if c.Hidden {
			return false
		}
	}
	return true
}

// flat collapses whitespace, so text compares independently of line breaks.
func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

func checkPage(t *testing.T, m *ast.Model, p *page) {
	t.Helper()
	root := m.Root
	title := m.Info.Title
	if title == "" {
		title = root.Name
	}
	source := root.Name
	if m.Info.Version != "" {
		source += " " + m.Info.Version
	}
	if want := []string{strings.ToUpper(root.Name), "1", "", source, title}; !slices.Equal(p.Header, want) {
		t.Errorf("header = %q, want %q", p.Header, want)
	}

	// Sections appear in this order, each only when it has content.
	var cmds []*ast.Command
	root.Walk(func(c *ast.Command) {
		if visible(c) && !c.IsRoot() {
			cmds = append(cmds, c)
		}
	})
	var want []string
	add := func(name string, ok bool) {
		if ok {
			want = append(want, name)
		}
	}
	add("NAME", true)
	add("SYNOPSIS", true)
	add("DESCRIPTION", root.Description != "")
	add("OPTIONS", len(visibleOptions(root.Options)) > 0)
	add("ARGUMENTS", len(visibleArguments(root.Arguments)) > 0)
	add("COMMANDS", len(cmds) > 0)
	add("EXAMPLES", len(root.Examples) > 0)
	add("EXIT STATUS", len(root.ExitCodes) > 0)
	if c := m.Info.Contact; root.Documentation != "" || m.Info.License != nil || c != nil && (c.Name != "" || c.Email != "" || c.URL != "") {
		want = append(want, "SEE ALSO")
	}
	var got []string
	for _, s := range p.Sections {
		got = append(got, s.Name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("sections = %q, want %q", got, want)
	}

	name := root.Name
	if root.Summary != "" {
		name += " - " + root.Summary
	}
	if b := p.section("NAME").Blocks; len(b) != 1 || b[0].Text != name {
		t.Errorf("NAME = %+v, want %q", b, name)
	}
	checkSynopsis(t, root, p.section("SYNOPSIS").Blocks)
	if root.Description != "" {
		var text []string
		for _, b := range p.section("DESCRIPTION").Blocks {
			text = append(text, b.Text)
		}
		if flat(strings.Join(text, " ")) != flat(root.Description) {
			t.Errorf("DESCRIPTION = %q, want %q", text, root.Description)
		}
	}
	if s := p.section("OPTIONS"); s != nil {
		checkOptionEntries(t, root.FullName(), visibleOptions(root.Options), s.Blocks)
	}
	if s := p.section("ARGUMENTS"); s != nil {
		checkArgumentEntries(t, root.FullName(), visibleArguments(root.Arguments), s.Blocks)
	}
	if s := p.section("COMMANDS"); s != nil {
		if len(s.Subsections) != len(cmds) {
			t.Errorf("COMMANDS has %d subsections, want %d", len(s.Subsections), len(cmds))
		}
		for i, c := range cmds {
			if i < len(s.Subsections) {
				checkCommand(t, c, s.Subsections[i])
			}
		}
	}
	if s := p.section("EXIT STATUS"); s != nil {
		checkExitCodes(t, root.FullName(), root.ExitCodes, s.Blocks)
	}
}

func checkSynopsis(t *testing.T, c *ast.Command, blocks []mblock) {
	t.Helper()
	if len(blocks) == 0 {
		t.Errorf("%s: no synopsis", c.FullName())
		return
	}
	if len(c.Usage) > 0 {
		if blocks[0].Kind != "literal" || blocks[0].Text != strings.Join(c.Usage, "\n") {
			t.Errorf("%s: synopsis = %+v, want the x-usage lines %q", c.FullName(), blocks[0], c.Usage)
		}
		return
	}
	if blocks[0].Kind != "paragraph" || flat(blocks[0].Text) != c.Synopsis() {
		t.Errorf("%s: synopsis = %+v, want %q", c.FullName(), blocks[0], c.Synopsis())
	}
}

func checkCommand(t *testing.T, c *ast.Command, s *msection) {
	t.Helper()
	where := c.FullName()
	if s.Name != where {
		t.Errorf("subsection %q, want %q", s.Name, where)
		return
	}
	checkSynopsis(t, c, s.Blocks)
	desc := c.Description
	if desc == "" {
		desc = c.Summary
	}
	if desc != "" && !strings.Contains(flat(joinTexts(s.Blocks)), flat(desc)) {
		t.Errorf("%s: description %q missing", where, desc)
	}
	if len(c.Aliases) > 0 && !hasParagraph(s.Blocks, "Aliases: "+strings.Join(c.Aliases, ", ")+".") {
		t.Errorf("%s: aliases %v not listed", where, c.Aliases)
	}
	checkOptionEntries(t, where, visibleOptions(c.Options), labelled(s.Blocks, "Options:"))
	checkArgumentEntries(t, where, visibleArguments(c.Arguments), labelled(s.Blocks, "Arguments:"))
	if len(c.Examples) > 0 {
		ex := labelled(s.Blocks, "Examples:")
		if len(ex) == 0 || ex[0].Kind != "literal" || ex[0].Text != strings.Join(c.Examples, "\n") {
			t.Errorf("%s: examples = %+v, want %q", where, ex, c.Examples)
		}
	}
	checkExitCodes(t, where, c.ExitCodes, labelled(s.Blocks, "Exit status:"))
}

// labelled returns the blocks after the paragraph label, up to the next
// label.
func labelled(blocks []mblock, label string) []mblock {
	labels := []string{"Options:", "Arguments:", "Examples:", "Exit status:"}
	for i, b := range blocks {
		if b.Kind == "paragraph" && b.Text == label {
			var out []mblock
			for _, x := range blocks[i+1:] {
				if x.Kind == "paragraph" && slices.Contains(labels, x.Text) {
					break
				}
				out = append(out, x)
			}
			return out
		}
	}
	return nil
}

func entries(blocks []mblock) []mblock {
	var out []mblock
	for _, b := range blocks {
		if b.Kind == "entry" {
			out = append(out, b)
		}
	}
	return out
}

func checkOptionEntries(t *testing.T, where string, opts []*ast.Option, blocks []mblock) {
	t.Helper()
	es := entries(blocks)
	if len(es) != len(opts) {
		t.Errorf("%s: %d option entries, want %d", where, len(es), len(opts))
		return
	}
	for i, o := range opts {
		tag := strings.Join(append([]string{o.Name}, o.Aliases...), ", ")
		if vu := o.ValueUsage(); vu != "" {
			tag += " " + vu
		}
		if es[i].Text != tag {
			t.Errorf("%s: option entry %d = %q, want %q", where, i, es[i].Text, tag)
		}
		body := flat(es[i].Body)
		if !strings.Contains(body, flat(o.Description)) {
			t.Errorf("%s: option %s: body %q lacks %q", where, o.Name, body, o.Description)
		}
		if o.Required && !strings.HasPrefix(body, "Required.") {
			t.Errorf("%s: option %s: body %q does not start with Required.", where, o.Name, body)
		}
		checkFacts(t, where+": option "+o.Name, o.Value, body)
	}
}

func checkArgumentEntries(t *testing.T, where string, args []*ast.Argument, blocks []mblock) {
	t.Helper()
	es := entries(blocks)
	if len(es) != len(args) {
		t.Errorf("%s: %d argument entries, want %d", where, len(es), len(args))
		return
	}
	for i, a := range args {
		tag := a.Name
		if a.MaxCount > 1 {
			tag += "..."
		}
		if es[i].Text != tag {
			t.Errorf("%s: argument entry %d = %q, want %q", where, i, es[i].Text, tag)
		}
		body := flat(es[i].Body)
		if !strings.Contains(body, flat(a.Description)) {
			t.Errorf("%s: argument %s: body %q lacks %q", where, a.Name, body, a.Description)
		}
		if a.Required && !strings.HasPrefix(body, "Required.") {
			t.Errorf("%s: argument %s: body %q does not start with Required.", where, a.Name, body)
		}
		checkFacts(t, where+": argument "+a.Name, a.Value, body)
	}
}

func checkExitCodes(t *testing.T, where string, codes []ast.ExitCode, blocks []mblock) {
	t.Helper()
	es := entries(blocks)
	if len(es) != len(codes) {
		t.Errorf("%s: %d exit status entries, want %d", where, len(es), len(codes))
		return
	}
	for i, e := range codes {
		if es[i].Text != fmt.Sprint(e.Code) || flat(es[i].Body) != flat(e.Description) {
			t.Errorf("%s: exit status entry %d = %q %q, want %d %q", where, i, es[i].Text, es[i].Body, e.Code, e.Description)
		}
	}
}

// checkFacts checks that an entry body states the declared type, accepted
// values, range and default.
func checkFacts(t *testing.T, what string, v ast.Value, body string) {
	t.Helper()
	var want []string
	if v.Explicit {
		want = append(want, string(v.Type))
	}
	if len(v.Accepted) > 0 {
		want = append(want, "one of: "+strings.Join(v.Accepted, ", "))
	}
	if r := v.RangeText(); r != "" {
		want = append(want, r)
	}
	if v.HasDefault() && len(v.Default) > 0 {
		want = append(want, "default: "+v.DefaultText())
	}
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("%s: %q lacks %q", what, body, w)
		}
	}
}

func visibleOptions(opts []*ast.Option) []*ast.Option {
	var out []*ast.Option
	for _, o := range opts {
		if !o.Hidden {
			out = append(out, o)
		}
	}
	return out
}

func visibleArguments(args []*ast.Argument) []*ast.Argument {
	var out []*ast.Argument
	for _, a := range args {
		if !a.Hidden {
			out = append(out, a)
		}
	}
	return out
}

func joinTexts(blocks []mblock) string {
	var parts []string
	for _, b := range blocks {
		parts = append(parts, b.Text, b.Body)
	}
	return strings.Join(parts, " ")
}

func hasParagraph(blocks []mblock, text string) bool {
	for _, b := range blocks {
		if b.Kind == "paragraph" && b.Text == text {
			return true
		}
	}
	return false
}
