package man_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/angrifel/opencligen/gen/man"
	"github.com/angrifel/opencligen/internal/golden"
	"github.com/angrifel/opencligen/spec"
)

// TestGolden renders every case in specs/golden.
func TestGolden(t *testing.T) {
	for _, c := range golden.Cases(t, "../..") {
		t.Run(c.Name, func(t *testing.T) {
			m, _, err := spec.LoadFile(c.Spec, spec.Options{})
			if err != nil {
				t.Fatal(err)
			}
			golden.Assert(t, "../..", c.Name+"/man.1.golden", man.Generate(m, man.Options{}))
		})
	}
}

// TestGroff renders pages with groff, when installed, and fails on warnings.
func TestGroff(t *testing.T) {
	groff, err := exec.LookPath("groff")
	if err != nil {
		t.Skip("groff not found")
	}
	files := []string{"../../testdata/upstream/git.json", "../../testdata/upstream/curl.json"}
	for _, c := range golden.Cases(t, "../..") {
		files = append(files, c.Spec)
	}
	for _, f := range files {
		m, _, err := spec.LoadFile(f, spec.Options{})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(groff, "-man", "-ww", "-z")
		cmd.Stdin = bytes.NewReader(man.Generate(m, man.Options{}))
		out, err := cmd.CombinedOutput()
		if err != nil || len(out) > 0 {
			t.Errorf("%s: groff: %v\n%s", f, err, firstLines(string(out), 10))
		}
	}
}

func TestEscaping(t *testing.T) {
	m, _, err := spec.Load("t.json", []byte(`{"opencli": "0.1", "info": {"title": "t", "version": "1"},
		"command": {"name": "t", "description": ".starts with a dot\nback\\slash",
		"options": [{"name": "--dry-run"}]}}`), spec.Options{})
	if err != nil {
		t.Fatal(err)
	}
	out := string(man.Generate(m, man.Options{}))
	for _, want := range []string{`\&.starts with a dot`, `back\eslash`, `\fB\-\-dry\-run\fR`} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
