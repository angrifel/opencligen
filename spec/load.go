package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/angrifel/opencligen/schema"
)

// Options control how strictly a document is checked.
type Options struct {
	// Lenient turns x- metadata that cannot be resolved (an unknown x-type,
	// an unparsable x-default or x-range) into warnings; the affected option
	// or argument is then treated as untyped.
	Lenient bool

	// AllowCLimits accepts C limit constants such as INT_MAX, -FLT_MAX or
	// I64_MIN in x-range and numeric x-default values.
	AllowCLimits bool
}

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

// LoadFile reads and loads the document at path.
func LoadFile(path string, opts Options) (*Model, []Diagnostic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return Load(path, data, opts)
}

// Load validates an OpenCLI document and resolves it into a Model. name is
// used in diagnostics. It returns the warnings found, and an *Error when the
// document has errors.
func Load(name string, data []byte, opts Options) (*Model, []Diagnostic, error) {
	l := &loader{name: name, data: data, opts: opts}
	m := l.load()
	l.locate()
	if m != nil {
		m.name, m.data = name, data
	}

	var errs, warns []Diagnostic
	for _, d := range l.diags {
		if d.Severity == SeverityWarning {
			warns = append(warns, d)
		} else {
			errs = append(errs, d)
		}
	}
	if len(errs) > 0 {
		return nil, warns, &Error{Diagnostics: errs}
	}
	return m, warns, nil
}

type loader struct {
	name  string
	data  []byte
	opts  Options
	diags []Diagnostic
}

func (l *loader) errorf(ptr, format string, args ...any) {
	l.diags = append(l.diags, Diagnostic{File: l.name, Pointer: ptr, Severity: SeverityError, Message: fmt.Sprintf(format, args...)})
}

func (l *loader) warnf(ptr, format string, args ...any) {
	l.diags = append(l.diags, Diagnostic{File: l.name, Pointer: ptr, Severity: SeverityWarning, Message: fmt.Sprintf(format, args...)})
}

func (l *loader) load() *Model {
	// Syntax first, so the error carries a position.
	var probe any
	if err := json.Unmarshal(l.data, &probe); err != nil {
		var se *json.SyntaxError
		if errors.As(err, &se) {
			// Offset counts the bytes read, including the offending one.
			line, col := lineCol(l.data, max(int(se.Offset)-1, 0))
			l.diags = append(l.diags, Diagnostic{File: l.name, Line: line, Column: col, Message: "invalid JSON: " + se.Error()})
		} else {
			l.errorf("", "invalid JSON: %v", err)
		}
		return nil
	}

	var head struct {
		OpenCLI string `json:"opencli"`
	}
	_ = json.Unmarshal(l.data, &head)
	version := head.OpenCLI
	if version == "" {
		// Let the schema of the newest version report the missing property.
		vs := schema.Versions()
		version = vs[len(vs)-1]
	}
	sch, err := compiled(version)
	if err != nil {
		l.errorf("/opencli", "%v", err)
		return nil
	}

	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(l.data))
	if err != nil {
		l.errorf("", "invalid JSON: %v", err)
		return nil
	}
	if err := sch.Validate(inst); err != nil {
		var ve *jsonschema.ValidationError
		if !errors.As(err, &ve) {
			l.errorf("", "%v", err)
			return nil
		}
		l.schemaErrors(ve)
		return nil
	}

	var doc rawDocument
	if err := json.Unmarshal(l.data, &doc); err != nil {
		l.errorf("", "decoding document: %v", err)
		return nil
	}
	return l.build(&doc)
}

var printer = message.NewPrinter(language.English)

// schemaErrors reports the leaf errors of a schema validation failure; the
// inner nodes only say that a subschema failed.
func (l *loader) schemaErrors(ve *jsonschema.ValidationError) {
	seen := map[string]bool{}
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		ptr := ""
		for _, tok := range e.InstanceLocation {
			ptr += "/" + escapePointer(tok)
		}
		msg := e.ErrorKind.LocalizedString(printer)
		if key := ptr + "\x00" + msg; !seen[key] {
			seen[key] = true
			l.errorf(ptr, "does not conform to the OpenCLI schema: %s", msg)
		}
	}
	walk(ve)
}

var (
	schemaMu    sync.Mutex
	schemaCache = map[string]*jsonschema.Schema{}
)

func compiled(version string) (*jsonschema.Schema, error) {
	raw, err := schema.For(version)
	if err != nil {
		return nil, err
	}
	mm := schema.MajorMinor(version)

	schemaMu.Lock()
	defer schemaMu.Unlock()
	if s, ok := schemaCache[mm]; ok {
		return s, nil
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("embedded schema %s: %w", mm, err)
	}
	url := "https://opencli.org/schema/" + mm + "/schema.json"
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("embedded schema %s: %w", mm, err)
	}
	s, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("embedded schema %s: %w", mm, err)
	}
	schemaCache[mm] = s
	return s, nil
}

// locate fills in line and column for diagnostics that have a pointer.
func (l *loader) locate() {
	var index map[string]int
	for i := range l.diags {
		d := &l.diags[i]
		if d.Line > 0 {
			continue
		}
		if index == nil {
			index = pointerOffsets(l.data)
		}
		d.Line, d.Column = position(l.data, index, d.Pointer)
	}
	sort.SliceStable(l.diags, func(i, j int) bool {
		a, b := l.diags[i], l.diags[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}

// position returns the line and column of the value at ptr, or of the
// nearest enclosing value that exists in the document; 0, 0 if none does.
func position(data []byte, index map[string]int, ptr string) (line, col int) {
	for {
		if off, ok := index[ptr]; ok {
			return lineCol(data, off)
		}
		if ptr == "" {
			return 0, 0
		}
		ptr = ptr[:strings.LastIndexByte(ptr, '/')]
	}
}

// Diagnostic returns a diagnostic about the value at the JSON pointer ptr in
// the document the model was loaded from, with its line and column. It lets
// generators report problems the same way the loader does.
func (m *Model) Diagnostic(sev Severity, ptr, format string, args ...any) Diagnostic {
	d := Diagnostic{File: m.name, Pointer: ptr, Severity: sev, Message: fmt.Sprintf(format, args...)}
	if m.data != nil {
		m.once.Do(func() { m.offsets = pointerOffsets(m.data) })
		d.Line, d.Column = position(m.data, m.offsets, ptr)
	}
	return d
}

// pointerOffsets maps the JSON pointer of every value in data to the byte
// offset where the value starts.
func pointerOffsets(data []byte) map[string]int {
	index := map[string]int{}
	dec := json.NewDecoder(bytes.NewReader(data))
	next := func() (json.Token, int, error) {
		off := int(dec.InputOffset())
		for off < len(data) && strings.IndexByte(" \t\r\n:,", data[off]) >= 0 {
			off++
		}
		tok, err := dec.Token()
		return tok, off, err
	}
	var walk func(ptr string) error
	walk = func(ptr string) error {
		tok, off, err := next()
		if err != nil {
			return err
		}
		index[ptr] = off
		switch tok {
		case json.Delim('{'):
			for dec.More() {
				key, _, err := next()
				if err != nil {
					return err
				}
				k, _ := key.(string)
				if err := walk(ptr + "/" + escapePointer(k)); err != nil {
					return err
				}
			}
			_, _, err = next()
		case json.Delim('['):
			for i := 0; dec.More(); i++ {
				if err := walk(ptr + "/" + strconv.Itoa(i)); err != nil {
					return err
				}
			}
			_, _, err = next()
		}
		return err
	}
	_ = walk("")
	return index
}

func escapePointer(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

func lineCol(data []byte, off int) (line, col int) {
	if off > len(data) {
		off = len(data)
	}
	before := data[:off]
	line = bytes.Count(before, []byte("\n")) + 1
	col = off - (bytes.LastIndexByte(before, '\n') + 1) + 1
	return line, col
}
