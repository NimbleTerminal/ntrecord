// Package tape parses and validates the VHS recording language.
package tape

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"nimbleterminal.dev/ntrecord/internal/encode"
)

// Mode selects the clock a recording runs on.
type Mode string

const (
	// Realtime samples the screen on the wall clock. It captures whatever a
	// program does, animations included, but two runs differ.
	Realtime Mode = "realtime"
	// Deterministic runs a virtual clock: each step of a tape is held for the
	// time the tape says, after the program has settled, so the same tape gives
	// the same recording and sleeping costs no real time.
	Deterministic Mode = "deterministic"
)

// ParseMode accepts "realtime", "deterministic", or "" (realtime).
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case "", Realtime:
		return Realtime, nil
	case Deterministic:
		return Deterministic, nil
	}
	return "", fmt.Errorf("unknown mode %q: use realtime or deterministic", s)
}

type Config struct {
	Output, Shell                                string
	Outputs                                      []string
	Env                                          map[string]string
	Require                                      []string
	FontSize                                     float64
	FontFamily                                   string
	Cols, Rows, Width, Height                    int
	LetterSpacing, LineHeight                    float64
	Padding, Margin, BorderRadius, WindowBarSize int
	WindowBar, MarginFill, Theme                 string
	Framerate, PlaybackSpeed                     float64
	LoopOffset                                   string
	CursorBlink                                  bool
	TypingSpeed, WaitTimeout                     time.Duration
	WaitPattern                                  string
	// Mode is the recording clock; the command line sets it, not tapes.
	Mode Mode
	// Version is the ntrecord release advertised to recorded programs; it is
	// set by the command line, not by tapes.
	Version string
}

func DefaultConfig() Config {
	return Config{Output: "recording.gif", Shell: "/bin/sh", FontSize: 18, Cols: 80, Rows: 24,
		LineHeight: 1, FontFamily: "Go Mono", Framerate: 20, PlaybackSpeed: 1,
		TypingSpeed: 25 * time.Millisecond, WaitTimeout: 15 * time.Second, WaitPattern: `[$>#]$`, WindowBarSize: 30, Mode: Realtime}
}

// ProgramVersion is Version without a leading "v", or "dev" when unset.
func (c Config) ProgramVersion() string {
	if v := strings.TrimPrefix(c.Version, "v"); v != "" {
		return v
	}
	return "dev"
}

func (c Config) Destinations() []string {
	if len(c.Outputs) > 0 {
		return c.Outputs
	}
	return []string{c.Output}
}

type Action struct {
	Command, Text string
	Duration      time.Duration
	Interval      time.Duration
	Count         int
	Scope         string
	Status        StatusCondition
	Line, Column  int
	Source        string
}

func (a Action) Error(err error) error {
	return fmt.Errorf("%s:%d:%d: %w", a.Source, a.Line, a.Column, err)
}

type Tape struct {
	Config  Config
	Actions []Action
}

type parser struct {
	tape       *Tape
	started    bool
	speed      time.Duration
	stack      map[string]bool
	bytes      int
	dimensions map[string]bool
}

func Parse(r io.Reader) (*Tape, error) { return parse(r, "<tape>") }
func ParseFile(path string) (*Tape, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse(f, path)
}
func parse(r io.Reader, name string) (*Tape, error) {
	c := DefaultConfig()
	p := &parser{tape: &Tape{Config: c}, speed: c.TypingSpeed, stack: make(map[string]bool), dimensions: make(map[string]bool)}
	if err := p.read(r, name, false); err != nil {
		return nil, err
	}
	return p.tape, nil
}

func (p *parser) read(r io.Reader, name string, sourced bool) error {
	abs, _ := filepath.Abs(name)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	if p.stack[abs] || len(p.stack) >= 32 {
		return fmt.Errorf("%s: Source cycle or nesting exceeds 32 files", name)
	}
	p.stack[abs] = true
	defer delete(p.stack, abs)
	b, err := io.ReadAll(io.LimitReader(r, 8*1024*1024+1))
	if err != nil {
		return err
	}
	p.bytes += len(b)
	if p.bytes > 8*1024*1024 {
		return fmt.Errorf("tape and sources exceed 8 MiB")
	}
	ts, err := lex(string(b))
	if err != nil {
		return fmt.Errorf("%s:%w", name, err)
	}
	s := &stream{tokens: ts}
	for !s.end() {
		t := s.pop()
		a := Action{Command: t.text, Line: t.line, Column: t.column, Source: name, Count: 1, Interval: p.speed}
		fail := func(err error) error { return a.Error(err) }
		if t.kind != 'w' {
			return fail(fmt.Errorf("expected command, got %q", t.text))
		}
		head, timing, hasTiming := strings.Cut(t.text, "@")
		// Ctrl+@ names a key, rather than a speed override.
		if strings.HasSuffix(head, "+") && timing == "" {
			head = t.text
			hasTiming = false
		}
		a.Command = head
		if hasTiming {
			if !s.end() && isUnit(s.peek().text) {
				timing += s.pop().text
			}
			a.Interval, err = duration(timing)
			if err != nil {
				return fail(err)
			}
		}
		if hasTiming && head != "Type" && !strings.HasPrefix(head, "Wait") && !isKey(head) {
			return fail(fmt.Errorf("%s does not accept @time", head))
		}
		value := func() (string, error) {
			if s.end() || s.peek().line > t.line && isCommand(s.peek().text) {
				return "", fmt.Errorf("%s requires a value", head)
			}
			return s.pop().text, nil
		}
		switch head {
		case "Output":
			if p.started && !sourced {
				return fail(fmt.Errorf("Output must precede actions"))
			}
			v, e := value()
			if e != nil {
				return fail(e)
			}
			if _, e = encode.Format(v); e != nil {
				return fail(e)
			}
			if !sourced {
				for _, out := range p.tape.Config.Outputs {
					if filepath.Clean(out) == filepath.Clean(v) {
						return fail(fmt.Errorf("duplicate output %q", v))
					}
				}
				p.tape.Config.Outputs = append(p.tape.Config.Outputs, v)
				p.tape.Config.Output = v
			}
			continue
		case "Source":
			v, e := value()
			if e != nil {
				return fail(e)
			}
			// VHS paths resolve from the invocation directory, including nested sources.
			f, e := os.Open(v)
			if e != nil {
				return fail(e)
			}
			e = p.read(f, v, true)
			_ = f.Close()
			if e != nil {
				return fail(e)
			}
			continue
		case "Require":
			if p.started {
				return fail(fmt.Errorf("Require must precede actions"))
			}
			v, e := value()
			if e != nil {
				return fail(e)
			}
			if v == "" {
				return fail(fmt.Errorf("Require needs a program"))
			}
			p.tape.Config.Require = append(p.tape.Config.Require, v)
			continue
		case "Env":
			if p.started {
				return fail(fmt.Errorf("Env must precede actions"))
			}
			key, e := value()
			if e != nil {
				return fail(e)
			}
			v, e := value()
			if e != nil {
				return fail(e)
			}
			if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(key) || strings.ContainsRune(v, 0) {
				return fail(fmt.Errorf("invalid environment variable"))
			}
			if p.tape.Config.Env == nil {
				p.tape.Config.Env = make(map[string]string)
			}
			p.tape.Config.Env[key] = v
			continue
		case "Set":
			setting, e := value()
			if e != nil {
				return fail(e)
			}
			v, e := value()
			if e != nil {
				return fail(e)
			}
			if !s.end() && isUnit(s.peek().text) {
				v += s.pop().text
			}
			if p.started && setting != "TypingSpeed" {
				return fail(fmt.Errorf("Set %s must precede actions", setting))
			}
			if e = p.setting(setting, v); e != nil {
				return fail(e)
			}
			continue
		case "Type", "Copy":
			v, e := value()
			if e != nil {
				return fail(e)
			}
			a.Text = v
			for !s.end() && (s.peek().kind == 's' || !isCommand(s.peek().text) && s.peek().line == t.line) {
				a.Text += " " + s.pop().text
			}
		case "Sleep":
			v, e := value()
			if e != nil {
				return fail(e)
			}
			if !s.end() && isUnit(s.peek().text) {
				v += s.pop().text
			}
			a.Duration, e = duration(v)
			if e != nil {
				return fail(e)
			}
		case "Wait+Status":
			a.Command, a.Scope = "Wait", "Status"
			a.Duration = p.tape.Config.WaitTimeout
			if hasTiming {
				a.Duration = a.Interval
			}
			if a.Duration <= 0 {
				return fail(fmt.Errorf("Wait timeout must be positive"))
			}
			state, e := value()
			if e != nil {
				return fail(e)
			}
			var filters []string
			for !s.end() && s.peek().line == t.line && !isCommand(s.peek().text) {
				filters = append(filters, s.pop().text)
			}
			a.Status, e = parseStatusCondition(state, filters)
			if e != nil {
				return fail(e)
			}
		case "Wait", "Wait+Line", "Wait+Screen":
			a.Command = "Wait"
			a.Scope = "Line"
			if head == "Wait+Screen" {
				a.Scope = "Screen"
			}
			a.Text = p.tape.Config.WaitPattern
			a.Duration = p.tape.Config.WaitTimeout
			if hasTiming {
				a.Duration = a.Interval
			}
			if a.Duration <= 0 {
				return fail(fmt.Errorf("Wait timeout must be positive"))
			}
			if !s.end() && s.peek().kind == 'r' {
				a.Text = s.pop().text
			}
			if _, e := regexp.Compile(a.Text); e != nil {
				return fail(e)
			}
		case "Screenshot":
			v, e := value()
			if e != nil {
				return fail(e)
			}
			a.Text = v
			if strings.ToLower(filepath.Ext(v)) != ".png" {
				return fail(fmt.Errorf("Screenshot requires a .png path"))
			}
		case "Hide", "Show", "Paste":
		default:
			if !isKey(head) {
				return fail(fmt.Errorf("unknown command %q", head))
			}
			if !s.end() {
				if n, e := strconv.Atoi(s.peek().text); e == nil {
					s.pop()
					if n < 1 || n > 1000000 {
						return fail(fmt.Errorf("repeat count must be between 1 and 1000000"))
					}
					a.Count = n
				}
			}
		}

		p.started = true
		p.tape.Actions = append(p.tape.Actions, a)
	}
	return nil
}

func duration(s string) (time.Duration, error) {
	s = strings.ReplaceAll(s, " ", "")
	if v, e := strconv.ParseFloat(s, 64); e == nil {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 86400*365 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(v * float64(time.Second)), nil
	}
	d, e := time.ParseDuration(s)
	if e != nil || d < 0 {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return d, nil
}
func isUnit(s string) bool {
	return s == "ms" || s == "s" || s == "m" || s == "h" || s == "px" || s == "%"
}
func isCommand(s string) bool {
	head, _, _ := strings.Cut(s, "@")
	switch head {
	case "Output", "Require", "Env", "Source", "Set", "Type", "Sleep", "Hide", "Show", "Wait", "Wait+Line", "Wait+Screen", "Wait+Status", "Screenshot", "Copy", "Paste":
		return true
	}
	return isKey(head)
}
func isKey(s string) bool {
	parts := strings.Split(s, "+")
	if len(parts) > 1 {
		n := 0
		for n < len(parts) && (parts[n] == "Ctrl" || parts[n] == "Alt" || parts[n] == "Shift") {
			n++
		}
		if n == 0 || n == len(parts) {
			return false
		}
		for _, key := range parts[n:] {
			if key == "ScrollUp" || key == "ScrollDown" {
				return false
			}
			if utf8.RuneCountInString(key) != 1 && !isKey(key) {
				return false
			}
		}
		return true
	}
	switch s {
	case "Enter", "Tab", "Space", "Backspace", "Delete", "Insert", "Escape", "Home", "End", "Up", "Down", "Left", "Right", "PageUp", "PageDown", "ScrollUp", "ScrollDown":
		return true
	}
	if strings.HasPrefix(s, "F") {
		n, e := strconv.Atoi(s[1:])
		return e == nil && n >= 1 && n <= 25
	}
	return false
}
