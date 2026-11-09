// Package golden compares test output with files under testdata/golden.
// Run the tests with -update to rewrite the files.
package golden

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Assert compares got with the golden file name, relative to the repository's
// testdata/golden directory. dir is the path from the test's package to the
// repository root.
func Assert(t *testing.T, root, name string, got []byte) {
	t.Helper()
	path := filepath.Join(root, "testdata", "golden", name)
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
