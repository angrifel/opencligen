// Package ast declares the resolved form of an OpenCLI description: the
// Model that the parser builds and every generator renders from, its values
// and types, and the diagnostics reported about it.
package ast

import (
	"encoding/json"
	"strings"
)

// Model is a validated OpenCLI document.
type Model struct {
	OpenCLI     string // the document's OpenCLI version
	Info        Info
	Conventions Conventions
	Root        *Command

	// Source is the document the model was parsed from, used to locate
	// diagnostics; nil for models built by hand.
	Source *Source
}

type Info struct {
	Title       string
	Summary     string
	Description string
	Version     string
	Contact     *Contact
	License     *License
}

type Contact struct {
	Name  string
	URL   string
	Email string
}

type License struct {
	Name       string
	Identifier string // SPDX identifier
	URL        string
}

// Conventions holds the document's conventions with schema defaults applied.
type Conventions struct {
	GroupOptions    bool   // short options may be grouped, as in -abc
	OptionSeparator string // between an option and its value, " " by default
}

type Command struct {
	Name        string
	Aliases     []string
	Summary     string // one line; for the root, info.summary when set
	Description string // full text; for the root, falls back to info.description
	Hidden      bool
	Interactive bool
	Examples    []string
	ExitCodes   []ExitCode

	// From the x-category, x-usage and x-documentation metadata used by the
	// upstream examples.
	Category      string
	Usage         []string
	Documentation string

	Options   []*Option
	Arguments []*Argument
	Commands  []*Command
	Parent    *Command

	Metadata Metadata // all metadata, for generator-specific extensions
	Pointer  string   // JSON pointer of the command in the document
}

type ExitCode struct {
	Code        int
	Description string
}

type Option struct {
	Name        string   // as written, e.g. "--format"
	Aliases     []string // as written
	Names       []string // distinct names without prefixes ("--v", "-v", "/v" are all "v"), primary first
	ValueNames  []string // names of the option's arguments; empty for presence flags
	Description string
	Group       string
	Required    bool
	Hidden      bool
	Recursive   bool // also applies to all subcommands
	Value       Value

	Metadata Metadata
	Pointer  string
}

// Argument is a positional argument.
type Argument struct {
	Name        string
	Description string
	Group       string
	Required    bool
	Hidden      bool
	MinCount    int // effective arity; 0 when the argument is optional
	MaxCount    int
	Value       Value

	Metadata Metadata
	Pointer  string
}

// Metadata is the list of custom metadata entries of an object.
type Metadata []MetadataEntry

type MetadataEntry struct {
	Name  string
	Value json.RawMessage
}

// Get returns the value of the first entry with the given name.
func (m Metadata) Get(name string) (json.RawMessage, bool) {
	for _, e := range m {
		if e.Name == name {
			return e.Value, true
		}
	}
	return nil, false
}

// Path returns the command names from the root to c.
func (c *Command) Path() []string {
	if c.Parent == nil {
		return []string{c.Name}
	}
	return append(c.Parent.Path(), c.Name)
}

// FullName returns the command path joined by spaces, e.g. "git remote add".
func (c *Command) FullName() string {
	return strings.Join(c.Path(), " ")
}

func (c *Command) IsRoot() bool { return c.Parent == nil }

// Runnable reports whether the command does something itself: leaf commands
// and group commands that take positional arguments. Other group commands
// only dispatch to their subcommands.
func (c *Command) Runnable() bool {
	return len(c.Commands) == 0 || len(c.Arguments) > 0
}

// InheritedOptions returns the recursive options of c's ancestors, nearest
// ancestor first. An option redeclared with the same names by c or by a
// nearer ancestor replaces the farther one, which is then left out.
func (c *Command) InheritedOptions() []*Option {
	seen := map[string]bool{}
	key := func(o *Option) string { return strings.Join(o.Names, "\x00") }
	for _, o := range c.Options {
		seen[key(o)] = true
	}
	var opts []*Option
	for p := c.Parent; p != nil; p = p.Parent {
		for _, o := range p.Options {
			if o.Recursive && !seen[key(o)] {
				seen[key(o)] = true
				opts = append(opts, o)
			}
		}
	}
	return opts
}

// Walk calls fn for c and all its descendants, depth first.
func (c *Command) Walk(fn func(*Command)) {
	fn(c)
	for _, s := range c.Commands {
		s.Walk(fn)
	}
}

// ArgsUsage renders the visible positional arguments, e.g. "<input> [<extra>...]".
func (c *Command) ArgsUsage() string {
	var parts []string
	for _, a := range c.Arguments {
		if a.Hidden {
			continue
		}
		s := "<" + a.Name + ">"
		if a.MaxCount > 1 {
			s += "..."
		}
		if !a.Required {
			s = "[" + s + "]"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

// Synopsis renders a usage line such as
// "sheila generate-parser ll [options] <grammar>".
func (c *Command) Synopsis() string {
	parts := []string{c.FullName()}
	if len(c.Options) > 0 || len(c.InheritedOptions()) > 0 {
		parts = append(parts, "[options]")
	}
	if len(c.Commands) > 0 {
		if c.Runnable() {
			parts = append(parts, "[<command>]")
		} else {
			parts = append(parts, "<command>")
		}
	}
	if au := c.ArgsUsage(); au != "" {
		parts = append(parts, au)
	}
	return strings.Join(parts, " ")
}

// ValueUsage renders an option's value placeholder, e.g. "<n>" or
// "<dir>...", or "" for presence flags.
func (o *Option) ValueUsage() string {
	if len(o.ValueNames) == 0 {
		return ""
	}
	parts := make([]string, len(o.ValueNames))
	for i, n := range o.ValueNames {
		parts[i] = "<" + n + ">"
	}
	s := strings.Join(parts, " ")
	if o.Value.Multi && len(o.ValueNames) == 1 {
		s += "..."
	}
	return s
}
