// Package man renders an OpenCLI model as a roff man page. All commands go
// into one page: the root's options in OPTIONS, and each subcommand in its
// own subsection of COMMANDS.
package man

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/angrifel/opencligen/gen/docs"
	"github.com/angrifel/opencligen/spec"
)

// Options configure the generated page.
type Options struct {
	Section       string // manual section; "1" by default
	Date          string // shown in the page footer; empty by default for reproducible output
	IncludeHidden bool   // document hidden commands, options and arguments
}

// Generate renders the model as a man page.
func Generate(m *spec.Model, opts Options) []byte {
	if opts.Section == "" {
		opts.Section = "1"
	}
	g := &generator{model: m, opts: opts}
	g.page()
	return g.buf.Bytes()
}

type generator struct {
	model *spec.Model
	opts  Options
	buf   bytes.Buffer
}

// line writes one roff line, escaping text that would be read as a request.
func (g *generator) line(s string) {
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "'") {
		s = `\&` + s
	}
	g.buf.WriteString(s + "\n")
}

func (g *generator) req(format string, args ...any) {
	fmt.Fprintf(&g.buf, format+"\n", args...)
}

// esc escapes text for roff.
func esc(s string) string {
	return strings.ReplaceAll(s, `\`, `\e`)
}

// optName escapes an option name, using \- so hyphens render as minus signs.
func optName(s string) string {
	return strings.ReplaceAll(esc(s), "-", `\-`)
}

// text writes paragraphs, one roff line per source line.
func (g *generator) text(s string) {
	for i, para := range strings.Split(strings.TrimSpace(s), "\n\n") {
		if i > 0 {
			g.req(".PP")
		}
		for _, l := range strings.Split(para, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				g.line(esc(l))
			}
		}
	}
}

func (g *generator) page() {
	m, root := g.model, g.model.Root
	source := root.Name
	if m.Info.Version != "" {
		source += " " + m.Info.Version
	}
	title := m.Info.Title
	if title == "" {
		title = root.Name
	}
	g.req(`.TH %q %q %q %q %q`, strings.ToUpper(root.Name), g.opts.Section, g.opts.Date, source, title)

	g.req(".SH NAME")
	name := esc(root.Name)
	if root.Summary != "" {
		name += ` \- ` + esc(root.Summary)
	}
	g.line(name)

	g.req(".SH SYNOPSIS")
	g.synopsis(root)

	if d := root.Description; d != "" {
		g.req(".SH DESCRIPTION")
		g.text(d)
	}

	if opts := docs.Options(root, g.opts.IncludeHidden); len(opts) > 0 {
		g.req(".SH OPTIONS")
		g.options(opts)
	}
	if args := docs.Arguments(root, g.opts.IncludeHidden); len(args) > 0 {
		g.req(".SH ARGUMENTS")
		g.arguments(args)
	}

	cmds := docs.Commands(root, g.opts.IncludeHidden)[1:]
	if len(cmds) > 0 {
		g.req(".SH COMMANDS")
		for _, c := range cmds {
			g.command(c)
		}
	}

	if len(root.Examples) > 0 {
		g.req(".SH EXAMPLES")
		g.examples(root.Examples)
	}
	if len(root.ExitCodes) > 0 {
		g.req(".SH EXIT STATUS")
		g.exitCodes(root.ExitCodes)
	}
	g.seeAlso()
}

func (g *generator) synopsis(c *spec.Command) {
	if len(c.Usage) > 0 {
		g.req(".nf")
		for _, u := range c.Usage {
			g.line(esc(u))
		}
		g.req(".fi")
		return
	}
	syn := c.Synopsis()
	g.req(".B %s", esc(c.FullName()))
	if rest := strings.TrimSpace(strings.TrimPrefix(syn, c.FullName())); rest != "" {
		g.line(esc(rest))
	}
}

func (g *generator) command(c *spec.Command) {
	g.req(".SS %q", esc(c.FullName()))
	g.synopsis(c)
	g.req(".PP")
	if c.Description != "" {
		g.text(c.Description)
	} else if c.Summary != "" {
		g.text(c.Summary)
	}
	if len(c.Aliases) > 0 {
		g.req(".PP")
		g.line("Aliases: " + esc(strings.Join(c.Aliases, ", ")) + ".")
	}
	if c.Interactive {
		g.req(".PP")
		g.line("This command prompts for input.")
	}
	if opts := docs.Options(c, g.opts.IncludeHidden); len(opts) > 0 {
		g.req(".PP")
		g.req(`\fBOptions:\fR`)
		g.options(opts)
	}
	if args := docs.Arguments(c, g.opts.IncludeHidden); len(args) > 0 {
		g.req(".PP")
		g.req(`\fBArguments:\fR`)
		g.arguments(args)
	}
	if len(c.Examples) > 0 {
		g.req(".PP")
		g.req(`\fBExamples:\fR`)
		g.examples(c.Examples)
	}
	if len(c.ExitCodes) > 0 {
		g.req(".PP")
		g.req(`\fBExit status:\fR`)
		g.exitCodes(c.ExitCodes)
	}
}

func (g *generator) options(opts []*spec.Option) {
	for _, o := range opts {
		g.req(".TP")
		names := docs.OptionNames(o)
		parts := make([]string, len(names))
		for i, n := range names {
			parts[i] = `\fB` + optName(n) + `\fR`
		}
		head := strings.Join(parts, ", ")
		if vu := o.ValueUsage(); vu != "" {
			head += ` \fI` + esc(vu) + `\fR`
		}
		g.line(head)
		desc := o.Description
		if o.Required {
			desc = strings.TrimSpace("Required. " + desc)
		}
		if facts := docs.ValueFacts(o.Value); len(facts) > 0 {
			desc = strings.TrimSpace(desc + " (" + strings.Join(facts, "; ") + ")")
		}
		if desc != "" {
			g.text(desc)
		}
	}
}

func (g *generator) arguments(args []*spec.Argument) {
	for _, a := range args {
		g.req(".TP")
		head := `\fI` + esc(a.Name) + `\fR`
		if a.MaxCount > 1 {
			head += "..."
		}
		g.line(head)
		desc := a.Description
		if a.Required {
			desc = strings.TrimSpace("Required. " + desc)
		}
		if facts := docs.ValueFacts(a.Value); len(facts) > 0 {
			desc = strings.TrimSpace(desc + " (" + strings.Join(facts, "; ") + ")")
		}
		if desc != "" {
			g.text(desc)
		}
	}
}

func (g *generator) examples(ex []string) {
	g.req(".PP")
	g.req(".RS 4")
	g.req(".nf")
	for _, e := range ex {
		g.line(esc(e))
	}
	g.req(".fi")
	g.req(".RE")
}

func (g *generator) exitCodes(codes []spec.ExitCode) {
	for _, e := range codes {
		g.req(".TP")
		g.line(fmt.Sprintf(`\fB%d\fR`, e.Code))
		if e.Description != "" {
			g.text(e.Description)
		}
	}
}

func (g *generator) seeAlso() {
	var lines []string
	if d := g.model.Root.Documentation; d != "" {
		lines = append(lines, "Documentation: "+esc(d))
	}
	if c := g.model.Info.Contact; c != nil {
		var parts []string
		for _, s := range []string{c.Name, c.Email, c.URL} {
			if s != "" {
				parts = append(parts, esc(s))
			}
		}
		if len(parts) > 0 {
			lines = append(lines, "Contact: "+strings.Join(parts, ", "))
		}
	}
	if l := g.model.Info.License; l != nil {
		name := l.Name
		if name == "" {
			name = l.Identifier
		}
		if name != "" {
			lines = append(lines, "License: "+esc(name))
		}
	}
	if len(lines) == 0 {
		return
	}
	g.req(".SH SEE ALSO")
	for i, l := range lines {
		if i > 0 {
			g.req(".br")
		}
		g.line(l)
	}
}
