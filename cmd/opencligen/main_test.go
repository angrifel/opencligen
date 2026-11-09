package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const kitchen = "../../testdata/kitchen.json"

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
		{nil, exitUsage, "", "Usage:"},
		{[]string{"help"}, exitOK, "Commands:", ""},
		{[]string{"version"}, exitOK, "OpenCLI versions: [0.1]", ""},
		{[]string{"frobnicate"}, exitUsage, "", `unknown command "frobnicate"`},
		{[]string{"validate"}, exitUsage, "", "Usage: opencligen validate"},
		{[]string{"validate", kitchen}, exitOK, "valid OpenCLI 0.1 description", ""},
		{[]string{"validate", bad}, exitInvalid, "", "bad.json:1:28: error: does not conform to the OpenCLI schema: missing property 'version'"},
		{[]string{"validate", "../../testdata/upstream/ffmpeg.json"}, exitInvalid, "", "is a C limit constant"},
		{[]string{"validate", "-lenient", "-allow-c-limits", "../../testdata/upstream/ffmpeg.json"}, exitOK, "valid", "warning:"},
		{[]string{"urfave", "-package", "x-y", kitchen}, exitInvalid, "", `invalid package name "x-y"`},
		{[]string{"validate", "-h"}, exitOK, "", "-allow-c-limits"},
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
	for _, c := range []struct{ cmd, file, want string }{
		{"urfave", "k/gen.go", "package cli"},
		{"markdown", "k/README.md", "# Kitchen"},
		{"man", "k/kitchen.1", `.TH "KITCHEN" "1"`},
	} {
		out := filepath.Join(dir, c.file)
		if code, _, stderr := runCLI(c.cmd, "-o", out, kitchen); code != exitOK {
			t.Fatalf("%s: exit %d: %s", c.cmd, code, stderr)
		}
		data, err := os.ReadFile(out)
		if err != nil || !strings.Contains(string(data), c.want) {
			t.Errorf("%s: %v, output lacks %q", c.cmd, err, c.want)
		}
	}
	// The default source in the urfave header is the SPEC path as given.
	data, _ := os.ReadFile(filepath.Join(dir, "k/gen.go"))
	if !strings.Contains(string(data), "from ../../testdata/kitchen.json.") {
		t.Errorf("header does not name the spec:\n%s", firstLine(string(data)))
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

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
