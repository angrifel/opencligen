package markdown_test

import (
	"regexp"
	"strings"
	"unicode"
)

// A small reader for the Markdown subset the generator writes. It turns a
// document into an outline of sections and blocks, ignoring layout details
// such as blank lines, table padding and the width of separator rows, so
// tests compare content and structure rather than text.

type section struct {
	Level    int        `json:"level"`
	Heading  string     `json:"heading"`
	Blocks   []block    `json:"blocks,omitempty"`
	Sections []*section `json:"sections,omitempty"`
}

type block struct {
	Kind   string     `json:"kind"`             // paragraph, code, table or list
	Text   string     `json:"text,omitempty"`   // paragraph or code text
	Header []string   `json:"header,omitempty"` // table header
	Rows   [][]string `json:"rows,omitempty"`   // table rows
	Items  []item     `json:"items,omitempty"`  // list items
}

type item struct {
	Depth int    `json:"depth"`
	Text  string `json:"text"`
}

var headingRE = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*$`)
var listRE = regexp.MustCompile(`^(\s*)[-*]\s+(.*)$`)

// readMarkdown parses a document into a root section (level 0) that holds the
// whole tree.
func readMarkdown(doc string) *section {
	root := &section{}
	stack := []*section{root}
	cur := func() *section { return stack[len(stack)-1] }
	add := func(b block) { cur().Blocks = append(cur().Blocks, b) }

	var para []string
	flush := func() {
		if len(para) > 0 {
			add(block{Kind: "paragraph", Text: strings.Join(para, "")})
			para = nil
		}
	}

	lines := strings.Split(doc, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "```"):
			flush()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			add(block{Kind: "code", Text: strings.Join(code, "\n")})
		case headingRE.MatchString(line):
			flush()
			m := headingRE.FindStringSubmatch(line)
			s := &section{Level: len(m[1]), Heading: m[2]}
			for cur().Level >= s.Level {
				stack = stack[:len(stack)-1]
			}
			cur().Sections = append(cur().Sections, s)
			stack = append(stack, s)
		case strings.HasPrefix(trimmed, "|"):
			flush()
			t := block{Kind: "table", Header: cells(trimmed)}
			for i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "|") {
				i++
				row := cells(strings.TrimSpace(lines[i]))
				if isSeparator(row) {
					continue
				}
				t.Rows = append(t.Rows, row)
			}
			add(t)
		case listRE.MatchString(line):
			flush()
			l := block{Kind: "list"}
			for ; i < len(lines) && listRE.MatchString(lines[i]); i++ {
				m := listRE.FindStringSubmatch(lines[i])
				l.Items = append(l.Items, item{Depth: len(m[1]) / 2, Text: m[2]})
			}
			i--
			add(l)
		default:
			// A trailing double space is a hard line break; other line
			// breaks inside a paragraph are spaces.
			text := strings.TrimSpace(line)
			if strings.HasSuffix(line, "  ") {
				text += "\n"
			} else {
				text += " "
			}
			para = append(para, text)
		}
	}
	flush()
	trimParagraphs(root)
	return root
}

func trimParagraphs(s *section) {
	for i := range s.Blocks {
		if s.Blocks[i].Kind == "paragraph" {
			s.Blocks[i].Text = strings.TrimSpace(s.Blocks[i].Text)
		}
	}
	for _, sub := range s.Sections {
		trimParagraphs(sub)
	}
}

// cells splits a table row on unescaped "|".
func cells(row string) []string {
	row = strings.TrimSuffix(strings.TrimPrefix(row, "|"), "|")
	var out []string
	var b strings.Builder
	for i := 0; i < len(row); i++ {
		switch {
		case row[i] == '\\' && i+1 < len(row) && row[i+1] == '|':
			b.WriteByte('|')
			i++
		case row[i] == '|':
			out = append(out, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteByte(row[i])
		}
	}
	return append(out, strings.TrimSpace(b.String()))
}

func isSeparator(row []string) bool {
	for _, c := range row {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

// find returns the sections with the given heading at any depth.
func (s *section) find(heading string) []*section {
	var out []*section
	for _, sub := range s.Sections {
		if sub.Heading == heading {
			out = append(out, sub)
		}
		out = append(out, sub.find(heading)...)
	}
	return out
}

// child returns the direct subsection with the given heading, or nil.
func (s *section) child(heading string) *section {
	for _, sub := range s.Sections {
		if sub.Heading == heading {
			return sub
		}
	}
	return nil
}

func (s *section) firstBlock(kind string) *block {
	for i := range s.Blocks {
		if s.Blocks[i].Kind == kind {
			return &s.Blocks[i]
		}
	}
	return nil
}

// walk calls fn for s and every section below it.
func (s *section) walk(fn func(*section)) {
	fn(s)
	for _, sub := range s.Sections {
		sub.walk(fn)
	}
}

var codeSpanRE = regexp.MustCompile("`([^`]*)`")
var linkRE = regexp.MustCompile(`\]\((#[^)]*)\)`)

// codeSpans returns the contents of the inline code spans in text.
func codeSpans(text string) []string {
	var out []string
	for _, m := range codeSpanRE.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// anchorOf computes a GitHub-style heading anchor, independently of the
// generator: lower case, punctuation other than "-" and "_" removed, spaces
// replaced with "-".
func anchorOf(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
