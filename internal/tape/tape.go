// Package tape implements sigmamax's deliberately small VHS-style language.
package tape

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/encode"
)

type Config struct {
	Output, Shell             string
	FontSize                  float64
	Cols, Rows, Width, Height int
}

func DefaultConfig() Config {
	return Config{Output: "recording.gif", Shell: "/bin/sh", FontSize: 18, Cols: 80, Rows: 24}
}

type Action struct {
	Command, Text string
	Duration      time.Duration
	Line          int
}

type Tape struct {
	Config  Config
	Actions []Action
}

func Parse(r io.Reader) (*Tape, error) {
	t := &Tape{Config: DefaultConfig()}
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), 1024*1024)
	started := false
	line := 0
	for s.Scan() {
		line++
		words, err := split(s.Text())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if len(words) == 0 {
			continue
		}
		fail := func(message string) (*Tape, error) { return nil, fmt.Errorf("line %d: %s", line, message) }
		a := Action{Command: words[0], Line: line}
		switch words[0] {
		case "Output":
			if started || len(words) != 2 {
				return fail("Output requires a .gif, .mp4, or .webm path before actions")
			}
			if _, err := encode.Format(words[1]); err != nil {
				return fail(err.Error())
			}
			t.Config.Output = words[1]
			continue
		case "Set":
			if started || len(words) != 3 {
				return fail("Set requires a setting and value before actions")
			}
			if words[1] == "Shell" {
				if words[2] == "" {
					return fail("Shell cannot be empty")
				}
				t.Config.Shell = words[2]
				continue
			}
			if words[1] == "FontSize" {
				v, e := strconv.ParseFloat(words[2], 64)
				if e != nil || !(v >= 6 && v <= 96) {
					return fail("FontSize must be between 6 and 96")
				}
				t.Config.FontSize = v
				continue
			}
			v, e := strconv.Atoi(words[2])
			if e != nil || v < 1 || v > 8192 {
				return fail("size must be an integer between 1 and 8192")
			}
			switch words[1] {
			case "Width":
				t.Config.Width = v
			case "Height":
				t.Config.Height = v
			case "Cols", "Columns":
				t.Config.Cols = v
			case "Rows":
				t.Config.Rows = v
			default:
				return fail("unknown setting " + words[1])
			}
			continue
		case "Type":
			if len(words) != 2 {
				return fail("Type requires one quoted string")
			}
			a.Text = words[1]
		case "Sleep":
			if len(words) != 2 {
				return fail("Sleep requires a duration such as 500ms or 2s")
			}
			a.Duration, err = time.ParseDuration(words[1])
			if err != nil || a.Duration < 0 {
				return fail("invalid Sleep duration")
			}
		case "Enter", "Hide", "Show":
			if len(words) != 1 {
				return fail(words[0] + " takes no arguments")
			}
		default:
			return fail("unknown command " + words[0])
		}
		started = true
		t.Actions = append(t.Actions, a)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return t, nil
}

// Double quotes use Go escapes; single quotes and backticks are literal.
// Comments start at # outside a quoted token.
func split(s string) ([]string, error) {
	var out []string
	for {
		s = strings.TrimSpace(s)
		if s == "" || s[0] == '#' {
			return out, nil
		}
		if s[0] == '"' || s[0] == '\'' || s[0] == '`' {
			quote := s[0]
			i := 1
			for i < len(s) {
				if quote == '"' && s[i] == '\\' {
					i += 2
					continue
				}
				if s[i] == quote {
					break
				}
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("unterminated quoted string")
			}
			v := s[1:i]
			if quote == '"' {
				var err error
				v, err = strconv.Unquote(s[:i+1])
				if err != nil {
					return nil, err
				}
			}
			out = append(out, v)
			s = s[i+1:]
			if s != "" && !strings.ContainsRune(" \t#", rune(s[0])) {
				return nil, fmt.Errorf("expected whitespace after string")
			}
		} else {
			i := strings.IndexAny(s, " \t#")
			if i < 0 {
				i = len(s)
			}
			out = append(out, s[:i])
			s = s[i:]
		}
	}
}
