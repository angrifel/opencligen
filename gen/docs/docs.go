// Package docs holds helpers shared by the documentation generators.
package docs

import (
	"strings"

	"github.com/angrifel/opencligen/internal/opencli/ast"
)

// Visible reports whether a command is documented: hidden commands and their
// descendants are skipped unless includeHidden is set.
func Visible(c *ast.Command, includeHidden bool) bool {
	if includeHidden {
		return true
	}
	for ; c != nil; c = c.Parent {
		if c.Hidden {
			return false
		}
	}
	return true
}

// Details returns the part of a command's description that follows its
// summary, so documents that show the summary do not repeat it.
func Details(c *ast.Command) string {
	return strings.TrimSpace(strings.TrimPrefix(c.Description, c.Summary))
}

// Commands returns c and its documented descendants, depth first.
func Commands(c *ast.Command, includeHidden bool) []*ast.Command {
	var out []*ast.Command
	c.Walk(func(x *ast.Command) {
		if Visible(x, includeHidden) {
			out = append(out, x)
		}
	})
	return out
}

// Options returns the documented options of c.
func Options(c *ast.Command, includeHidden bool) []*ast.Option {
	return filter(c.Options, includeHidden)
}

// InheritedOptions returns the documented recursive options c inherits.
func InheritedOptions(c *ast.Command, includeHidden bool) []*ast.Option {
	return filter(c.InheritedOptions(), includeHidden)
}

func filter(opts []*ast.Option, includeHidden bool) []*ast.Option {
	var out []*ast.Option
	for _, o := range opts {
		if includeHidden || !o.Hidden {
			out = append(out, o)
		}
	}
	return out
}

// Arguments returns the documented positional arguments of c.
func Arguments(c *ast.Command, includeHidden bool) []*ast.Argument {
	var out []*ast.Argument
	for _, a := range c.Arguments {
		if includeHidden || !a.Hidden {
			out = append(out, a)
		}
	}
	return out
}

// OptionNames renders an option's names as written, e.g. "-k, --lookahead".
func OptionNames(o *ast.Option) []string {
	return append([]string{o.Name}, o.Aliases...)
}

// ValueFacts describes a value for documentation: its type when declared,
// accepted values, range and default, e.g. ["int", ">= 1", "default: 1"].
func ValueFacts(v ast.Value) []string {
	var facts []string
	if v.Explicit {
		facts = append(facts, string(v.Type))
	}
	if len(v.Accepted) > 0 {
		facts = append(facts, "one of: "+strings.Join(v.Accepted, ", "))
	}
	if r := v.RangeText(); r != "" {
		facts = append(facts, r)
	}
	if v.HasDefault() && len(v.Default) > 0 {
		facts = append(facts, "default: "+v.DefaultText())
	}
	return facts
}
