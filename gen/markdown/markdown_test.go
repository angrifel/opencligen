package markdown_test

import (
	"strings"
	"testing"

	"github.com/angrifel/opencligen/gen/markdown"
	"github.com/angrifel/opencligen/internal/golden"
	"github.com/angrifel/opencligen/spec"
)

func TestGolden(t *testing.T) {
	m, _, err := spec.LoadFile("../../testdata/kitchen.json", spec.Options{})
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "../..", "kitchen.md.golden", markdown.Generate(m, markdown.Options{}))

	all := string(markdown.Generate(m, markdown.Options{IncludeHidden: true}))
	for _, s := range []string{"`kitchen debug`", "`--secret`"} {
		if !strings.Contains(all, s) {
			t.Errorf("IncludeHidden output lacks %s", s)
		}
	}
}

func TestUpstream(t *testing.T) {
	for _, f := range []string{"git.json", "curl.json"} {
		m, _, err := spec.LoadFile("../../testdata/upstream/"+f, spec.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if out := markdown.Generate(m, markdown.Options{}); len(out) == 0 {
			t.Errorf("%s: empty output", f)
		}
	}
}
