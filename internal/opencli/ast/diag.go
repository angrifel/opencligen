package ast

import (
	"fmt"
	"strings"
)

type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Diagnostic is a problem found in a document, located by JSON pointer and,
// when known, by line and column (both 1-based).
type Diagnostic struct {
	File     string
	Pointer  string
	Line     int
	Column   int
	Severity Severity
	Message  string
}

func (d Diagnostic) String() string {
	loc := d.File
	if d.Line > 0 {
		loc += fmt.Sprintf(":%d:%d", d.Line, d.Column)
	}
	return fmt.Sprintf("%s: %s: %s", loc, d.Severity, d.Message)
}

// Error is returned when a document has at least one error-level diagnostic.
type Error struct {
	Diagnostics []Diagnostic // errors only; warnings are returned separately
}

func (e *Error) Error() string {
	lines := make([]string, len(e.Diagnostics))
	for i, d := range e.Diagnostics {
		lines[i] = d.String()
	}
	return strings.Join(lines, "\n")
}

// Diagnostic returns a diagnostic about the value at the JSON pointer ptr in
// the document the model was parsed from, with its line and column. It lets
// generators report problems the same way the loader does.
func (m *Model) Diagnostic(sev Severity, ptr, format string, args ...any) Diagnostic {
	d := Diagnostic{Pointer: ptr, Severity: sev, Message: fmt.Sprintf(format, args...)}
	if m.Source != nil {
		d.File = m.Source.Name
		d.Line, d.Column = m.Source.Position(ptr)
	}
	return d
}
