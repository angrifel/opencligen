// Package selfgen generates opencligen's own CLI code and docs from
// specs/opencligen.json.
//
// It calls the generator packages directly instead of running the opencligen
// command, so regeneration works even when the committed generated code no
// longer compiles (for example after a change to what the urfave output
// emits). "make generate" runs it through ./internal/selfgen/cmd, and
// cmd/opencligen's tests use it to check that the committed files are up to
// date.
package selfgen

import (
	"path/filepath"

	"github.com/angrifel/opencligen/gen/man"
	"github.com/angrifel/opencligen/gen/markdown"
	"github.com/angrifel/opencligen/gen/urfave"
	"github.com/angrifel/opencligen/spec"
)

// Spec is the description of opencligen, relative to the repository root.
const Spec = "specs/opencligen.json"

// Files returns the generated files keyed by their path relative to the
// repository root, which is root.
func Files(root string) (map[string][]byte, error) {
	m, _, err := spec.LoadFile(filepath.Join(root, Spec), spec.Options{})
	if err != nil {
		return nil, err
	}
	code, _, err := urfave.Generate(m, urfave.Options{Package: "app", Source: Spec})
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"cmd/opencligen/internal/app/app.gen.go": code,
		"docs/cli.md":                            markdown.Generate(m, markdown.Options{}),
		"docs/opencligen.1":                      man.Generate(m, man.Options{}),
	}, nil
}
