// Package parser reads OpenCLI descriptions (https://opencli.org/): it checks
// them against the OpenCLI JSON schema and opencligen's semantic rules,
// resolves x- metadata into types, defaults and ranges, and builds an
// ast.Model.
package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/angrifel/opencligen/internal/opencli/ast"
	"github.com/angrifel/opencligen/internal/opencli/schema"
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

// LoadFile reads and loads the document at path.
func LoadFile(path string, opts Options) (*ast.Model, []ast.Diagnostic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return Load(path, data, opts)
}

// Load validates an OpenCLI document and resolves it into a Model. name is
// used in diagnostics. It returns the warnings found, and an *Error when the
// document has errors.
func Load(name string, data []byte, opts Options) (*ast.Model, []ast.Diagnostic, error) {
	l := &loader{name: name, data: data, opts: opts, src: &ast.Source{Name: name, Data: data}}
	m := l.load()
	l.locate()
	if m != nil {
		m.Source = l.src
	}

	var errs, warns []ast.Diagnostic
	for _, d := range l.diags {
		if d.Severity == ast.SeverityWarning {
			warns = append(warns, d)
		} else {
			errs = append(errs, d)
		}
	}
	if len(errs) > 0 {
		return nil, warns, &ast.Error{Diagnostics: errs}
	}
	return m, warns, nil
}

type loader struct {
	name  string
	data  []byte
	opts  Options
	src   *ast.Source
	diags []ast.Diagnostic
}

func (l *loader) errorf(ptr, format string, args ...any) {
	l.diags = append(l.diags, ast.Diagnostic{File: l.name, Pointer: ptr, Severity: ast.SeverityError, Message: fmt.Sprintf(format, args...)})
}

func (l *loader) warnf(ptr, format string, args ...any) {
	l.diags = append(l.diags, ast.Diagnostic{File: l.name, Pointer: ptr, Severity: ast.SeverityWarning, Message: fmt.Sprintf(format, args...)})
}

func (l *loader) load() *ast.Model {
	// Syntax first, so the error carries a position.
	var probe any
	if err := json.Unmarshal(l.data, &probe); err != nil {
		var se *json.SyntaxError
		if errors.As(err, &se) {
			// Offset counts the bytes read, including the offending one.
			line, col := ast.LineCol(l.data, max(int(se.Offset)-1, 0))
			l.diags = append(l.diags, ast.Diagnostic{File: l.name, Line: line, Column: col, Message: "invalid JSON: " + se.Error()})
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
			ptr += "/" + ast.EscapePointer(tok)
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
	for i := range l.diags {
		if d := &l.diags[i]; d.Line == 0 {
			d.Line, d.Column = l.src.Position(d.Pointer)
		}
	}
	sort.SliceStable(l.diags, func(i, j int) bool {
		a, b := l.diags[i], l.diags[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}
