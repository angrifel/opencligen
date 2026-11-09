// Package markdown renders an OpenCLI model as a single Markdown reference
// document: an overview and command index, then one section per command
// with its usage, options, arguments, subcommands, examples and exit codes.
package markdown

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/angrifel/opencligen/gen/docs"
	"github.com/angrifel/opencligen/internal/opencli/ast"
)

// Options configure the generated document.
type Options struct {
	IncludeHidden bool // document hidden commands, options and arguments
}

// Generate renders the model as Markdown.
func Generate(m *ast.Model, opts Options) []byte {
	g := &generator{model: m, opts: opts}
	g.document()
	return g.buf.Bytes()
}

type generator struct {
	model *ast.Model
	opts  Options
	buf   bytes.Buffer
}

func (g *generator) p(format string, args ...any) {
	fmt.Fprintf(&g.buf, format, args...)
}

func (g *generator) document() {
	m, root := g.model, g.model.Root
	title := m.Info.Title
	if title == "" {
		title = root.Name
	}
	g.p("# %s\n\n", title)
	if root.Summary != "" {
		g.p("%s\n\n", root.Summary)
	}
	if d := docs.Details(root); d != "" {
		g.p("%s\n\n", d)
	}
	g.facts()

	cmds := docs.Commands(root, g.opts.IncludeHidden)
	if len(cmds) > 1 {
		g.p("## Commands\n\n")
		for _, c := range cmds {
			indent := strings.Repeat("  ", len(c.Path())-1)
			g.p("%s- [`%s`](#%s)", indent, c.FullName(), anchor(c.FullName()))
			if c.Summary != "" {
				g.p(": %s", c.Summary)
			}
			g.p("\n")
		}
		g.p("\n")
	}
	for _, c := range cmds {
		g.command(c)
	}
}

func (g *generator) facts() {
	info := g.model.Info
	var rows [][2]string
	if info.Version != "" {
		rows = append(rows, [2]string{"Version", info.Version})
	}
	if l := info.License; l != nil {
		name := l.Name
		if name == "" {
			name = l.Identifier
		} else if l.Identifier != "" {
			name += " (" + l.Identifier + ")"
		}
		rows = append(rows, [2]string{"License", link(name, l.URL)})
	}
	if c := info.Contact; c != nil {
		var parts []string
		if c.Name != "" {
			parts = append(parts, link(c.Name, c.URL))
		} else if c.URL != "" {
			parts = append(parts, link(c.URL, c.URL))
		}
		if c.Email != "" {
			parts = append(parts, link(c.Email, "mailto:"+c.Email))
		}
		if len(parts) > 0 {
			rows = append(rows, [2]string{"Contact", strings.Join(parts, ", ")})
		}
	}
	if d := g.model.Root.Documentation; d != "" {
		rows = append(rows, [2]string{"Documentation", link(d, d)})
	}
	if len(rows) == 0 {
		return
	}
	g.p("| | |\n|---|---|\n")
	for _, r := range rows {
		g.p("| %s | %s |\n", r[0], r[1])
	}
	g.p("\n")
}

func link(text, url string) string {
	if url == "" {
		return cell(text)
	}
	return "[" + cell(text) + "](" + url + ")"
}

func (g *generator) command(c *ast.Command) {
	g.p("## `%s`\n\n", c.FullName())
	if !c.IsRoot() {
		if c.Summary != "" {
			g.p("%s\n\n", c.Summary)
		}
		if d := docs.Details(c); d != "" {
			g.p("%s\n\n", d)
		}
	}
	var notes []string
	if len(c.Aliases) > 0 {
		notes = append(notes, "Aliases: "+code(c.Aliases...))
	}
	if c.Category != "" {
		notes = append(notes, "Category: "+c.Category)
	}
	if c.Interactive {
		notes = append(notes, "Interactive: this command prompts for input.")
	}
	if c.Hidden {
		notes = append(notes, "Hidden.")
	}
	if c.Documentation != "" && !c.IsRoot() {
		notes = append(notes, "Documentation: "+link(c.Documentation, c.Documentation))
	}
	for _, n := range notes {
		g.p("%s  \n", n)
	}
	if len(notes) > 0 {
		g.p("\n")
	}

	g.p("```\n")
	if len(c.Usage) > 0 {
		g.p("%s\n", strings.Join(c.Usage, "\n"))
	} else {
		g.p("%s\n", c.Synopsis())
	}
	g.p("```\n\n")

	if opts := docs.Options(c, g.opts.IncludeHidden); len(opts) > 0 {
		g.p("### Options\n\n")
		g.optionTable(opts)
	}
	if inh := docs.InheritedOptions(c, g.opts.IncludeHidden); len(inh) > 0 {
		g.p("### Inherited options\n\n")
		g.optionTable(inh)
	}
	if args := docs.Arguments(c, g.opts.IncludeHidden); len(args) > 0 {
		g.p("### Arguments\n\n| Argument | Values | Description |\n|---|---|---|\n")
		for _, a := range args {
			name := "`<" + a.Name + ">"
			if a.MaxCount > 1 {
				name += "..."
			}
			name += "`"
			desc := a.Description
			if a.Required {
				desc = join("Required.", desc)
			}
			g.p("| %s | %s | %s |\n", cell(name), cell(strings.Join(docs.ValueFacts(a.Value), "; ")), cell(desc))
		}
		g.p("\n")
	}

	var subs []*ast.Command
	for _, s := range c.Commands {
		if docs.Visible(s, g.opts.IncludeHidden) {
			subs = append(subs, s)
		}
	}
	if len(subs) > 0 {
		g.p("### Subcommands\n\n| Command | Description |\n|---|---|\n")
		for _, s := range subs {
			g.p("| %s | %s |\n", cell("[`"+s.Name+"`](#"+anchor(s.FullName())+")"), cell(s.Summary))
		}
		g.p("\n")
	}

	if len(c.Examples) > 0 {
		g.p("### Examples\n\n```\n%s\n```\n\n", strings.Join(c.Examples, "\n"))
	}
	if len(c.ExitCodes) > 0 {
		g.p("### Exit codes\n\n| Code | Description |\n|---|---|\n")
		for _, e := range c.ExitCodes {
			g.p("| %d | %s |\n", e.Code, cell(e.Description))
		}
		g.p("\n")
	}
}

func (g *generator) optionTable(opts []*ast.Option) {
	g.p("| Option | Value | Description |\n|---|---|---|\n")
	for _, o := range opts {
		names := code(docs.OptionNames(o)...)
		value := ""
		if vu := o.ValueUsage(); vu != "" {
			value = code(vu)
		}
		if facts := docs.ValueFacts(o.Value); len(facts) > 0 {
			value = join(value, strings.Join(facts, "; "))
		}
		desc := o.Description
		if o.Required {
			desc = join("Required.", desc)
		}
		if o.Group != "" {
			desc = join(desc, "Group: "+o.Group+".")
		}
		g.p("| %s | %s | %s |\n", cell(names), cell(value), cell(desc))
	}
	g.p("\n")
}

func join(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + " " + b
}

// code renders values as inline code, comma separated.
func code(ss ...string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = "`" + strings.ReplaceAll(s, "`", "'") + "`"
	}
	return strings.Join(q, ", ")
}

// cell escapes text for a table cell. Every cell goes through it: a "|" in
// any value, even inside inline code, would otherwise split the cell.
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n\n", "<br><br>")
	return strings.ReplaceAll(s, "\n", " ")
}

// anchor returns the GitHub-style heading anchor of a command section, whose
// heading is the command's full name in inline code.
func anchor(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
