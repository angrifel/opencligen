package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/angrifel/opencligen/internal/opencli/ast"
)

// typeAliases maps other common type names onto the supported ones.
var typeAliases = map[string]ast.Type{
	"int64":    ast.Int,
	"unsigned": ast.Uint,
	"uint64":   ast.Uint,
	"double":   ast.Float,
	"bool":     ast.Bool,
}

var types = []ast.Type{ast.String, ast.Int, ast.Uint, ast.Float, ast.Bool, ast.Duration}

// cLimits are the C limit names accepted as x-range bounds or numeric
// x-default values with Options.AllowCLimits, as used by the upstream ffmpeg
// example.
var cLimits = map[string]string{
	"INT_MIN":    "-2147483648",
	"INT_MAX":    "2147483647",
	"UINT_MAX":   "4294967295",
	"INT32_MIN":  "-2147483648",
	"INT32_MAX":  "2147483647",
	"UINT32_MAX": "4294967295",
	"I64_MIN":    "-9223372036854775808",
	"I64_MAX":    "9223372036854775807",
	"INT64_MIN":  "-9223372036854775808",
	"INT64_MAX":  "9223372036854775807",
	"UINT64_MAX": "18446744073709551615",
	"FLT_MAX":    "3.40282346638528859811704183484516925440e+38",
	"FLT_MIN":    "1.17549435082228750796873653722224568e-38",
	"DBL_MAX":    "1.79769313486231570814527423731704357e+308",
	"DBL_MIN":    "2.22507385850720138309023271733240406e-308",
}

// valueError is an x- metadata problem; index is the metadata entry it is
// about, or -1.
type valueError struct {
	index int
	msg   string
}

func (e *valueError) Error() string { return e.msg }

func verr(index int, format string, args ...any) *valueError {
	return &valueError{index: index, msg: fmt.Sprintf(format, args...)}
}

// parseValue resolves a Value from an object's metadata. takesValue is false
// for presence-only options; multi and accepted come from the arguments.
func parseValue(meta ast.Metadata, takesValue, multi bool, accepted []string, opts Options) (ast.Value, *valueError) {
	v := ast.Value{TakesValue: takesValue, Multi: multi, Accepted: accepted, Type: ast.Bool}
	if takesValue {
		v.Type = ast.String
	}

	idx := map[string]int{}
	for i, m := range meta {
		if m.Name != "x-type" && m.Name != "x-default" && m.Name != "x-range" {
			continue
		}
		if _, dup := idx[m.Name]; dup {
			return v, verr(i, "%s is declared more than once", m.Name)
		}
		idx[m.Name] = i
	}

	if i, ok := idx["x-type"]; ok {
		var name string
		if err := json.Unmarshal(meta[i].Value, &name); err != nil {
			return v, verr(i, "x-type must be a string")
		}
		t := ast.Type(name)
		if a, ok := typeAliases[name]; ok {
			t = a
		}
		if !slices.Contains(types, t) {
			return v, verr(i, "unsupported x-type %q (supported: string, int, uint, float, boolean, duration)", name)
		}
		v.Type, v.Explicit = t, true
		switch {
		case !takesValue && t != ast.Bool:
			return v, verr(i, "x-type %q needs the option to take an argument", name)
		case multi && (t == ast.Bool || t == ast.Duration):
			return v, verr(i, "x-type %q cannot be used for several values", name)
		case len(accepted) > 0 && t != ast.String:
			return v, verr(i, "acceptedValues can only be used with x-type string, not %q", name)
		}
	}

	if i, ok := idx["x-range"]; ok {
		if err := parseRange(&v, meta[i].Value, opts); err != nil {
			return v, verr(i, "%v", err)
		}
	}

	if i, ok := idx["x-default"]; ok {
		raw := meta[i].Value
		// Decode into a fresh slice: unmarshalling into one that already
		// holds raw would reuse, and overwrite, the document's bytes.
		var items []json.RawMessage
		if !multi {
			items = []json.RawMessage{raw}
		} else if err := json.Unmarshal(raw, &items); err != nil {
			return v, verr(i, "x-default must be an array for several values")
		}
		v.Default = []any{}
		for _, item := range items {
			d, err := decode(v, item, opts)
			if err != nil {
				return v, verr(i, "x-default: %v", err)
			}
			v.Default = append(v.Default, d)
		}
	}
	return v, nil
}

func parseRange(v *ast.Value, raw json.RawMessage, opts Options) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r struct {
		Minimum json.RawMessage `json:"minimum"`
		Maximum json.RawMessage `json:"maximum"`
	}
	if err := dec.Decode(&r); err != nil {
		return fmt.Errorf("x-range must be an object with a minimum and/or maximum: %v", err)
	}
	if r.Minimum == nil && r.Maximum == nil {
		return fmt.Errorf("x-range needs a minimum or a maximum")
	}
	if !v.Numeric() {
		return fmt.Errorf("x-range can only be used with x-type int, uint or float, not %q", v.Type)
	}
	var err error
	if r.Minimum != nil {
		if v.Min, err = parseNumber(*v, r.Minimum, opts); err != nil {
			return fmt.Errorf("x-range minimum: %v", err)
		}
	}
	if r.Maximum != nil {
		if v.Max, err = parseNumber(*v, r.Maximum, opts); err != nil {
			return fmt.Errorf("x-range maximum: %v", err)
		}
	}
	if v.Min != nil && v.Max != nil && compareNumbers(v.Min, v.Max) > 0 {
		return fmt.Errorf("x-range minimum %s is greater than maximum %s", ast.FormatValue(v.Min), ast.FormatValue(v.Max))
	}
	return nil
}

// scalarText returns the text of a JSON number or string.
func scalarText(raw json.RawMessage) (string, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var x any
	if err := dec.Decode(&x); err != nil {
		return "", false
	}
	switch y := x.(type) {
	case json.Number:
		return y.String(), true
	case string:
		return strings.TrimSpace(y), true
	}
	return "", false
}

// ParseNumber parses text as a number of type t, returning an int64, uint64
// or float64. Integral float notation such as "2.14748e+09" is accepted for
// integer types.
func ParseNumber(t ast.Type, text string) (any, error) {
	bad := fmt.Errorf("%q is not a valid %s", text, t)
	switch t {
	case ast.Int:
		if i, err := strconv.ParseInt(text, 10, 64); err == nil {
			return i, nil
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || f != math.Trunc(f) || f < math.MinInt64 || f >= math.MaxInt64 {
			return nil, bad
		}
		return int64(f), nil
	case ast.Uint:
		if u, err := strconv.ParseUint(text, 10, 64); err == nil {
			return u, nil
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || f != math.Trunc(f) || f < 0 || f >= math.MaxUint64 {
			return nil, bad
		}
		return uint64(f), nil
	case ast.Float:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, bad
		}
		return f, nil
	}
	return nil, fmt.Errorf("type %q is not numeric", t)
}

// parseNumber parses a JSON number, numeric string or, with AllowCLimits, a
// possibly negated C limit constant.
func parseNumber(v ast.Value, raw json.RawMessage, opts Options) (any, error) {
	text, ok := scalarText(raw)
	if !ok {
		return nil, fmt.Errorf("%s is not a number", raw)
	}
	resolved := text
	name, neg := strings.CutPrefix(text, "-")
	if c, ok := cLimits[name]; ok {
		if !opts.AllowCLimits {
			return nil, fmt.Errorf("%q is a C limit constant; enable AllowCLimits (--allow-c-limits) to accept it", text)
		}
		resolved = c
		if neg {
			if pos, wasNeg := strings.CutPrefix(c, "-"); wasNeg {
				resolved = pos
			} else {
				resolved = "-" + c
			}
		}
	}
	n, err := ParseNumber(v.Type, resolved)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid %s", text, v.Type)
	}
	return n, nil
}

func compareNumbers(a, b any) int {
	switch x := a.(type) {
	case int64:
		return cmpOrdered(x, b.(int64))
	case uint64:
		return cmpOrdered(x, b.(uint64))
	case float64:
		return cmpOrdered(x, b.(float64))
	}
	panic(fmt.Sprintf("compareNumbers: unexpected %T", a))
}

func cmpOrdered[T int64 | uint64 | float64](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// decode decodes one default value and checks it against the type, accepted
// values and range.
func decode(v ast.Value, raw json.RawMessage, opts Options) (any, error) {
	switch v.Type {
	case ast.String:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("%s is not a valid string", raw)
		}
		if len(v.Accepted) > 0 && !slices.Contains(v.Accepted, s) {
			return nil, fmt.Errorf("%q is not one of the accepted values (%s)", s, strings.Join(v.Accepted, ", "))
		}
		return s, nil
	case ast.Bool:
		var b bool
		if err := json.Unmarshal(raw, &b); err == nil {
			return b, nil
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if b, err := strconv.ParseBool(s); err == nil {
				return b, nil
			}
		}
		return nil, fmt.Errorf("%s is not a valid boolean", raw)
	case ast.Duration:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("%s is not a duration string such as \"1m30s\"", raw)
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, err
		}
		return d, nil
	}

	n, err := parseNumber(v, raw, opts)
	if err != nil {
		return nil, err
	}
	if v.Min != nil && compareNumbers(n, v.Min) < 0 {
		return nil, fmt.Errorf("%s is below the x-range minimum %s", ast.FormatValue(n), ast.FormatValue(v.Min))
	}
	if v.Max != nil && compareNumbers(n, v.Max) > 0 {
		return nil, fmt.Errorf("%s is above the x-range maximum %s", ast.FormatValue(n), ast.FormatValue(v.Max))
	}
	return n, nil
}
