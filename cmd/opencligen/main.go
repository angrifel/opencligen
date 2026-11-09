// Command opencligen validates OpenCLI descriptions (https://opencli.org/)
// and generates code and documentation from them.
//
//	opencligen validate [flags] SPEC
//	opencligen urfave   [flags] -o FILE [-package NAME] SPEC
//	opencligen markdown [flags] -o FILE SPEC
//	opencligen man      [flags] -o FILE [-section N] SPEC
//
// Run "opencligen help" for details.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/angrifel/opencligen/gen/man"
	"github.com/angrifel/opencligen/gen/markdown"
	"github.com/angrifel/opencligen/gen/urfave"
	"github.com/angrifel/opencligen/schema"
	"github.com/angrifel/opencligen/spec"
)

const usage = `opencligen validates OpenCLI descriptions and generates code and docs from them.

Usage:
  opencligen <command> [flags] SPEC

Commands:
  validate   check SPEC against the OpenCLI schema and opencligen's rules
  urfave     generate a github.com/urfave/cli/v3 command tree (Go)
  markdown   generate a Markdown reference
  man        generate a roff man page
  version    print the opencligen version
  help       show this help

Flags for every command:
  -lenient          report x- metadata that cannot be typed, and features the
                    output cannot express, as warnings instead of errors
  -allow-c-limits   accept C limit constants (INT_MAX, -FLT_MAX, I64_MIN, ...)
                    in x-range and x-default

Run "opencligen <command> -h" for the flags of a command.
See docs/extensions.md for the x- metadata opencligen understands.
`

// Exit codes.
const (
	exitOK      = 0
	exitInvalid = 1 // the document has errors, or the output could not be generated
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	cmd, args := args[0], args[1:]
	switch cmd {
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, "opencligen", version())
		fmt.Fprintln(stdout, "OpenCLI versions:", schema.Versions())
		return exitOK
	case "validate", "urfave", "markdown", "man":
	default:
		fmt.Fprintf(stderr, "opencligen: unknown command %q\n\n%s", cmd, usage)
		return exitUsage
	}

	fs := flag.NewFlagSet("opencligen "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts spec.Options
	fs.BoolVar(&opts.Lenient, "lenient", false, "report problems that can be worked around as warnings")
	fs.BoolVar(&opts.AllowCLimits, "allow-c-limits", false, "accept C limit constants such as INT_MAX in x-range and x-default")
	out := "-"
	if cmd != "validate" {
		fs.StringVar(&out, "o", "-", "output file, or - for standard output")
	}
	pkg := fs.String("package", "cli", "Go package name (urfave)")
	source := fs.String("source", "", "document name in the generated header; defaults to the SPEC path (urfave)")
	section := fs.String("section", "1", "manual section (man)")
	date := fs.String("date", "", "date shown in the page footer (man)")
	hidden := fs.Bool("hidden", false, "document hidden commands, options and arguments (markdown, man)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: opencligen %s [flags] SPEC\n\nFlags:\n", cmd)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}
	path := fs.Arg(0)

	m, warns, err := spec.LoadFile(path, opts)
	report(stderr, warns)
	if err != nil {
		return fail(stderr, err)
	}

	var data []byte
	switch cmd {
	case "validate":
		fmt.Fprintf(stdout, "%s: valid OpenCLI %s description\n", path, m.OpenCLI)
		return exitOK
	case "urfave":
		src := *source
		if src == "" {
			src = filepath.ToSlash(path)
		}
		var gw []spec.Diagnostic
		data, gw, err = urfave.Generate(m, urfave.Options{Package: *pkg, Source: src, Lenient: opts.Lenient})
		report(stderr, gw)
		if err != nil {
			return fail(stderr, err)
		}
	case "markdown":
		data = markdown.Generate(m, markdown.Options{IncludeHidden: *hidden})
	case "man":
		data = man.Generate(m, man.Options{Section: *section, Date: *date, IncludeHidden: *hidden})
	}

	if err := write(out, data, stdout); err != nil {
		return fail(stderr, err)
	}
	return exitOK
}

func report(w io.Writer, diags []spec.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintln(w, d)
	}
}

func fail(w io.Writer, err error) int {
	fmt.Fprintln(w, err)
	var se *spec.Error
	if errors.As(err, &se) {
		fmt.Fprintf(w, "opencligen: %d error(s)\n", len(se.Diagnostics))
	}
	return exitInvalid
}

// write writes data to path, or to stdout for "-". An existing file with the
// same content is left untouched so build tools see it as up to date.
func write(path string, data []byte, stdout io.Writer) error {
	if path == "-" {
		_, err := stdout.Write(data)
		return err
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}
