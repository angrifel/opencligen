// Package runtime holds the helpers that opencligen's urfave/cli output
// copies into every generated file, so generated code depends only on
// urfave/cli and the standard library.
//
// Everything after the import block is copied verbatim. Keep it
// self-contained: unexported identifiers prefixed with "ocli" (to avoid
// clashes with the user's code in the same package), and imports limited to
// those the generated file declares (see urfave.go).
package runtime

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

// ocliArg describes a positional argument: how many values it takes and how
// each value is checked.
type ocliArg struct {
	Name     string
	Min, Max int
	Check    func(string) error // nil accepts any value
}

// ocliCheckArgs enforces the arity and values of a command's positional
// arguments. Values are assigned to arguments in order, each taking as many
// values as it can while leaving enough for the minimums of the arguments
// after it.
func ocliCheckArgs(cmd *cli.Command, specs []ocliArg) error {
	args := cmd.Args().Slice()
	minTotal, maxTotal := 0, 0
	for _, s := range specs {
		minTotal += s.Min
		maxTotal += s.Max
	}
	if len(args) < minTotal {
		return fmt.Errorf("%s: expected at least %d argument(s), got %d", cmd.FullName(), minTotal, len(args))
	}
	if len(args) > maxTotal {
		return fmt.Errorf("%s: expected at most %d argument(s), got %d", cmd.FullName(), maxTotal, len(args))
	}
	i, reserved := 0, minTotal
	for _, s := range specs {
		reserved -= s.Min
		n := min(s.Max, len(args)-i-reserved)
		if s.Check != nil {
			for _, v := range args[i : i+n] {
				if err := s.Check(v); err != nil {
					return fmt.Errorf("%s: %w", cmd.FullName(), err)
				}
			}
		}
		i += n
	}
	return nil
}

// ocliOneOf accepts only the listed values. name is the flag or argument as the
// user sees it, e.g. "--format" or "<mode>".
func ocliOneOf(name string, accepted ...string) func(string) error {
	return func(v string) error {
		if !slices.Contains(accepted, v) {
			return fmt.Errorf("invalid value %q for %s (accepted: %s)", v, name, strings.Join(accepted, ", "))
		}
		return nil
	}
}

// ocliNumber is the set of numeric flag value types.
type ocliNumber interface{ ~int | ~uint | ~float64 }

// ocliBetween accepts values within the given bounds; a bound only applies when
// its has flag is set.
func ocliBetween[T ocliNumber](name string, lo T, hasLo bool, hi T, hasHi bool) func(T) error {
	return func(v T) error {
		if hasLo && v < lo {
			return fmt.Errorf("invalid value %v for %s (minimum: %v)", v, name, lo)
		}
		if hasHi && v > hi {
			return fmt.Errorf("invalid value %v for %s (maximum: %v)", v, name, hi)
		}
		return nil
	}
}

// ocliEach applies check to every value of a repeated flag.
func ocliEach[T any](check func(T) error) func([]T) error {
	return func(vs []T) error {
		for _, v := range vs {
			if err := check(v); err != nil {
				return err
			}
		}
		return nil
	}
}

// ocliBool accepts the values strconv.ParseBool accepts, for boolean options
// that take an explicit value ("-flag true").
func ocliBool(name string) func(string) error {
	return func(v string) error {
		if _, err := strconv.ParseBool(v); err != nil {
			return fmt.Errorf("invalid value %q for %s (expected true or false)", v, name)
		}
		return nil
	}
}

// ocliInt parses a positional value as an int and applies check, if any.
func ocliInt(name string, check func(int) error) func(string) error {
	return ocliParsed(name, "an integer", func(s string) (int, error) { return strconv.Atoi(s) }, check)
}

// ocliUint parses a positional value as a uint and applies check, if any.
func ocliUint(name string, check func(uint) error) func(string) error {
	return ocliParsed(name, "a non-negative integer", func(s string) (uint, error) {
		u, err := strconv.ParseUint(s, 10, 0)
		return uint(u), err
	}, check)
}

// ocliFloat parses a positional value as a float64 and applies check, if any.
func ocliFloat(name string, check func(float64) error) func(string) error {
	return ocliParsed(name, "a number", func(s string) (float64, error) { return strconv.ParseFloat(s, 64) }, check)
}

// ocliDuration parses a positional value with time.ParseDuration.
func ocliDuration(name string) func(string) error {
	return ocliParsed[time.Duration](name, `a duration such as "1m30s"`, time.ParseDuration, nil)
}

func ocliParsed[T any](name, what string, parse func(string) (T, error), check func(T) error) func(string) error {
	return func(s string) error {
		v, err := parse(s)
		if err != nil {
			return fmt.Errorf("invalid value %q for %s (expected %s)", s, name, what)
		}
		if check != nil {
			return check(v)
		}
		return nil
	}
}
