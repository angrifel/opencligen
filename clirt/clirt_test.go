package clirt_test

import (
	"context"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/angrifel/opencligen/clirt"
)

// checkArgs runs CheckArgs on a command invoked with args.
func checkArgs(t *testing.T, specs []clirt.Arg, args ...string) error {
	t.Helper()
	var got error
	cmd := &cli.Command{
		Name: "x",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			got = clirt.CheckArgs(cmd, specs)
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
	specs := []clirt.Arg{
		{Name: "<input>", Min: 1, Max: 1},
		{Name: "<extra>", Min: 0, Max: 2, Check: clirt.OneOf("<extra>", "a", "b")},
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
		err := checkArgs(t, specs, c.args...)
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
	specs := []clirt.Arg{
		{Name: "<a>", Min: 0, Max: 3, Check: record("a")},
		{Name: "<b>", Min: 1, Max: 1, Check: record("b")},
	}
	if err := checkArgs(t, specs, "1", "2"); err != nil {
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
	between := clirt.Between[int]("-k", 1, true, 9, true)
	cases := []check{
		{"between ok", between(5), ""},
		{"between low", between(0), "invalid value 0 for -k (minimum: 1)"},
		{"between high", between(10), "invalid value 10 for -k (maximum: 9)"},
		{"open bound", clirt.Between[float64]("--f", 0, true, 0, false)(1e9), ""},
		{"each", clirt.Each(between)([]int{1, 12}), "maximum: 9"},
		{"one of", clirt.OneOf("--s", "a", "b")("c"), `invalid value "c" for --s (accepted: a, b)`},
		{"bool", clirt.Bool("--salt")("maybe"), "expected true or false"},
		{"bool ok", clirt.Bool("--salt")("TRUE"), ""},
		{"int", clirt.Int("<n>", nil)("x"), `invalid value "x" for <n> (expected an integer)`},
		{"int range", clirt.Int("<n>", clirt.Between[int]("<n>", 1, true, 0, false))("0"), "minimum: 1"},
		{"uint", clirt.Uint("<n>", nil)("-1"), "expected a non-negative integer"},
		{"float", clirt.Float("<f>", nil)("1.5"), ""},
		{"duration", clirt.Duration("<d>")("5x"), `expected a duration such as "1m30s"`},
	}
	for _, c := range cases {
		if c.want == "" && c.err != nil || c.want != "" && (c.err == nil || !strings.Contains(c.err.Error(), c.want)) {
			t.Errorf("%s: error = %v, want %q", c.name, c.err, c.want)
		}
	}
}
