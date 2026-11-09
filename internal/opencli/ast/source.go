package ast

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
)

// Source is a JSON document, used to turn JSON pointers into line and column
// positions, the way go/token positions nodes of go/ast.
type Source struct {
	Name string // file name used in diagnostics
	Data []byte

	once    sync.Once
	offsets map[string]int
}

// Position returns the 1-based line and column of the value at the JSON
// pointer ptr, or of the nearest enclosing value that exists in the
// document; 0, 0 if none does.
func (s *Source) Position(ptr string) (line, col int) {
	s.once.Do(func() { s.offsets = pointerOffsets(s.Data) })
	for {
		if off, ok := s.offsets[ptr]; ok {
			return LineCol(s.Data, off)
		}
		if ptr == "" {
			return 0, 0
		}
		ptr = ptr[:strings.LastIndexByte(ptr, '/')]
	}
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
				if err := walk(ptr + "/" + EscapePointer(k)); err != nil {
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

// EscapePointer escapes a JSON object key for use in a JSON pointer.
func EscapePointer(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

// LineCol returns the 1-based line and column of byte offset off in data.
func LineCol(data []byte, off int) (line, col int) {
	if off > len(data) {
		off = len(data)
	}
	before := data[:off]
	line = bytes.Count(before, []byte("\n")) + 1
	col = off - (bytes.LastIndexByte(before, '\n') + 1) + 1
	return line, col
}
