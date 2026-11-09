package ast

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Type is the type of an option or argument value. OpenCLI 0.1 has no value
// types; opencligen's parser reads them from the x-type metadata (see
// specs/extensions.md).
type Type string

const (
	String   Type = "string"
	Int      Type = "int"
	Uint     Type = "uint"
	Float    Type = "float"
	Bool     Type = "boolean"
	Duration Type = "duration"
)

// Value describes the value of an option or positional argument.
type Value struct {
	Type       Type
	Explicit   bool // Type came from x-type
	TakesValue bool // false for options that are only present or absent
	Multi      bool // several values: a repeatable option or a variadic argument
	Accepted   []string

	// Default holds the x-default value(s): string, bool, int64, uint64,
	// float64 or time.Duration according to Type. Nil when absent; one
	// element unless Multi.
	Default []any

	// Min and Max are the x-range bounds: int64, uint64 or float64 according
	// to Type. Nil when unbounded.
	Min, Max any
}

func (v Value) Numeric() bool    { return v.Type == Int || v.Type == Uint || v.Type == Float }
func (v Value) HasDefault() bool { return v.Default != nil }
func (v Value) HasRange() bool   { return v.Min != nil || v.Max != nil }

// DefaultText renders the default for documentation, e.g. "1" or "a, b".
func (v Value) DefaultText() string {
	parts := make([]string, len(v.Default))
	for i, d := range v.Default {
		parts[i] = FormatValue(d)
	}
	return strings.Join(parts, ", ")
}

// RangeText renders the range for documentation, e.g. ">= 1" or "0..7".
func (v Value) RangeText() string {
	switch {
	case v.Min != nil && v.Max != nil:
		return FormatValue(v.Min) + ".." + FormatValue(v.Max)
	case v.Min != nil:
		return ">= " + FormatValue(v.Min)
	case v.Max != nil:
		return "<= " + FormatValue(v.Max)
	}
	return ""
}

// FormatValue renders a Default, Min or Max element as plain text.
func FormatValue(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case time.Duration:
		return x.String()
	default:
		return fmt.Sprint(x)
	}
}
