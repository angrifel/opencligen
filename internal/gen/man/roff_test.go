package man_test

import (
	"fmt"
	"strings"
)

// A reader for the roff subset the man generator writes. It turns a page
// into sections, subsections and blocks of plain text, so tests compare
// content and structure rather than the exact requests used. Requests and
// escapes outside the subset are errors: when the generator starts using a
// new one, the reader must be taught it deliberately.

type page struct {
	Header   []string    `json:"header"` // .TH arguments
	Sections []*msection `json:"sections"`
}

type msection struct {
	Name        string      `json:"name"`
	Blocks      []mblock    `json:"blocks,omitempty"`
	Subsections []*msection `json:"subsections,omitempty"`
}

type mblock struct {
	Kind     string `json:"kind"`           // paragraph, literal or entry
	Text     string `json:"text,omitempty"` // paragraph or literal text, or entry tag
	Body     string `json:"body,omitempty"` // entry body
	Indented bool   `json:"indented,omitempty"`
}

func readRoff(src string) (*page, error) {
	p := &page{}
	var sec, sub *msection
	target := func() *msection {
		if sub != nil {
			return sub
		}
		return sec
	}
	var text []string // current paragraph or entry body
	var entry *mblock // open .TP entry, waiting for its body
	wantTag := false
	literal := false
	var lit []string
	indented := false

	flush := func() {
		joined := strings.TrimSpace(strings.Join(text, " "))
		joined = strings.ReplaceAll(joined, " \n\n ", "\n\n")
		joined = strings.ReplaceAll(joined, " \n ", "\n")
		text = nil
		t := target()
		switch {
		case entry != nil:
			entry.Body = joined
			t.Blocks = append(t.Blocks, *entry)
			entry = nil
		case joined != "" && t != nil:
			t.Blocks = append(t.Blocks, mblock{Kind: "paragraph", Text: joined, Indented: indented})
		}
	}

	for n, line := range strings.Split(strings.TrimRight(src, "\n"), "\n") {
		fail := func(format string, args ...any) (*page, error) {
			return nil, fmt.Errorf("line %d: %s: %q", n+1, fmt.Sprintf(format, args...), line)
		}
		if literal {
			if line == ".fi" {
				literal = false
				target().Blocks = append(target().Blocks, mblock{Kind: "literal", Text: strings.Join(lit, "\n"), Indented: indented})
				lit = nil
				continue
			}
			s, err := unescape(line)
			if err != nil {
				return fail("%v", err)
			}
			lit = append(lit, s)
			continue
		}
		if wantTag {
			s, err := unescape(line)
			if err != nil {
				return fail("%v", err)
			}
			entry = &mblock{Kind: "entry", Text: s, Indented: indented}
			wantTag = false
			continue
		}
		if !strings.HasPrefix(line, ".") {
			s, err := unescape(line)
			if err != nil {
				return fail("%v", err)
			}
			text = append(text, s)
			continue
		}
		req, args, _ := strings.Cut(line, " ")
		switch req {
		case ".TH":
			p.Header = quotedArgs(args)
		case ".SH", ".SS":
			flush()
			name, err := unescape(strings.Join(quotedArgs(args), " "))
			if err != nil {
				return fail("%v", err)
			}
			s := &msection{Name: name}
			if req == ".SH" {
				p.Sections = append(p.Sections, s)
				sec, sub = s, nil
			} else {
				if sec == nil {
					return fail(".SS outside a section")
				}
				sec.Subsections = append(sec.Subsections, s)
				sub = s
			}
		case ".PP":
			flush()
		case ".IP":
			// Inside an entry, a new paragraph of the entry's body;
			// otherwise an ordinary paragraph break.
			if entry != nil {
				text = append(text, "\n\n")
			} else {
				flush()
			}
		case ".TP":
			flush()
			wantTag = true
		case ".nf":
			flush()
			literal = true
		case ".RS":
			flush()
			indented = true
		case ".RE":
			flush()
			indented = false
		case ".br":
			text = append(text, "\n")
		case ".B":
			s, err := unescape(args)
			if err != nil {
				return fail("%v", err)
			}
			text = append(text, s)
		default:
			return fail("unsupported request %s", req)
		}
	}
	flush()
	return p, nil
}

// unescape removes font changes and resolves the escapes the generator
// writes.
func unescape(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) {
			return "", fmt.Errorf("trailing backslash")
		}
		i++
		switch s[i] {
		case 'f':
			if i+1 >= len(s) || !strings.ContainsRune("BIRP", rune(s[i+1])) {
				return "", fmt.Errorf("unsupported font escape")
			}
			i++
		case '-':
			b.WriteByte('-')
		case 'e':
			b.WriteByte('\\')
		case '&':
		default:
			return "", fmt.Errorf("unsupported escape \\%c", s[i])
		}
	}
	return b.String(), nil
}

// quotedArgs splits request arguments, honouring double quotes.
func quotedArgs(s string) []string {
	var out []string
	for s = strings.TrimSpace(s); s != ""; s = strings.TrimSpace(s) {
		if s[0] == '"' {
			end := strings.IndexByte(s[1:], '"')
			if end < 0 {
				return append(out, s[1:])
			}
			out = append(out, s[1:end+1])
			s = s[end+2:]
			continue
		}
		word, rest, _ := strings.Cut(s, " ")
		out = append(out, word)
		s = rest
	}
	return out
}

func (p *page) section(name string) *msection {
	for _, s := range p.Sections {
		if s.Name == name {
			return s
		}
	}
	return nil
}
