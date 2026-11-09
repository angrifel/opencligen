package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angrifel/opencligen/internal/selfgen"
)

const kitchen = "../../specs/golden/kitchen/spec.json"

func runCLI(args ...string) (code int, stdout, stderr string) {
	var out, errw bytes.Buffer
	code = run(args, &out, &errw)
	return code, out.String(), errw.String()
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"opencli": "0.1", "info": {"title": "t"}, "command": {"name": "t"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args       []string
		code       int
		stdoutWant string
		stderrWant string
	}{
		{nil, exitOK, "COMMANDS:", ""},
		{[]string{"help"}, exitOK, "COMMANDS:", ""},
		{[]string{"--help"}, exitOK, "Invalid usage", ""}, // exit codes from the spec are in the help
		{[]string{"--version"}, exitOK, "(OpenCLI 0.1)", ""},
		{[]string{"frobnicate"}, exitUsage, "", "No help topic for 'frobnicate'"},
		{[]string{"validate"}, exitUsage, "", "opencligen validate: expected at least 1 argument(s), got 0"},
		{[]string{"validate", "a", "b"}, exitUsage, "", "expected at most 1 argument(s), got 2"},
		{[]string{"validate", "--nope", kitchen}, exitUsage, "", "flag provided but not defined: -nope"},
		{[]string{"validate", kitchen}, exitOK, "valid OpenCLI 0.1 description", ""},
		{[]string{"validate", "nosuch.json"}, exitInvalid, "", "no such file or directory"},
		{[]string{"validate", bad}, exitInvalid, "", "bad.json:1:28: error: does not conform to the OpenCLI schema: missing property 'version'"},
		{[]string{"validate", "../../testdata/upstream/ffmpeg.json"}, exitInvalid, "", "is a C limit constant"},
		// Global flags work before and after the command.
		{[]string{"validate", "--lenient", "--allow-c-limits", "../../testdata/upstream/ffmpeg.json"}, exitOK, "valid", "warning:"},
		{[]string{"--lenient", "--allow-c-limits", "validate", "../../testdata/upstream/ffmpeg.json"}, exitOK, "valid", "warning:"},
		{[]string{"urfave", "--package", "x-y", kitchen}, exitInvalid, "", `invalid package name "x-y"`},
		{[]string{"validate", "--help"}, exitOK, "--allow-c-limits", ""},
	}
	for _, c := range cases {
		code, stdout, stderr := runCLI(c.args...)
		if code != c.code || !strings.Contains(stdout, c.stdoutWant) || !strings.Contains(stderr, c.stderrWant) {
			t.Errorf("opencligen %s: code %d, stdout %q, stderr %q; want code %d, stdout ~%q, stderr ~%q",
				strings.Join(c.args, " "), code, firstLine(stdout), firstLine(stderr), c.code, c.stdoutWant, c.stderrWant)
		}
	}
}

func TestOutputs(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		args []string
		file string
		want string
	}{
		{[]string{"urfave"}, "k/gen.go", "package cli"},
		{[]string{"urfave", "--package", "kit", "--source", "kitchen.json"}, "k/kit.go", "package kit"},
		{[]string{"markdown"}, "k/README.md", "# Kitchen"},
		{[]string{"markdown", "--hidden"}, "k/ALL.md", "`kitchen debug`"},
		{[]string{"man"}, "k/kitchen.1", `.TH "KITCHEN" "1" ""`},
		{[]string{"man", "--section", "8", "--date", "2026-01-02"}, "k/kitchen.8", `.TH "KITCHEN" "8" "2026-01-02"`},
	} {
		out := filepath.Join(dir, c.file)
		args := append(append(c.args, "-o", out), kitchen)
		if code, _, stderr := runCLI(args...); code != exitOK {
			t.Fatalf("%v: exit %d: %s", c.args, code, stderr)
		}
		data, err := os.ReadFile(out)
		if err != nil || !strings.Contains(string(data), c.want) {
			t.Errorf("%v: %v, output lacks %q", c.args, err, c.want)
		}
	}
	// The default source in the urfave header is the SPEC path as given.
	data, _ := os.ReadFile(filepath.Join(dir, "k/gen.go"))
	if !strings.Contains(string(data), "from ../../specs/golden/kitchen/spec.json.") {
		t.Errorf("header does not name the spec: %s", firstLine(string(data)))
	}
	data, _ = os.ReadFile(filepath.Join(dir, "k/kit.go"))
	if !strings.Contains(string(data), "from kitchen.json.") {
		t.Errorf("--source not used: %s", firstLine(string(data)))
	}
	// Standard output is the default.
	if code, stdout, _ := runCLI("markdown", kitchen); code != exitOK || !strings.HasPrefix(stdout, "# Kitchen") {
		t.Errorf("markdown to stdout: exit %d, %q", code, firstLine(stdout))
	}
}

func TestUnchangedOutputIsNotRewritten(t *testing.T) {
	out := filepath.Join(t.TempDir(), "gen.go")
	if code, _, stderr := runCLI("urfave", "-o", out, kitchen); code != exitOK {
		t.Fatal(stderr)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(out, old, old); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCLI("urfave", "-o", out, kitchen); code != exitOK {
		t.Fatal(stderr)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(old) {
		t.Errorf("unchanged output was rewritten")
	}
}

// TestGeneratedFilesUpToDate checks that the committed code and docs
// generated from specs/opencligen.json match the spec and the current
// generators. Run "make generate" to update them.
func TestGeneratedFilesUpToDate(t *testing.T) {
	files, err := selfgen.Files("../..")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Errorf("%v; run make generate", err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is out of date with %s; run make generate", name, selfgen.Spec)
		}
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
