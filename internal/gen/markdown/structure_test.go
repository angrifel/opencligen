package markdown_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/angrifel/opencligen/internal/gen/markdown"
	"github.com/angrifel/opencligen/internal/golden"
	"github.com/angrifel/opencligen/internal/opencli/ast"
	"github.com/angrifel/opencligen/internal/opencli/parser"
)

// TestStructure reads the generated Markdown back into an outline and checks
// it against the description, so layout changes (spacing, table padding,
// blank lines) never fail a test. For golden cases the outline is also
// compared with specs/golden/<case>/markdown.json.
func TestStructure(t *testing.T) {
	type tc struct {
		name, path string
		golden     bool
	}
	var cases []tc
	for _, c := range golden.Cases(t, "../../..") {
		cases = append(cases, tc{c.Name, c.Spec, true})
	}
	cases = append(cases,
		tc{"upstream-git", "../../../testdata/upstream/git.json", false},
		tc{"upstream-curl", "../../../testdata/upstream/curl.json", false})

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, _, err := parser.LoadFile(c.path, parser.Options{})
			if err != nil {
				t.Fatal(err)
			}
			doc := readMarkdown(string(markdown.Generate(m, markdown.Options{})))
			checkDocument(t, m, doc)
			if c.golden {
				data, err := json.MarshalIndent(doc, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				golden.Assert(t, "../../..", c.name+"/markdown.json", append(data, '\n'))
			}
		})
	}
}

func TestIncludeHidden(t *testing.T) {
	m, _, err := parser.LoadFile("../../../specs/golden/nested-commands/spec.json", parser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc := readMarkdown(string(markdown.Generate(m, markdown.Options{IncludeHidden: true})))
	for _, h := range []string{"`repo admin`", "`repo admin gc`", "`repo remote branch prune`"} {
		if len(doc.find(h)) != 1 {
			t.Errorf("IncludeHidden: no section %s", h)
		}
	}
}

// visible reports whether a command is documented by default: neither it nor
// an ancestor is hidden.
func visible(c *ast.Command) bool {
	for ; c != nil; c = c.Parent {
		if c.Hidden {
			return false
		}
	}
	return true
}

func checkDocument(t *testing.T, m *ast.Model, doc *section) {
	t.Helper()
	if len(doc.Sections) != 1 || doc.Sections[0].Level != 1 {
		t.Fatalf("want one top-level heading, got %d sections", len(doc.Sections))
	}
	top := doc.Sections[0]
	title := m.Info.Title
	if title == "" {
		title = m.Root.Name
	}
	if top.Heading != title {
		t.Errorf("title = %q, want %q", top.Heading, title)
	}
	if s := m.Root.Summary; s != "" && (len(top.Blocks) == 0 || top.Blocks[0].Text != s) {
		t.Errorf("document does not start with the summary %q", s)
	}

	var cmds []*ast.Command
	m.Root.Walk(func(c *ast.Command) {
		if visible(c) {
			cmds = append(cmds, c)
		}
	})

	// The command index lists every visible command, in order, linking to
	// its section.
	index := top.child("Commands")
	if len(cmds) > 1 {
		if index == nil || index.firstBlock("list") == nil {
			t.Fatal("no command index")
		}
		items := index.firstBlock("list").Items
		if len(items) != len(cmds) {
			t.Errorf("index has %d entries, want %d", len(items), len(cmds))
		}
		for i, c := range cmds {
			if i >= len(items) {
				break
			}
			want := fmt.Sprintf("[`%s`](#%s)", c.FullName(), anchorOf("`"+c.FullName()+"`"))
			if !strings.HasPrefix(items[i].Text, want) || items[i].Depth != len(c.Path())-1 {
				t.Errorf("index entry %d = %q (depth %d), want %q at depth %d", i, items[i].Text, items[i].Depth, want, len(c.Path())-1)
			}
		}
	}

	// Each visible command has exactly one section; hidden ones have none.
	m.Root.Walk(func(c *ast.Command) {
		secs := top.find("`" + c.FullName() + "`")
		switch {
		case !visible(c) && len(secs) > 0:
			t.Errorf("%s: hidden command is documented", c.FullName())
		case visible(c) && len(secs) != 1:
			t.Errorf("%s: %d sections, want 1", c.FullName(), len(secs))
		case visible(c):
			checkCommand(t, c, secs[0])
		}
	})

	// Every internal link resolves to a heading.
	anchors := map[string]bool{}
	doc.walk(func(s *section) { anchors["#"+anchorOf(s.Heading)] = true })
	data, _ := json.Marshal(doc)
	for _, m := range linkRE.FindAllStringSubmatch(string(data), -1) {
		if !anchors[m[1]] {
			t.Errorf("link %s does not resolve to a heading", m[1])
		}
	}
}

func checkCommand(t *testing.T, c *ast.Command, sec *section) {
	t.Helper()
	where := c.FullName()
	errorf := func(format string, args ...any) {
		t.Helper()
		t.Errorf("%s: %s", where, fmt.Sprintf(format, args...))
	}

	usage := c.Synopsis()
	if len(c.Usage) > 0 {
		usage = strings.Join(c.Usage, "\n")
	}
	if code := sec.firstBlock("code"); code == nil || code.Text != usage {
		errorf("usage block = %v, want %q", code, usage)
	}
	if !c.IsRoot() && c.Summary != "" {
		if p := sec.firstBlock("paragraph"); p == nil || p.Text != c.Summary {
			errorf("first paragraph = %v, want the summary %q", p, c.Summary)
		}
	}
	if len(c.Aliases) > 0 && !containsBlockText(sec, "Aliases: `"+strings.Join(c.Aliases, "`, `")+"`") {
		errorf("aliases %v not listed", c.Aliases)
	}
	if c.Category != "" && !containsBlockText(sec, "Category: "+c.Category) {
		errorf("category %q not listed", c.Category)
	}

	checkOptions(t, where, "Options", visibleOptions(c.Options), sec)
	checkOptions(t, where, "Inherited options", visibleOptions(c.InheritedOptions()), sec)

	var args []*ast.Argument
	for _, a := range c.Arguments {
		if !a.Hidden {
			args = append(args, a)
		}
	}
	rows := tableRows(t, where, sec, "Arguments", len(args))
	for i, a := range args {
		if i >= len(rows) {
			break
		}
		name := "<" + a.Name + ">"
		if a.MaxCount > 1 {
			name += "..."
		}
		if got := codeSpans(rows[i][0]); !slices.Equal(got, []string{name}) {
			errorf("argument %d = %v, want %q", i, got, name)
		}
		checkFacts(t, where+": argument "+a.Name, a.Value, rows[i][1])
		checkDescription(t, where+": argument "+a.Name, a.Description, a.Required, "", rows[i][2])
	}

	var subs []*ast.Command
	for _, s := range c.Commands {
		if visible(s) {
			subs = append(subs, s)
		}
	}
	rows = tableRows(t, where, sec, "Subcommands", len(subs))
	for i, s := range subs {
		if i >= len(rows) {
			break
		}
		want := fmt.Sprintf("[`%s`](#%s)", s.Name, anchorOf("`"+s.FullName()+"`"))
		if rows[i][0] != want || rows[i][1] != s.Summary {
			errorf("subcommand row %d = %q, want %q, %q", i, rows[i], want, s.Summary)
		}
	}

	if len(c.Examples) > 0 {
		ex := sec.child("Examples")
		if ex == nil || ex.firstBlock("code") == nil {
			errorf("no examples")
		} else {
			for _, e := range c.Examples {
				if !strings.Contains(ex.firstBlock("code").Text, e) {
					errorf("examples lack %q", e)
				}
			}
		}
	}
	rows = tableRows(t, where, sec, "Exit codes", len(c.ExitCodes))
	for i, e := range c.ExitCodes {
		if i < len(rows) && (rows[i][0] != fmt.Sprint(e.Code) || rows[i][1] != e.Description) {
			errorf("exit code row %d = %q, want %d %q", i, rows[i], e.Code, e.Description)
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

func checkOptions(t *testing.T, where, heading string, opts []*ast.Option, sec *section) {
	t.Helper()
	rows := tableRows(t, where, sec, heading, len(opts))
	for i, o := range opts {
		if i >= len(rows) {
			break
		}
		what := fmt.Sprintf("%s: %s %s", where, strings.ToLower(heading), o.Name)
		if got, want := codeSpans(rows[i][0]), append([]string{o.Name}, o.Aliases...); !slices.Equal(got, want) {
			t.Errorf("%s: names = %v, want %v", what, got, want)
		}
		if vu := o.ValueUsage(); vu != "" && !slices.Contains(codeSpans(rows[i][1]), vu) {
			t.Errorf("%s: value %q lacks %q", what, rows[i][1], vu)
		}
		checkFacts(t, what, o.Value, rows[i][1])
		checkDescription(t, what, o.Description, o.Required, o.Group, rows[i][2])
	}
}

// tableRows returns the rows of the table under the subsection heading,
// checking that it is present exactly when n > 0 and has n rows.
func tableRows(t *testing.T, where string, sec *section, heading string, n int) [][]string {
	t.Helper()
	sub := sec.child(heading)
	if n == 0 {
		if sub != nil {
			t.Errorf("%s: unexpected %q section", where, heading)
		}
		return nil
	}
	if sub == nil || sub.firstBlock("table") == nil {
		t.Errorf("%s: no %q table", where, heading)
		return nil
	}
	rows := sub.firstBlock("table").Rows
	if len(rows) != n {
		t.Errorf("%s: %q has %d rows, want %d", where, heading, len(rows), n)
	}
	return rows
}

// checkFacts checks that a value cell states the declared type, accepted
// values, range and default.
func checkFacts(t *testing.T, what string, v ast.Value, cell string) {
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
		if !strings.Contains(cell, w) {
			t.Errorf("%s: value %q lacks %q", what, cell, w)
		}
	}
}

func checkDescription(t *testing.T, what, desc string, required bool, group, cell string) {
	t.Helper()
	flat := strings.Join(strings.Fields(desc), " ")
	got := strings.Join(strings.Fields(strings.ReplaceAll(cell, "<br>", " ")), " ")
	if !strings.Contains(got, flat) {
		t.Errorf("%s: description %q lacks %q", what, cell, flat)
	}
	if required && !strings.HasPrefix(cell, "Required.") {
		t.Errorf("%s: description %q does not start with Required.", what, cell)
	}
	if group != "" && !strings.Contains(cell, "Group: "+group+".") {
		t.Errorf("%s: description %q lacks the group %q", what, cell, group)
	}
}

func containsBlockText(sec *section, text string) bool {
	for _, b := range sec.Blocks {
		if b.Kind == "paragraph" && strings.Contains(b.Text, text) {
			return true
		}
	}
	return false
}
