package tape

import (
	"encoding/json"
	"fmt"
	"strings"
)

type token struct {
	text         string
	kind         byte
	line, column int
}
type stream struct {
	tokens []token
	pos    int
}

func (s *stream) end() bool   { return s.pos >= len(s.tokens) }
func (s *stream) peek() token { return s.tokens[s.pos] }
func (s *stream) pop() token  { t := s.peek(); s.pos++; return t }

// lex keeps quoted text literal, as VHS does: shell escapes belong to the
// shell. Regex and JSON literals have their own delimiters and may contain #.
func lex(input string) ([]token, error) {
	var out []token
	line, col := 1, 1
	advance := func(s string) {
		for _, c := range s {
			if c == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
	}
	regexNext := false
	for i := 0; i < len(input); {
		c := input[i]
		if strings.ContainsRune(" \t\r\n", rune(c)) {
			advance(input[i : i+1])
			i++
			continue
		}
		if c == '#' {
			j := i
			for j < len(input) && input[j] != '\n' {
				j++
			}
			advance(input[i:j])
			i = j
			continue
		}
		t := token{kind: 'w', line: line, column: col}
		start := i
		switch {
		case c == '\'' || c == '"' || c == '`':
			quote := c
			i++
			j := i
			for i < len(input) && input[i] != quote {
				i++
			}
			if i == len(input) {
				return nil, fmt.Errorf("%d:%d: unterminated quoted string", line, col)
			}
			t.kind = 's'
			t.text = input[j:i]
			i++
			if i < len(input) && !strings.ContainsRune(" \t\r\n#", rune(input[i])) {
				return nil, fmt.Errorf("%d:%d: expected whitespace after string", line, col)
			}
		case c == '{':
			depth := 0
			quoted, escaped := false, false
			for i < len(input) {
				b := input[i]
				i++
				if quoted {
					if escaped {
						escaped = false
					} else if b == '\\' {
						escaped = true
					} else if b == '"' {
						quoted = false
					}
					continue
				}
				if b == '"' {
					quoted = true
				} else if b == '{' {
					depth++
				} else if b == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			t.kind = 'j'
			t.text = input[start:i]
			if !json.Valid([]byte(t.text)) {
				return nil, fmt.Errorf("%d:%d: invalid JSON object", line, col)
			}
		case c == '/' && regexNext:
			i++
			var b strings.Builder
			closed := false
			for i < len(input) {
				if input[i] == '/' {
					i++
					closed = true
					break
				}
				if input[i] == '\n' {
					break
				}
				if input[i] == '\\' && i+1 < len(input) {
					if input[i+1] == '/' {
						b.WriteByte('/')
					} else {
						b.WriteString(input[i : i+2])
					}
					i += 2
				} else {
					b.WriteByte(input[i])
					i++
				}
			}
			if !closed {
				return nil, fmt.Errorf("%d:%d: unterminated regular expression", line, col)
			}
			t.kind = 'r'
			t.text = b.String()
			regexNext = false
		default:
			for i < len(input) && !strings.ContainsRune(" \t\r\n#", rune(input[i])) {
				i++
			}
			t.text = input[start:i]
		}
		if t.kind == 'w' {
			if t.text == "WaitPattern" || t.text == "Wait" || strings.HasPrefix(t.text, "Wait+") || strings.HasPrefix(t.text, "Wait@") {
				regexNext = true
			} else if isCommand(t.text) {
				regexNext = false
			}
		}
		out = append(out, t)
		advance(input[start:i])
	}
	// Whitespace around command modifiers is insignificant.
	for i := 0; i < len(out); i++ {
		if out[i].kind != 'w' || !isCommand(out[i].text) && out[i].text != "Ctrl" && out[i].text != "Alt" && out[i].text != "Shift" {
			continue
		}
		for i+1 < len(out) && out[i+1].kind == 'w' {
			next := out[i+1].text
			if strings.HasSuffix(out[i].text, "@") && next != "" {
				out[i].text += next
				out = append(out[:i+1], out[i+2:]...)
				continue
			}
			if (next == "+" || next == "@") && i+2 < len(out) {
				out[i].text += next + out[i+2].text
				out = append(out[:i+1], out[i+3:]...)
			} else if strings.HasPrefix(next, "@") || strings.HasPrefix(next, "+") {
				out[i].text += next
				out = append(out[:i+1], out[i+2:]...)
			} else {
				break
			}
		}
	}
	return out, nil
}
