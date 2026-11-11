package urfave_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angrifel/opencligen/internal/golden"
	"github.com/angrifel/opencligen/internal/opencli/ast"
	"github.com/angrifel/opencligen/internal/opencli/parser"
)

// TestReferenceRuns builds a reference CLI for every golden case that has a
// runs.txt and replays its command lines. The reference CLI is the
// generated code plus a generated Handlers implementation: each handler
// prints the handler it is, its positional arguments, and the value of every
// flag that applies to its command. See specs/golden/README.md for the
// runs.txt format.
func TestReferenceRuns(t *testing.T) {
	type refCase struct {
		golden.Case
		runs []run
	}
	var cases []refCase
	for _, c := range golden.Cases(t, "../../..") {
		path := filepath.Join(filepath.Dir(c.Spec), "runs.txt")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		runs, err := parseRuns(string(data))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		cases = append(cases, refCase{c, runs})
	}
	if len(cases) == 0 {
		t.Fatal("no golden case has a runs.txt")
	}

	s := newScratch(t)
	for _, c := range cases {
		m, _, err := parser.LoadFile(c.Spec, parser.Options{})
		if err != nil {
			t.Fatal(err)
		}
		s.generate(m, c.Package, false)
		s.write("cmd/"+c.Package+"/main.go", referenceMain(m, c.Package))
	}
	bin := filepath.Join(s.dir, "bin")
	s.gocmd("vet", "./...")
	s.gocmd("build", "-o", bin+string(filepath.Separator), "./cmd/...")

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			for _, r := range c.runs {
				r.check(t, filepath.Join(bin, c.Package))
			}
		})
	}
}

// run is one command line of a runs.txt and what it must produce.
type run struct {
	line   int
	args   []string
	stdout []string // lines that must appear in standard output
	output []string // text that must appear in standard output
	errors []string // text that must appear in standard error; the run must fail
}

// parseRuns reads the runs.txt format: blocks that start with "$ args",
// followed by expectation lines; "#" starts a comment line.
func parseRuns(src string) ([]run, error) {
	var runs []run
	sc := bufio.NewScanner(strings.NewReader(src))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimRight(sc.Text(), " \t")
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "$"):
			runs = append(runs, run{line: n, args: strings.Fields(strings.TrimPrefix(line, "$"))})
		case len(runs) == 0:
			return nil, fmt.Errorf("line %d: expectation before the first $ line", n)
		case strings.HasPrefix(line, "error: "):
			runs[len(runs)-1].errors = append(runs[len(runs)-1].errors, strings.TrimPrefix(line, "error: "))
		case strings.HasPrefix(line, "output: "):
			runs[len(runs)-1].output = append(runs[len(runs)-1].output, strings.TrimPrefix(line, "output: "))
		default:
			runs[len(runs)-1].stdout = append(runs[len(runs)-1].stdout, line)
		}
	}
	return runs, sc.Err()
}

func (r run) check(t *testing.T, bin string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, r.args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	where := fmt.Sprintf("runs.txt:%d: $ %s", r.line, strings.Join(r.args, " "))
	show := func() string { return "\nstdout:\n" + stdout.String() + "stderr:\n" + stderr.String() }

	if len(r.errors) == 0 && err != nil {
		t.Errorf("%s: failed: %v%s", where, err, show())
		return
	}
	if len(r.errors) > 0 && err == nil {
		t.Errorf("%s: succeeded, want an error%s", where, show())
		return
	}
	lines := strings.Split(stdout.String(), "\n")
	for _, want := range r.stdout {
		found := false
		for _, l := range lines {
			if strings.TrimRight(l, " \t") == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: no output line %q%s", where, want, show())
		}
	}
	for _, want := range r.output {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("%s: output lacks %q%s", where, want, show())
		}
	}
	for _, want := range r.errors {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("%s: error output lacks %q%s", where, want, show())
		}
	}
}

// referenceMain generates the reference Handlers implementation and main
// function for a golden case.
func referenceMain(m *ast.Model, pkg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	%q
)

type handlers struct{}

// report prints what the handler received, one item per line.
func report(cmd *cli.Command, handler string, flags ...string) error {
	fmt.Println("handler:", handler)
	fmt.Printf("args: %%v\n", cmd.Args().Slice())
	for _, f := range flags {
		fmt.Printf("--%%s: %%v\n", f, cmd.Value(f))
	}
	return nil
}

func main() {
	if err := %s.New(handlers{}).Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
`, "example.com/scratch/"+pkg, pkg)
	m.Root.Walk(func(c *ast.Command) {
		if !c.Runnable() {
			return
		}
		var flags []string
		for _, o := range append(append([]*ast.Option(nil), c.Options...), c.InheritedOptions()...) {
			if !builtin(m, c, o) && !builtin(m, m.Root, o) {
				flags = append(flags, fmt.Sprintf("%q", o.Names[0]))
			}
		}
		method := wantHandler(c)
		fmt.Fprintf(&b, "\nfunc (handlers) %s(ctx context.Context, cmd *cli.Command) error {\n\treturn report(cmd, %q", method, method)
		for _, f := range flags {
			b.WriteString(", " + f)
		}
		b.WriteString(")\n}\n")
	})
	return b.String()
}
