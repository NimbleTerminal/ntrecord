package render

import (
	"bytes"
	"image/color"
	"strings"
	"testing"
	"time"

	"nimbleterminal.dev/ntrecord/internal/tape"
)

func TestKeysAndPasteModes(t *testing.T) {
	r, e := New(tape.DefaultConfig())
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	cases := map[string]string{"Enter": "\r", "Tab": "\t", "Escape": "\x1b", "Ctrl+C": "\x03", "Alt+x": "\x1bx", "Shift+Tab": "\x1b[Z", "Up": "\x1b[A", "Home": "\x1b[H", "Backspace": "\x7f", "Ctrl+@": "\x00"}
	for key, want := range cases {
		b, e := r.Key(key)
		if e != nil || string(b) != want {
			t.Errorf("%s = %q (%v), want %q", key, b, e, want)
		}
	}
	if e = r.Write([]byte("\x1b[?1h\x1b[?2004h")); e != nil {
		t.Fatal(e)
	}
	if b, e := r.Key("Up"); e != nil || string(b) != "\x1bOA" {
		t.Fatalf("application cursor: %q %v", b, e)
	}
	if b, e := r.Paste("hello\nworld"); e != nil || string(b) != "\x1b[200~hello\nworld\x1b[201~" {
		t.Fatalf("bracketed paste: %q %v", b, e)
	}
	if e = r.Write([]byte("\x1b[>1u")); e != nil {
		t.Fatal(e)
	}
	if b, e := r.Key("Escape"); e != nil || string(b) != "\x1b[27u" {
		t.Fatalf("Kitty keyboard: %q %v", b, e)
	}
}

func TestTypedTextKeyboardModes(t *testing.T) {
	r, err := New(tape.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, ch := range "Abé世界 !" {
		b, e := r.TypeRune(ch)
		if e != nil || string(b) != string(ch) {
			t.Fatalf("typed %q: %q %v", ch, b, e)
		}
	}
	if err = r.Write([]byte("\x1b[>8u")); err != nil {
		t.Fatal(err)
	}
	b, err := r.TypeRune('a')
	if err != nil || !bytes.HasPrefix(b, []byte("\x1b[")) {
		t.Fatalf("Kitty report-all ignored: %q %v", b, err)
	}
}

func TestTextWaitAndScroll(t *testing.T) {
	c := tape.DefaultConfig()
	c.Cols = 30
	c.Rows = 3
	r, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if e = r.Write([]byte("gone\r\x1b[2K世界 ready\r\nsecond\r\nthird\r\nfourth")); e != nil {
		t.Fatal(e)
	}
	text, e := r.Text("Screen")
	if e != nil || strings.Contains(text, "gone") || strings.Contains(text, "ready") || !strings.Contains(text, "fourth") {
		t.Fatalf("screen: %q %v", text, e)
	}
	if text, e = r.Text("Line"); e != nil || text != "fourth" {
		t.Fatalf("line: %q %v", text, e)
	}
	if e = r.Scroll(-1); e != nil {
		t.Fatal(e)
	}
	if text, e = r.Text("Screen"); e != nil || !strings.Contains(text, "世界 ready") {
		t.Fatalf("scrollback: %q %v", text, e)
	}
	if e = r.Scroll(1); e != nil {
		t.Fatal(e)
	}
	if text, e = r.Text("Screen"); e != nil || !strings.Contains(text, "fourth") {
		t.Fatalf("scroll bottom: %q %v", text, e)
	}
}

func TestThemeGeometryAndCursorBlink(t *testing.T) {
	c := tape.DefaultConfig()
	c.Cols = 10
	c.Rows = 3
	c.Padding = 8
	c.Margin = 12
	c.WindowBar = "colorful"
	c.BorderRadius = 8
	c.MarginFill = "#445566"
	c.Theme = `{"background":"#112233","foreground":"#ffffff","red":"#010203"}`
	c.CursorBlink = true
	r, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if e = r.Write([]byte("\x1b[31mX")); e != nil {
		t.Fatal(e)
	}
	fg, e := r.term.ColorPalette()
	if e != nil || fg[1].R != 1 || fg[1].G != 2 || fg[1].B != 3 {
		t.Fatal("theme palette not applied", e)
	}
	first, e := r.FrameAt(r.blinkStart)
	if e != nil {
		t.Fatal(e)
	}
	if first.Bounds().Dx() != r.Width+40 || first.Bounds().Dy() != r.Height+70 {
		t.Fatal("decoration dimensions", first.Bounds())
	}
	if got := first.RGBAAt(0, 0); got != (color.RGBA{0x44, 0x55, 0x66, 255}) {
		t.Fatal("margin", got)
	}
	if got := first.RGBAAt(30, 45); got != (color.RGBA{0x11, 0x22, 0x33, 255}) {
		t.Fatal("background", got)
	}
	second, e := r.FrameAt(r.blinkStart.Add(600 * time.Millisecond))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("cursor did not blink")
	}
	c.Theme = "Catppuccin Mocha"
	themed, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	themed.Close()
}
