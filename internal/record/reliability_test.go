package record

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/gif"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/term"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

type failedOutput struct{ err error }

func (w failedOutput) Write([]byte) (int, error) { return 0, w.err }

func assertGIFSaved(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil || len(g.Image) == 0 {
		t.Fatalf("invalid recording: %v", err)
	}
}

func TestForwardingErrorStillSavesRecording(t *testing.T) {
	c := tape.DefaultConfig()
	c.Cols, c.Rows = 20, 3
	c.Output = filepath.Join(t.TempDir(), "saved.gif")
	failure := errors.New("forwarded stdout closed")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := Run(ctx, Options{Config: c, Command: []string{"/bin/sh", "-c", "printf SAVED; sleep 5"}, Output: failedOutput{failure}})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	assertGIFSaved(t, c.Output)
}

func TestTapeFinalEnterDrainsOutput(t *testing.T) {
	for i := 0; i < 5; i++ {
		p, err := tape.Parse(strings.NewReader("Set TypingSpeed 0ms\nType `printf '\\106\\111\\116\\101\\114\\012'`\nEnter"))
		if err != nil {
			t.Fatal(err)
		}
		p.Config.Cols, p.Config.Rows = 60, 4
		p.Config.Output = filepath.Join(t.TempDir(), "out.txt")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = Run(ctx, Options{Config: p.Config, Tape: p})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(p.Config.Output)
		if err != nil || !strings.Contains(string(b), "FINAL") {
			t.Fatalf("lost final command output: %s (%v)", b, err)
		}
	}
}

func TestFailedVideoStillFinalizesOtherOutputs(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$2\" = -encoders ]; then echo ' V..... libx264 fake'; exit 0; fi\n/bin/cat >/dev/null\necho 'codec failed' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	c := tape.DefaultConfig()
	c.Cols, c.Rows = 20, 3
	c.Outputs = []string{filepath.Join(dir, "bad.mp4"), filepath.Join(dir, "good.gif"), filepath.Join(dir, "good.txt")}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := Run(ctx, Options{Config: c, Command: []string{"/bin/sh", "-c", "printf SAVED"}})
	if err == nil || !strings.Contains(err.Error(), "codec failed") {
		t.Fatal(err)
	}
	assertGIFSaved(t, c.Outputs[1])
	if b, err := os.ReadFile(c.Outputs[2]); err != nil || !bytes.Contains(b, []byte("SAVED")) {
		t.Fatalf("text lost: %s %v", b, err)
	}
}

// This child fills PTY output with device queries before reading a large paste.
// The recorder must keep reading and enqueue replies while its writer blocks.
func TestBackpressureHelper(t *testing.T) {
	if os.Getenv("NTRECORD_PRESSURE_HELPER") != "1" {
		return
	}
	if _, err := term.MakeRaw(int(os.Stdin.Fd())); err != nil {
		panic(err)
	}
	fmt.Print("READY")
	if _, err := io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
		panic(err)
	}
	chunk := strings.Repeat("x", 2045) + "\x1b[c"
	for range 2048 {
		if _, err := io.WriteString(os.Stdout, chunk); err != nil {
			panic(err)
		}
	}
	buf := make([]byte, 4096)
	var tail string
	for !strings.Contains(tail, "PASTE-END") {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			panic(err)
		}
		tail += string(buf[:n])
		if len(tail) > 8192 {
			tail = tail[len(tail)-8192:]
		}
	}
	fmt.Print("\r\nCOMPLETE\r\n")
	os.Exit(0)
}

func TestPasteAndRepliesDoNotDeadlock(t *testing.T) {
	c := tape.DefaultConfig()
	c.Cols, c.Rows = 20, 3
	c.Output = filepath.Join(t.TempDir(), "pressure.txt")
	c.Env = map[string]string{"NTRECORD_PRESSURE_HELPER": "1"}
	// Wait for raw mode before pasting; canonical PTYs deliberately limit input.
	p := &tape.Tape{Actions: []tape.Action{
		{Command: "Wait", Scope: "Screen", Text: "READY", Duration: 5 * time.Second},
		{Command: "Copy", Text: strings.Repeat("a", 1024*1024) + "PASTE-END"},
		{Command: "Paste"},
		{Command: "Wait", Scope: "Screen", Text: "COMPLETE", Duration: 5 * time.Second},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err := Run(ctx, Options{Config: c, Tape: p, Command: []string{os.Args[0], "-test.run=^TestBackpressureHelper$"}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(c.Output)
	if err != nil || !bytes.Contains(b, []byte("COMPLETE")) {
		t.Fatalf("missing completion: %s %v", b, err)
	}
}

func TestHideFlushesVisibleHoldAndOmitsHiddenFrames(t *testing.T) {
	// The first paint starts a synchronized-output hold. Hide must preserve
	// this last visible viewport, even though the periodic sampler holds it.
	source := "Set TypingSpeed 0ms\nSet Framerate 120\nSet PlaybackSpeed 10\n" +
		"Type `printf '\\033[?2026h\\033[48;2;0;0;255m\\033[2J\\033[H\\126\\111\\123\\111\\102\\114\\105'`\n" +
		"Enter\nWait+Screen /VISIBLE/\nHide\n" +
		"Type `printf '\\033[?2026l\\033[48;2;255;0;0m\\033[2J\\033[H\\110\\111\\104\\104\\105\\116'; sleep 0.1; printf '\\033[48;2;0;255;0m\\033[2J\\033[H\\104\\117\\116\\105'`\n" +
		"Enter\nWait+Screen /DONE/\nShow\nSleep 50ms"
	p, err := tape.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	p.Config.Cols, p.Config.Rows = 20, 3
	p.Config.Output = filepath.Join(t.TempDir(), "visibility.gif")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = Run(ctx, Options{Config: p.Config, Tape: p}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p.Config.Output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	blue, green := false, false
	// Look at the unused bottom-right corner, away from text and the cursor.
	for _, im := range g.Image {
		r, g, b, _ := im.At(im.Bounds().Max.X-1, im.Bounds().Max.Y-1).RGBA()
		if r > g*2 && r > b*2 {
			t.Fatal("hidden red viewport leaked")
		}
		blue = blue || b > 100*257 && b > r && b > g
		green = green || g > 100*257 && g > r && g > b
	}
	if !blue || !green {
		t.Fatalf("lost visibility boundary: blue=%v green=%v", blue, green)
	}
}
