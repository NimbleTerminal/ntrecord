package asciicast

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// FuzzReader feeds arbitrary bytes to NewReader and Next, checking that
// accepted headers and events satisfy the format invariants and that output
// events round-trip through Writer and Reader.
func FuzzReader(f *testing.F) {
	f.Add([]byte("{\"version\":2,\"width\":80,\"height\":24}\n[0.5,\"o\",\"hello\\r\\n\"]\n[1.2,\"m\",\"checkpoint\"]\n"))
	f.Add([]byte("{\"version\":3,\"term\":{\"cols\":80,\"rows\":24,\"theme\":{\"fg\":\"#ffffff\",\"bg\":\"#000000\",\"palette\":\"#000000:#ff0000:#00ff00:#ffff00:#0000ff:#ff00ff:#00ffff:#ffffff\"}}}\n# comment\n[0.25,\"o\",\"hello 🐈\\u001b_Ga=T;YWJj\\u001b\\\\\"]\n[0.75,\"m\",\"\"]\n"))
	f.Add([]byte("{\"version\":2,\"width\":80,\"height\":24,\"duration\":3.5,\"theme\":{\"fg\":\"#e0e4eb\",\"bg\":\"#181a20\",\"palette\":\"#000:#111:#222:#333:#444:#555:#666:#777:#888:#999:#aaa:#bbb:#ccc:#ddd:#eee:#fff\"}}\n[0,\"i\",\"ls\\n\"]\n[1,\"r\",\"100x40\"]\n[2,\"x\",\"0\"]\n"))
	f.Add([]byte("{\"version\":3,\"term\":{\"cols\":1,\"rows\":1}}\n[0,\"o\",\"\"]\n"))
	f.Add([]byte("{\"version\":2,\"width\":80,\"height\":24}\n[1,\"o\",\"a\"]\n[0,\"o\",\"b\"]\n"))
	f.Add([]byte("{\"version\":2,\"width\":0,\"height\":24}\n"))
	f.Add([]byte("null\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			return
		}
		r, err := NewReader(bytes.NewReader(data))
		if err != nil {
			return
		}
		if r.Header.Version != 2 && r.Header.Version != 3 {
			t.Fatalf("accepted version %d", r.Header.Version)
		}
		cols, rows := r.Header.Size()
		if cols < 1 || rows < 1 || cols > 8192 || rows > 8192 {
			t.Fatalf("accepted out-of-range grid %dx%d", cols, rows)
		}
		if colors := r.Header.Colors(); colors != nil {
			if s, err := colors.ThemeJSON(); err == nil && !json.Valid([]byte(s)) {
				t.Fatalf("ThemeJSON produced invalid JSON: %q", s)
			}
		}

		var (
			prevAt  time.Duration
			outputs []Event
			wantOut strings.Builder
		)
		for i := 0; i < 256; i++ {
			e, err := r.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return
			}
			if e.Code == "" {
				t.Fatal("accepted event with empty code")
			}
			if e.At < 0 || e.At < prevAt {
				t.Fatalf("event time %v decreased from %v", e.At, prevAt)
			}
			prevAt = e.At
			if e.Code == "o" && e.Data != "" && utf8.ValidString(e.Data) {
				outputs = append(outputs, e)
				wantOut.WriteString(e.Data)
			}
		}

		if len(outputs) == 0 {
			return
		}
		var buf bytes.Buffer
		w, err := NewWriter(&buf, Terminal{Cols: cols, Rows: rows, Theme: r.Header.Colors()})
		if err != nil {
			t.Fatalf("NewWriter: %v", err)
		}
		for _, e := range outputs {
			if err := w.Output(e.At, []byte(e.Data)); err != nil {
				t.Fatalf("Writer.Output: %v", err)
			}
		}
		if err := w.End(prevAt); err != nil {
			t.Fatalf("Writer.End: %v", err)
		}
		again, err := NewReader(&buf)
		if err != nil {
			t.Fatalf("re-read header: %v", err)
		}
		var gotOut strings.Builder
		var lastAt time.Duration
		for {
			e, err := again.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("re-read event: %v", err)
			}
			if e.At < lastAt {
				t.Fatalf("re-read event time %v decreased from %v", e.At, lastAt)
			}
			lastAt = e.At
			if e.Code == "o" {
				gotOut.WriteString(e.Data)
			}
		}
		if gotOut.String() != wantOut.String() {
			t.Fatalf("round-trip output mismatch:\ngot:  %q\nwant: %q", gotOut.String(), wantOut.String())
		}
	})
}

// FuzzWriterUTF8 splits arbitrary byte streams across Writer.Output calls and
// verifies that buffered multi-byte UTF-8 sequences reassemble cleanly.
func FuzzWriterUTF8(f *testing.F) {
	f.Add([]byte("hello 🐈\x1b_Ga=T;YWJj\x1b\\"), uint8(7), uint16(10), uint16(20))
	f.Add([]byte("€£¥𐍈"), uint8(1), uint16(0), uint16(5))
	f.Add([]byte("\xe2\x82\xac\xf0\x9f\x90\x88"), uint8(2), uint16(100), uint16(200))
	f.Add([]byte("\xff\xfe\xfd\x80"), uint8(2), uint16(1), uint16(1))

	f.Fuzz(func(t *testing.T, data []byte, split uint8, dt1, dt2 uint16) {
		if len(data) > 16*1024 {
			return
		}
		var buf bytes.Buffer
		w, err := NewWriter(&buf, Terminal{Cols: 80, Rows: 24})
		if err != nil {
			t.Fatal(err)
		}
		t1 := time.Duration(dt1) * time.Millisecond
		t2 := t1 + time.Duration(dt2)*time.Millisecond
		cut := 0
		if len(data) > 0 {
			cut = int(split) % (len(data) + 1)
		}
		if err := w.Output(t1, data[:cut]); err != nil {
			t.Fatalf("first Output: %v", err)
		}
		if err := w.Output(t2, data[cut:]); err != nil {
			t.Fatalf("second Output: %v", err)
		}
		if err := w.End(t2); err != nil {
			t.Fatalf("End: %v", err)
		}

		r, err := NewReader(&buf)
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}
		var got strings.Builder
		var last Event
		for {
			e, err := r.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if !utf8.ValidString(e.Data) {
				t.Fatalf("Reader returned invalid UTF-8: %q", e.Data)
			}
			if e.Code == "o" {
				got.WriteString(e.Data)
			}
			last = e
		}
		if last.Code != "m" {
			t.Fatalf("final event code = %q, want m", last.Code)
		}
		if utf8.Valid(data) && got.String() != string(data) {
			t.Fatalf("valid UTF-8 changed across split writes:\ngot:  %q\nwant: %q", got.String(), string(data))
		}
	})
}
