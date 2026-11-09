package parser

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/angrifel/opencligen/internal/opencli/ast"
)

// The document as decoded, after it passed schema validation.

type rawDocument struct {
	OpenCLI     string          `json:"opencli"`
	Info        rawInfo         `json:"info"`
	Conventions *rawConventions `json:"conventions"`
	Command     rawCommand      `json:"command"`
}

type rawInfo struct {
	Title       string       `json:"title"`
	Summary     string       `json:"summary"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	Contact     *ast.Contact `json:"contact"`
	License     *ast.License `json:"license"`
}

type rawConventions struct {
	GroupOptions    *bool   `json:"groupOptions"`
	OptionSeparator *string `json:"optionSeparator"`
}

type rawCommand struct {
	Name        string         `json:"name"`
	Aliases     []string       `json:"aliases"`
	Options     []rawOption    `json:"options"`
	Arguments   []rawArgument  `json:"arguments"`
	Commands    []rawCommand   `json:"commands"`
	ExitCodes   []ast.ExitCode `json:"exitCodes"`
	Description string         `json:"description"`
	Hidden      bool           `json:"hidden"`
	Examples    []string       `json:"examples"`
	Interactive bool           `json:"interactive"`
	Metadata    []rawMetadata  `json:"metadata"`
}

type rawOption struct {
	Name        string        `json:"name"`
	Required    bool          `json:"required"`
	Aliases     []string      `json:"aliases"`
	Arguments   []rawArgument `json:"arguments"`
	Group       string        `json:"group"`
	Description string        `json:"description"`
	Recursive   bool          `json:"recursive"`
	Hidden      bool          `json:"hidden"`
	Metadata    []rawMetadata `json:"metadata"`
}

type rawArgument struct {
	Name           string        `json:"name"`
	Required       bool          `json:"required"`
	Arity          *rawArity     `json:"arity"`
	AcceptedValues []string      `json:"acceptedValues"`
	Group          string        `json:"group"`
	Description    string        `json:"description"`
	Hidden         bool          `json:"hidden"`
	Metadata       []rawMetadata `json:"metadata"`
}

type rawArity struct {
	Minimum *int `json:"minimum"`
	Maximum *int `json:"maximum"`
}

type rawMetadata struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

// arity returns the declared arity with the schema defaults (1, 1).
func (a rawArgument) arity() (lo, hi int) {
	lo, hi = 1, 1
	if a.Arity != nil {
		if a.Arity.Minimum != nil {
			lo = *a.Arity.Minimum
		}
		if a.Arity.Maximum != nil {
			hi = *a.Arity.Maximum
		}
	}
	return lo, hi
}

func metadata(raw []rawMetadata) ast.Metadata {
	if len(raw) == 0 {
		return nil
	}
	m := make(ast.Metadata, len(raw))
	for i, r := range raw {
		m[i] = ast.MetadataEntry(r)
	}
	return m
}

// FlagName strips option prefixes: "--verbose" -> "verbose", "-v" -> "v",
// "/v" -> "v".
func FlagName(s string) string {
	return strings.TrimLeft(s, "-/")
}

func (l *loader) build(doc *rawDocument) *ast.Model {
	m := &ast.Model{
		OpenCLI: doc.OpenCLI,
		Info: ast.Info{
			Title:       doc.Info.Title,
			Summary:     doc.Info.Summary,
			Description: doc.Info.Description,
			Version:     doc.Info.Version,
			Contact:     doc.Info.Contact,
			License:     doc.Info.License,
		},
		Conventions: ast.Conventions{GroupOptions: true, OptionSeparator: " "},
	}
	if c := doc.Conventions; c != nil {
		if c.GroupOptions != nil {
			m.Conventions.GroupOptions = *c.GroupOptions
		}
		if c.OptionSeparator != nil {
			m.Conventions.OptionSeparator = *c.OptionSeparator
		}
	}
	m.Root = l.command(&doc.Command, nil, "/command")
	if m.Root != nil {
		if m.Root.Description == "" {
			m.Root.Description = strings.TrimSpace(doc.Info.Description)
		}
		if doc.Info.Summary != "" {
			m.Root.Summary = doc.Info.Summary
		} else if m.Root.Summary == "" {
			m.Root.Summary = firstLine(m.Root.Description)
		}
	}
	return m
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(line)
}

func (l *loader) command(rc *rawCommand, parent *ast.Command, ptr string) *ast.Command {
	c := &ast.Command{
		Name:        rc.Name,
		Aliases:     rc.Aliases,
		Description: strings.TrimSpace(rc.Description),
		Hidden:      rc.Hidden,
		Interactive: rc.Interactive,
		Examples:    rc.Examples,
		ExitCodes:   rc.ExitCodes,
		Parent:      parent,
		Metadata:    metadata(rc.Metadata),
		Pointer:     ptr,
	}
	c.Summary = firstLine(c.Description)
	where := c.FullName()

	if strings.TrimSpace(c.Name) == "" {
		l.errorf(ptr+"/name", "command name is empty")
	}
	l.commandMetadata(c)

	owner := map[string]string{}
	for i := range rc.Options {
		optr := ptr + "/options/" + strconv.Itoa(i)
		o := l.option(&rc.Options[i], where, optr)
		for j, n := range o.Names {
			if n == "" {
				l.errorf(optr, "%s: option %q has an empty name or alias", where, o.Name)
				continue
			}
			if prev, ok := owner[n]; ok {
				nptr := optr + "/name"
				if j > 0 {
					nptr = optr + "/aliases"
				}
				l.errorf(nptr, "%s: options %q and %q both declare the name %q", where, prev, o.Name, n)
			}
			owner[n] = o.Name
		}
		c.Options = append(c.Options, o)
	}

	for i := range rc.Arguments {
		c.Arguments = append(c.Arguments, l.argument(&rc.Arguments[i], where, ptr+"/arguments/"+strconv.Itoa(i)))
	}

	seen := map[string]bool{}
	for i := range rc.Commands {
		sptr := ptr + "/commands/" + strconv.Itoa(i)
		sub := l.command(&rc.Commands[i], c, sptr)
		for _, n := range append([]string{sub.Name}, sub.Aliases...) {
			if seen[n] {
				l.errorf(sptr+"/name", "%s: subcommand name %q is declared more than once", where, n)
			}
			seen[n] = true
		}
		c.Commands = append(c.Commands, sub)
	}
	return c
}

// commandMetadata reads the x-category, x-usage and x-documentation entries
// used by the upstream examples.
func (l *loader) commandMetadata(c *ast.Command) {
	for i, e := range c.Metadata {
		mptr := c.Pointer + "/metadata/" + strconv.Itoa(i) + "/value"
		switch e.Name {
		case "x-category":
			if err := json.Unmarshal(e.Value, &c.Category); err != nil {
				l.errorf(mptr, "%s: x-category must be a string", c.FullName())
			}
		case "x-documentation":
			if err := json.Unmarshal(e.Value, &c.Documentation); err != nil {
				l.errorf(mptr, "%s: x-documentation must be a string", c.FullName())
			}
		case "x-usage":
			var one string
			if err := json.Unmarshal(e.Value, &one); err == nil {
				c.Usage = []string{one}
			} else if err := json.Unmarshal(e.Value, &c.Usage); err != nil {
				l.errorf(mptr, "%s: x-usage must be a string or an array of strings", c.FullName())
			}
		}
	}
}

func (l *loader) option(ro *rawOption, where, ptr string) *ast.Option {
	o := &ast.Option{
		Name:        ro.Name,
		Aliases:     ro.Aliases,
		Description: strings.TrimSpace(ro.Description),
		Group:       ro.Group,
		Required:    ro.Required,
		Hidden:      ro.Hidden,
		Recursive:   ro.Recursive,
		Metadata:    metadata(ro.Metadata),
		Pointer:     ptr,
	}
	for _, n := range append([]string{ro.Name}, ro.Aliases...) {
		if n = FlagName(n); !slices.Contains(o.Names, n) {
			o.Names = append(o.Names, n)
		}
	}

	var accepted []string
	multi := len(ro.Arguments) > 1
	for i, a := range ro.Arguments {
		o.ValueNames = append(o.ValueNames, a.Name)
		lo, hi := a.arity()
		if lo > hi {
			l.errorf(ptr+"/arguments/"+strconv.Itoa(i)+"/arity", "%s: option %q argument %q has arity minimum %d greater than maximum %d", where, o.Name, a.Name, lo, hi)
		}
		if i == 0 {
			multi = multi || hi > 1
			accepted = a.AcceptedValues
		}
	}

	what := fmt.Sprintf("%s: option %q", where, o.Name)
	o.Value = l.value(o.Metadata, len(ro.Arguments) > 0, multi, accepted, what, ptr)
	return o
}

func (l *loader) argument(ra *rawArgument, where, ptr string) *ast.Argument {
	lo, hi := ra.arity()
	if lo > hi {
		l.errorf(ptr+"/arity", "%s: argument %q has arity minimum %d greater than maximum %d", where, ra.Name, lo, hi)
	}
	if !ra.Required {
		lo = 0
	}
	a := &ast.Argument{
		Name:        ra.Name,
		Description: strings.TrimSpace(ra.Description),
		Group:       ra.Group,
		Required:    ra.Required,
		Hidden:      ra.Hidden,
		MinCount:    lo,
		MaxCount:    hi,
		Metadata:    metadata(ra.Metadata),
		Pointer:     ptr,
	}
	what := fmt.Sprintf("%s: argument %q", where, a.Name)
	a.Value = l.value(a.Metadata, true, hi > 1, ra.AcceptedValues, what, ptr)
	return a
}

// value resolves x- metadata, downgrading problems to warnings in lenient
// mode.
func (l *loader) value(meta ast.Metadata, takesValue, multi bool, accepted []string, what, ptr string) ast.Value {
	v, err := parseValue(meta, takesValue, multi, accepted, l.opts)
	if err == nil {
		return v
	}
	eptr := ptr
	if err.index >= 0 {
		eptr = ptr + "/metadata/" + strconv.Itoa(err.index) + "/value"
	}
	if !l.opts.Lenient {
		l.errorf(eptr, "%s: %s", what, err.msg)
		return v
	}
	l.warnf(eptr, "%s: %s; treating it as untyped", what, err.msg)
	untyped, _ := parseValue(nil, takesValue, multi, accepted, l.opts)
	return untyped
}
