//go:build libghostty_fuzz

package render

import (
	"testing"
	"unicode/utf8"
)

// FuzzWriteAndFrame feeds arbitrary VT sequences, OSC 7501 status updates,
// Kitty graphics APCs, and Unicode placeholders to a reusable Renderer and
// checks that Frame, Text, and ProgramStatuses preserve their invariants.
func FuzzWriteAndFrame(f *testing.F) {
	f.Add([]byte("hello \x1b[31;1mworld\x1b[0m\r\n"))
	f.Add([]byte("\x1b]7501;state=working;app=build;id=main/step;progress=50;title=Compile\x1b\\"))
	f.Add([]byte("\x1b_Ga=T,t=d,f=24,s=1,v=1,c=2,r=1,i=7,q=2;/wAA\x1b\\"))
	f.Add([]byte("\x1b_Ga=T,U=1,t=d,f=24,s=1,v=1,c=2,r=1,i=3,q=2;/wAA\x1b\\\x1b[38;5;3m\U0010eeee\u0305\u0305\U0010eeee\x1b[0m"))
	f.Add([]byte("\x1b[?1049h\x1b[2J\x1b[Halt screen\x1b[?1049l"))

	cfg := DefaultConfig()
	cfg.Cols = 20
	cfg.Rows = 8
	r, err := New(cfg)
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(r.Close)

	w, h := r.OutputSize()
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			return
		}
		if err := r.Write([]byte("\x1bc")); err != nil {
			t.Fatalf("reset Write: %v", err)
		}
		_ = r.TakeReplies()
		if err := r.Write(data); err != nil {
			t.Fatalf("Write: %v", err)
		}
		_ = r.TakeReplies()

		img, err := r.Frame()
		if err != nil {
			t.Fatalf("Frame: %v", err)
		}
		if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
			t.Fatalf("Frame bounds = %v, want %dx%d", img.Bounds(), w, h)
		}
		for _, scope := range []string{"Screen", "Line"} {
			txt, err := r.Text(scope)
			if err != nil {
				t.Fatalf("Text(%q): %v", scope, err)
			}
			if !utf8.ValidString(txt) {
				t.Fatalf("Text(%q) returned invalid UTF-8: %q", scope, txt)
			}
		}
		statuses := r.ProgramStatuses()
		if len(statuses) > 256 {
			t.Fatalf("ProgramStatuses length = %d, max 256", len(statuses))
		}
		for i, st := range statuses {
			if i > 0 && statuses[i-1].ID > st.ID {
				t.Fatalf("ProgramStatuses not sorted by ID: %q > %q", statuses[i-1].ID, st.ID)
			}
			if st.Progress < -1 || st.Progress > 100 {
				t.Fatalf("ProgramStatus progress out of bounds: %d", st.Progress)
			}
		}
		r.ProgramExited()
	})
}

// FuzzKey feeds arbitrary key specifications to Renderer.Key.
func FuzzKey(f *testing.F) {
	for _, seed := range []string{"Enter", "Tab", "Backspace", "Ctrl+C", "Ctrl+@", "Alt+Enter", "Shift+Up", "F12", "ScrollUp", "ScrollDown", "Ctrl+Alt+Delete", "unknown"} {
		f.Add(seed)
	}
	cfg := DefaultConfig()
	cfg.Cols = 20
	cfg.Rows = 8
	r, err := New(cfg)
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(r.Close)

	f.Fuzz(func(t *testing.T, spec string) {
		if len(spec) > 256 {
			return
		}
		_, _ = r.Key(spec)
	})
}
