// Package golden compares test output with the golden cases under
// specs/golden/<case>/.
// Run the tests with -update to rewrite the files.
package golden

import (
	"bytes"
	"flag"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Case is a golden case: a directory under specs/golden holding spec.json and
// the expected output of each generator.
type Case struct {
	Name    string // directory name, such as "all-features"
	Package string // Go package name for generated code: Name without hyphens
	Spec    string // path to spec.json from the test's package
}

// Cases lists the golden cases. root is the path from the test's package to
// the repository root.
func Cases(t *testing.T, root string) []Case {
	t.Helper()
	dir := filepath.Join(root, "specs", "golden")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var cases []Case
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		spec := filepath.Join(dir, e.Name(), "spec.json")
		if _, err := os.Stat(spec); err != nil {
			t.Fatalf("golden case %s: %v", e.Name(), err)
		}
		pkg := strings.ReplaceAll(e.Name(), "-", "")
		if !token.IsIdentifier(pkg) {
			t.Fatalf("golden case %s: %q is not a valid Go package name", e.Name(), pkg)
		}
		cases = append(cases, Case{Name: e.Name(), Package: pkg, Spec: spec})
	}
	if len(cases) == 0 {
		t.Fatalf("no golden cases in %s", dir)
	}
	return cases
}

// Assert compares got with the golden file name ("<case>/<file>"), relative
// to the repository's specs/golden directory. root is the path from the
// test's package to the repository root.
func Assert(t *testing.T, root, name string, got []byte) {
	t.Helper()
	path := filepath.Join(root, "specs", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s (run with -update to accept):\n%s", path, diff(want, got))
	}
}

// diff shows the first differing line with some context.
func diff(want, got []byte) string {
	wl := bytes.Split(want, []byte("\n"))
	gl := bytes.Split(got, []byte("\n"))
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g []byte
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if !bytes.Equal(w, g) {
			return "line " + itoa(i+1) + ":\n  want: " + string(w) + "\n  got:  " + string(g)
		}
	}
	return "(no line difference)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
