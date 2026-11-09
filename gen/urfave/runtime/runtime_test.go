package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// checkArgs runs CheckArgs on a command invoked with args.
func runCheckArgs(t *testing.T, specs []ocliArg, args ...string) error {
	t.Helper()
	var got error
	cmd := &cli.Command{
		Name: "x",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			got = ocliCheckArgs(cmd, specs)
			return nil
		},
	}
	if err := cmd.Run(context.Background(), append([]string{"x"}, args...)); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestCheckArgs(t *testing.T) {
	// <input> [<extra>...] with extra in {a, b}, up to 2.
	specs := []ocliArg{
		{Name: "<input>", Min: 1, Max: 1},
		{Name: "<extra>", Min: 0, Max: 2, Check: ocliOneOf("<extra>", "a", "b")},
	}
	cases := []struct {
		args []string
		want string // "" for success, otherwise an error substring
	}{
		{[]string{"f"}, ""},
		{[]string{"f", "a", "b"}, ""},
		{nil, "expected at least 1 argument(s), got 0"},
		{[]string{"f", "a", "b", "a"}, "expected at most 3 argument(s), got 4"},
		{[]string{"f", "z"}, `x: invalid value "z" for <extra> (accepted: a, b)`},
		// The first argument is never checked against <extra>'s values.
		{[]string{"z"}, ""},
	}
	for _, c := range cases {
		err := runCheckArgs(t, specs, c.args...)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("args %q: error = %v, want %q", c.args, err, c.want)
		}
	}
}

func TestCheckArgsReservesMinimums(t *testing.T) {
	// [<a>...] <b>: a greedy <a> must leave one value for <b>.
	var seen []string
	record := func(name string) func(string) error {
		return func(v string) error { seen = append(seen, name+"="+v); return nil }
	}
	specs := []ocliArg{
		{Name: "<a>", Min: 0, Max: 3, Check: record("a")},
		{Name: "<b>", Min: 1, Max: 1, Check: record("b")},
	}
	if err := runCheckArgs(t, specs, "1", "2"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(seen, " "); got != "a=1 b=2" {
		t.Errorf("assignment = %q, want %q", got, "a=1 b=2")
	}
}

func TestValidators(t *testing.T) {
	type check struct {
		name string
		err  error
		want string // "" for success
	}
	between := ocliBetween[int]("-k", 1, true, 9, true)
	cases := []check{
		{"between ok", between(5), ""},
		{"between low", between(0), "invalid value 0 for -k (minimum: 1)"},
		{"between high", between(10), "invalid value 10 for -k (maximum: 9)"},
		{"open bound", ocliBetween[float64]("--f", 0, true, 0, false)(1e9), ""},
		{"each", ocliEach(between)([]int{1, 12}), "maximum: 9"},
		{"one of", ocliOneOf("--s", "a", "b")("c"), `invalid value "c" for --s (accepted: a, b)`},
		{"bool", ocliBool("--salt")("maybe"), "expected true or false"},
		{"bool ok", ocliBool("--salt")("TRUE"), ""},
		{"int", ocliInt("<n>", nil)("x"), `invalid value "x" for <n> (expected an integer)`},
		{"int range", ocliInt("<n>", ocliBetween[int]("<n>", 1, true, 0, false))("0"), "minimum: 1"},
		{"uint", ocliUint("<n>", nil)("-1"), "expected a non-negative integer"},
		{"float", ocliFloat("<f>", nil)("1.5"), ""},
		{"duration", ocliDuration("<d>")("5x"), `expected a duration such as "1m30s"`},
	}
	for _, c := range cases {
		if c.want == "" && c.err != nil || c.want != "" && (c.err == nil || !strings.Contains(c.err.Error(), c.want)) {
			t.Errorf("%s: error = %v, want %q", c.name, c.err, c.want)
		}
	}
}
