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

// scratch is a throwaway Go module for building generated code. It requires
// only urfave/cli: generated code must not depend on opencligen.
type scratch struct {
	t     *testing.T
	dir   string
	goBin string
}

func newScratch(t *testing.T) *scratch {
	t.Helper()
	if testing.Short() {
		t.Skip("builds Go code")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not found")
	}
	sum, err := os.ReadFile("../../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	s := &scratch{t: t, dir: t.TempDir(), goBin: goBin}
	s.write("go.mod", "module example.com/scratch\n\ngo 1.24\n\nrequire github.com/urfave/cli/v3 v3.14.0\n")
	s.write("go.sum", string(sum))
	return s
}

func (s *scratch) write(name, content string) {
	s.t.Helper()
	path := filepath.Join(s.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

// generate writes the code generated for a description into package pkg
// and returns the source.
func (s *scratch) generate(m *spec.Model, pkg string, lenient bool) []byte {
	s.t.Helper()
	src, _, err := urfave.Generate(m, urfave.Options{Package: pkg, Lenient: lenient})
	if err != nil {
		s.t.Fatal(err)
	}
	if strings.Contains(string(src), "angrifel/opencligen") {
		s.t.Errorf("%s: generated code refers to opencligen", pkg)
	}
	s.write(pkg+"/gen.go", string(src))
	return src
}

// gocmd runs the go command in the module, offline.
func (s *scratch) gocmd(args ...string) {
	s.t.Helper()
	cmd := exec.Command(s.goBin, args...)
	cmd.Dir = s.dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		s.t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// build vets the module and builds its main package, returning the binary.
func (s *scratch) build() string {
	s.t.Helper()
	bin := filepath.Join(s.dir, "bin")
	s.gocmd("vet", "./...")
	s.gocmd("build", "-o", bin, ".")
	return bin
}
