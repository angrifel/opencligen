package urfave_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angrifel/opencligen/gen/urfave"
	"github.com/angrifel/opencligen/spec"
)

// kitchenMain implements the kitchen Handlers by printing what each handler
// received.
const kitchenMain = `package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"example.com/e2e/kitchen"
)

type handlers struct{}

func (handlers) Cook(ctx context.Context, c *cli.Command) error {
	fmt.Printf("cook args=%v heat=%d timer=%s style=%s weights=%v salt=%s batch=%d verbose=%t\n",
		c.Args().Slice(), c.Int("heat"), c.Duration("timer"), c.String("style"), c.FloatSlice("weights"),
		c.String("salt"), c.Uint("batch"), c.Bool("verbose"))
	return nil
}

func (handlers) Pantry(ctx context.Context, c *cli.Command) error {
	fmt.Printf("pantry args=%v\n", c.Args().Slice())
	return nil
}

func (handlers) AddToPantry(ctx context.Context, c *cli.Command) error {
	fmt.Printf("add args=%v\n", c.Args().Slice())
	return nil
}

func (handlers) Debug(ctx context.Context, c *cli.Command) error { return nil }

func main() {
	if err := kitchen.New(handlers{}).Run(context.Background(), os.Args); err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
}
`

// TestEndToEnd generates code into a scratch module that depends only on
// urfave/cli, builds it and runs the kitchen CLI; it also compiles the code
// generated for the upstream examples.
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go code")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not found")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	// The scratch module requires only urfave/cli: generated code must not
	// depend on opencligen.
	write("go.mod", "module example.com/e2e\n\ngo 1.24\n\nrequire github.com/urfave/cli/v3 v3.14.0\n")
	write("go.sum", string(sum))
	write("main.go", kitchenMain)

	generate := func(specPath, pkg string, opts spec.Options) {
		t.Helper()
		m, _, err := spec.LoadFile(specPath, opts)
		if err != nil {
			t.Fatal(err)
		}
		src, _, err := urfave.Generate(m, urfave.Options{Package: pkg, Lenient: opts.Lenient})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "angrifel/opencligen") {
			t.Errorf("%s: generated code refers to opencligen", pkg)
		}
		write(pkg+"/gen.go", string(src))
	}
	generate(filepath.Join(root, "specs/golden/all-features/spec.json"), "kitchen", spec.Options{})
	generate(filepath.Join(root, "testdata/upstream/git.json"), "git", spec.Options{})
	generate(filepath.Join(root, "testdata/upstream/curl.json"), "curl", spec.Options{})
	generate(filepath.Join(root, "testdata/upstream/ffmpeg.json"), "ffmpeg", spec.Options{Lenient: true, AllowCLimits: true})

	gocmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command(goBin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	bin := filepath.Join(dir, "kitchen-bin")
	gocmd("build", "-o", bin, ".")
	gocmd("vet", "./...")

	cases := []struct {
		args string
		want string // substring of the combined output
	}{
		{"cook --batch 1 pasta", "cook args=[pasta] heat=5 timer=1m30s style=plain weights=[0.5 2] salt=true batch=1 verbose=false"},
		{"c --batch 2 -v --heat 9 -w 1 -w 3 --salt false soup 4", "cook args=[soup 4] heat=9 timer=1m30s style=plain weights=[1 3] salt=false batch=2 verbose=true"},
		{"-v cook --batch 1 salad", "verbose=true"},
		{"cook --batch 1 --heat 10 pasta", "invalid value 10 for --heat (maximum: 9)"},
		{"cook --batch 1 --style ugly pasta", `invalid value "ugly" for --style (accepted: plain, fancy)`},
		{"cook --batch 1 --salt maybe pasta", `invalid value "maybe" for --salt (expected true or false)`},
		{"cook --batch 1 --timer 5x pasta", `invalid value "5x" for flag -timer`},
		{"cook --batch 1 rice", `kitchen cook: invalid value "rice" for <dish> (accepted: pasta, soup, salad)`},
		{"cook --batch 1 pasta 13", `kitchen cook: invalid value 13 for <servings> (maximum: 12)`},
		{"cook --batch 1 pasta lots", `invalid value "lots" for <servings> (expected an integer)`},
		{"cook pasta", `Required flag "batch" not set`},
		{"pantry", "pantry args=[]"},
		{"pantry a b c", "pantry args=[a b c]"},
		{"pantry a b c d", "expected at most 3 argument(s), got 4"},
		{"pantry add x y", "add args=[x y]"},
		{"pantry add", "kitchen pantry add: expected at least 1 argument(s), got 0"},
		{"--version", "kitchen version 1.2.3"},
		{"cook --help", "Heat level (default: 5)"},
	}
	for _, c := range cases {
		out, _ := exec.Command(bin, strings.Fields(c.args)...).CombinedOutput()
		if !strings.Contains(string(out), c.want) {
			t.Errorf("kitchen %s:\n%s\nwant it to contain %q", c.args, out, c.want)
		}
	}
}
