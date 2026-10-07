// Package asciicast streams standard asciicast v2/v3 terminal recordings.
package asciicast

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxLine bounds memory used by an individual event, including inline graphics.
const MaxLine = 64 * 1024 * 1024

type Theme struct {
	FG      string `json:"fg"`
	BG      string `json:"bg"`
	Palette string `json:"palette"`
}

type Terminal struct {
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
	Theme *Theme `json:"theme,omitempty"`
}

type Header struct {
	Version  int       `json:"version"`
	Term     *Terminal `json:"term,omitempty"`
	Width    int       `json:"width,omitempty"`
	Height   int       `json:"height,omitempty"`
	Theme    *Theme    `json:"theme,omitempty"`
	Duration float64   `json:"duration,omitempty"`
}

func (h Header) Size() (int, int) {
	if h.Version == 3 && h.Term != nil {
		return h.Term.Cols, h.Term.Rows
	}
	return h.Width, h.Height
}

func (h Header) Colors() *Theme {
	if h.Version == 3 && h.Term != nil {
		return h.Term.Theme
	}
	return h.Theme
}

// ThemeJSON translates the standard palette into the renderer's theme keys.
func (t *Theme) ThemeJSON() (string, error) {
	if t == nil {
		return "", nil
	}
	colors := strings.Split(t.Palette, ":")
	if len(colors) != 8 && len(colors) != 16 {
		return "", fmt.Errorf("asciicast theme palette must contain 8 or 16 colors")
	}
	names := []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite"}
	m := map[string]string{"foreground": t.FG, "background": t.BG}
	for i, c := range colors {
		m[names[i]] = c
	}
	b, err := json.Marshal(m)
	return string(b), err
}

type Event struct {
	At   time.Duration // absolute time, for both format versions
	Code string
	Data string
}

type Reader struct {
	Header  Header
	scanner *bufio.Scanner
	line    int
	at      time.Duration
}

func NewReader(src io.Reader) (*Reader, error) {
	r := &Reader{scanner: bufio.NewScanner(src), line: 1}
	r.scanner.Buffer(make([]byte, 4096), MaxLine)
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("asciicast: missing header")
	}
	if err := json.Unmarshal(r.scanner.Bytes(), &r.Header); err != nil {
		return nil, fmt.Errorf("asciicast header: %w", err)
	}
	if r.Header.Version != 2 && r.Header.Version != 3 {
		return nil, fmt.Errorf("unsupported asciicast version %d (use v2 or v3)", r.Header.Version)
	}
	if r.Header.Version == 3 && r.Header.Term == nil {
		return nil, fmt.Errorf("asciicast v3 requires term")
	}
	cols, rows := r.Header.Size()
	if cols < 1 || rows < 1 || cols > 8192 || rows > 8192 {
		return nil, fmt.Errorf("invalid asciicast grid %dx%d", cols, rows)
	}
	if _, err := seconds(r.Header.Duration); err != nil {
		return nil, fmt.Errorf("asciicast duration: %w", err)
	}
	return r, nil
}

func seconds(v float64) (time.Duration, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v >= float64(math.MaxInt64)/float64(time.Second) {
		return 0, fmt.Errorf("invalid time %v", v)
	}
	return time.Duration(v * float64(time.Second)), nil
}

func (r *Reader) Next() (Event, error) {
	for r.scanner.Scan() {
		r.line++
		b := r.scanner.Bytes()
		if r.Header.Version == 3 && len(b) > 0 && b[0] == '#' {
			continue
		}
		e, err := r.parse(b)
		if err != nil {
			return Event{}, fmt.Errorf("asciicast line %d: %w", r.line, err)
		}
		return e, nil
	}
	if err := r.scanner.Err(); err != nil {
		return Event{}, fmt.Errorf("asciicast line %d: %w", r.line+1, err)
	}
	return Event{}, io.EOF
}

func (r *Reader) parse(b []byte) (Event, error) {
	var fields []json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return Event{}, err
	}
	if len(fields) != 3 {
		return Event{}, fmt.Errorf("event must have three fields")
	}
	// Reject nulls as well as wrong JSON types.
	var interval *float64
	var code, data *string
	if err := json.Unmarshal(fields[0], &interval); err != nil {
		return Event{}, err
	}
	if err := json.Unmarshal(fields[1], &code); err != nil {
		return Event{}, err
	}
	if err := json.Unmarshal(fields[2], &data); err != nil {
		return Event{}, err
	}
	if interval == nil || code == nil || data == nil || *code == "" {
		return Event{}, fmt.Errorf("invalid event fields")
	}
	at, err := seconds(*interval)
	if err != nil {
		return Event{}, err
	}
	if r.Header.Version == 3 {
		if at > time.Duration(math.MaxInt64)-r.at {
			return Event{}, fmt.Errorf("event time overflow")
		}
		at += r.at
	}
	if at < r.at {
		return Event{}, fmt.Errorf("event times must not decrease")
	}
	r.at = at
	return Event{at, *code, *data}, nil
}

// Writer writes v3 output events. It buffers incomplete UTF-8 across PTY reads;
// genuinely invalid UTF-8 is replaced by JSON's U+FFFD, as required by the format.
// Input, environment, and command metadata are deliberately not collected.
type Writer struct {
	enc     *json.Encoder
	last    time.Duration
	pending []byte
	err     error
}

func NewWriter(dst io.Writer, term Terminal) (*Writer, error) {
	w := &Writer{enc: json.NewEncoder(dst)}
	w.enc.SetEscapeHTML(false)
	if err := w.enc.Encode(Header{Version: 3, Term: &term}); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) event(at time.Duration, code, data string) error {
	if w.err != nil {
		return w.err
	}
	if at < w.last {
		w.err = fmt.Errorf("asciicast output time moved backwards")
		return w.err
	}
	w.err = w.enc.Encode([3]any{(at - w.last).Seconds(), code, data})
	w.last = at
	return w.err
}

func (w *Writer) Output(at time.Duration, b []byte) error {
	if w.err != nil {
		return w.err
	}
	b = append(w.pending, b...)
	n := 0
	for n < len(b) && utf8.FullRune(b[n:]) {
		_, size := utf8.DecodeRune(b[n:])
		n += size
	}
	text := string(b[:n])
	w.pending = append([]byte(nil), b[n:]...)
	if n == 0 {
		return nil
	}
	return w.event(at, "o", text)
}

// End preserves trailing silence without inventing a process exit status.
func (w *Writer) End(at time.Duration) error {
	if len(w.pending) > 0 {
		if err := w.event(at, "o", string(w.pending)); err != nil {
			return err
		}
		w.pending = nil
	}
	return w.event(at, "m", "")
}
