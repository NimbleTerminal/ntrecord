package record

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/asciicast"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
)

func TestCastPTYKittyRoundTrip(t *testing.T) {
	c := tape.DefaultConfig()
	c.Cols, c.Rows = 20, 5
	c.Theme = `{"background":"#123456"}`
	dir := t.TempDir()
	c.Outputs = []string{filepath.Join(dir, "session.cast"), filepath.Join(dir, "session.txt")}
	t.Setenv("SIGMAMAX_PRIVATE_TEST", "must-not-be-metadata")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := Run(ctx, Options{Config: c, Command: []string{"/bin/sh", "-c", `printf '\033[?25l'; cat "$1"; printf '\033[2;1HHELLO 👍🏽'; sleep 0.15; exit 7`, "cast-test", "../render/testdata/kitty.apc"}})
	if err == nil || !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("lost exit failure: %v", err)
	}
	data, err := os.ReadFile(c.Outputs[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("must-not-be-metadata")) || bytes.Contains(data, []byte(`"env"`)) || bytes.Contains(data, []byte(`"command"`)) {
		t.Fatal("private metadata included")
	}
	reader, err := asciicast.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if reader.Header.Term.Theme.BG != "#123456" {
		t.Fatal("theme not recorded")
	}
	var output strings.Builder
	var end time.Duration
	for {
		e, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if e.Code == "o" {
			output.WriteString(e.Data)
		}
		end = e.At
	}
	if !strings.HasPrefix(output.String(), "\x1b[?2027h") {
		t.Fatal("cast omitted initial grapheme mode")
	}
	if !strings.Contains(output.String(), "HELLO 👍🏽") || !strings.Contains(output.String(), "\x1b_Ga=T") || end < 100*time.Millisecond {
		t.Fatalf("missing output or trailing time: %q %s", output.String(), end)
	}
	c.Outputs = nil
	c.Output = filepath.Join(dir, "frames") + "/"
	if err := RenderCast(ctx, bytes.NewReader(data), c); err != nil {
		t.Fatal(err)
	}
	frames, err := filepath.Glob(filepath.Join(c.Output, "*.png"))
	if err != nil || len(frames) == 0 {
		t.Fatalf("no frames: %v", err)
	}
	f, err := os.Open(frames[len(frames)-1])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(img.At(2, 2)); got != (color.RGBA{240, 72, 88, 255}) {
		t.Fatalf("Kitty image lost: %v", got)
	}
	if got := color.RGBAModel.Convert(img.At(img.Bounds().Max.X-1, img.Bounds().Max.Y-1)); got != (color.RGBA{18, 52, 86, 255}) {
		t.Fatalf("theme lost: %v", got)
	}
	c.Output = filepath.Join(dir, "replay.txt")
	if err := RenderCast(ctx, bytes.NewReader(data), c); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "session.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(c.Output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("screen mismatch:\n%s\nwant:\n%s", got, want)
	}
}

func TestCastUnsupportedActionsBeforeExecution(t *testing.T) {
	for _, action := range []string{"Hide", "ScrollUp", "ScrollDown"} {
		c := tape.DefaultConfig()
		c.Output = filepath.Join(t.TempDir(), "out.cast")
		tape := &tape.Tape{Config: c, Actions: []tape.Action{{Command: action}}}
		if _, err := Validate(tape); err == nil || !strings.Contains(err.Error(), action) {
			t.Fatalf("validate accepted %s: %v", action, err)
		}
		err := Run(context.Background(), Options{Config: c, Tape: tape, Command: []string{"/nonexistent"}})
		if err == nil || !strings.Contains(err.Error(), action) {
			t.Fatalf("run accepted %s: %v", action, err)
		}
		if _, err := os.Stat(c.Output); !os.IsNotExist(err) {
			t.Fatal("created rejected cast")
		}
	}
}

func TestCastDeterministicTiming(t *testing.T) {
	c := tape.DefaultConfig()
	c.Mode = tape.Deterministic
	c.PlaybackSpeed = 2
	c.Cols, c.Rows = 10, 2
	c.Output = filepath.Join(t.TempDir(), "virtual.cast")
	tape := &tape.Tape{Config: c, Actions: []tape.Action{{Command: "Sleep", Duration: 2 * time.Second}}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Run(ctx, Options{Config: c, Tape: tape, Command: []string{"/bin/sh", "-c", "printf READY; sleep 10"}}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(c.Output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := asciicast.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var end time.Duration
	for {
		e, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		end = e.At
		if e.Code == "o" && e.At != 0 {
			t.Fatalf("output used wall clock: %s", e.At)
		}
	}
	if end != 1100*time.Millisecond {
		t.Fatalf("virtual end=%s, want 1.1s (startup + Sleep at 2x, then final hold)", end)
	}
}

func TestRenderCastFailurePreservesDestination(t *testing.T) {
	for _, body := range []string{`[0,"o","first"]` + "\n" + `[0.1,"r","90x30"]`, `[0,"o","first"]` + "\n" + `[null,"o","bad"]`} {
		c := tape.DefaultConfig()
		c.Output = filepath.Join(t.TempDir(), "out.gif")
		if err := os.WriteFile(c.Output, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
		src := "{\"version\":3,\"term\":{\"cols\":20,\"rows\":5}}\n" + body
		if err := RenderCast(context.Background(), strings.NewReader(src), c); err == nil {
			t.Fatal("accepted unsupported cast")
		}
		b, err := os.ReadFile(c.Output)
		if err != nil || string(b) != "original" {
			t.Fatalf("overwrote destination: %s %v", b, err)
		}
		leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(c.Output), ".sigmamax-*"))
		if len(leftovers) > 0 {
			t.Fatal(leftovers)
		}
	}
}

func TestRenderCastFinalOutputBetweenFrames(t *testing.T) {
	c := tape.DefaultConfig()
	c.Output = filepath.Join(t.TempDir(), "out.txt")
	src := "{\"version\":2,\"width\":20,\"height\":5}\n[0.01,\"o\",\"FINAL\"]"
	if err := RenderCast(context.Background(), strings.NewReader(src), c); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(c.Output)
	if err != nil || !bytes.Contains(b, []byte("FINAL")) {
		t.Fatalf("lost final event: %s %v", b, err)
	}
}

func TestCastCancellationFinalizes(t *testing.T) {
	c := tape.DefaultConfig()
	c.Output = filepath.Join(t.TempDir(), "cancelled.cast")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancel only after output arrives, avoiding startup timing assumptions.
	err := Run(ctx, Options{Config: c, Command: []string{"/bin/sh", "-c", "printf SAVED; sleep 10"}, Output: cancelOutput{cancel}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	b, err := os.ReadFile(c.Output)
	if err != nil || !bytes.Contains(b, []byte("SAVED")) {
		t.Fatalf("lost cancelled cast: %s %v", b, err)
	}
	r, err := asciicast.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var last asciicast.Event
	for {
		e, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		last = e
	}
	if last.Code != "m" {
		t.Fatalf("missing final duration: %+v", last)
	}
}

type cancelOutput struct{ cancel context.CancelFunc }

func (c cancelOutput) Write(b []byte) (int, error) { c.cancel(); return len(b), nil }

func TestRenderCastTimingAndInputIgnored(t *testing.T) {
	// v3 deltas accumulate to 1s, then speed=2 makes ten 50ms frames.
	src := "{\"version\":3,\"term\":{\"cols\":10,\"rows\":2}}\n[0,\"o\",\"VISIBLE\"]\n[0.2,\"i\",\"SECRET\"]\n[0.3,\"future\",\"UNKNOWN\"]\n[0.5,\"m\",\"\"]"
	c := tape.DefaultConfig()
	c.PlaybackSpeed = 2
	dir := t.TempDir()
	c.Outputs = []string{filepath.Join(dir, "frames") + "/", filepath.Join(dir, "screen.txt")}
	if err := RenderCast(context.Background(), strings.NewReader(src), c); err != nil {
		t.Fatal(err)
	}
	frames, _ := filepath.Glob(filepath.Join(dir, "frames", "*.png"))
	if len(frames) != 10 {
		t.Fatalf("got %d frames, want 10", len(frames))
	}
	b, err := os.ReadFile(c.Outputs[1])
	if err != nil || !bytes.Contains(b, []byte("VISIBLE")) || bytes.Contains(b, []byte("SECRET")) || bytes.Contains(b, []byte("UNKNOWN")) {
		t.Fatalf("wrong output: %s %v", b, err)
	}
}

func TestRenderCastSynchronizedUpdate(t *testing.T) {
	// The initial held update must retain the blank frame until the release.
	var cast bytes.Buffer
	w, err := asciicast.NewWriter(&cast, asciicast.Terminal{Cols: 10, Rows: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Output(0, []byte("\x1b[?2026h\x1b[?25l\x1b[41m  \x1b[0m")); err != nil {
		t.Fatal(err)
	}
	if err = w.Output(100*time.Millisecond, []byte("\x1b[?2026l")); err != nil {
		t.Fatal(err)
	}
	if err = w.End(200 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	c := tape.DefaultConfig()
	c.Output = filepath.Join(t.TempDir(), "frames") + "/"
	if err = RenderCast(context.Background(), &cast, c); err != nil {
		t.Fatal(err)
	}
	pixel := func(name string) color.Color {
		f, err := os.Open(filepath.Join(c.Output, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		return color.RGBAModel.Convert(img.At(15, 0))
	}
	bg := color.RGBA{24, 26, 32, 255}
	if first := pixel("000001.png"); first != bg {
		t.Fatalf("partial update leaked: %v", first)
	}
	if released := pixel("000003.png"); released == bg {
		t.Fatal("held update never released")
	}
}

func TestRenderCastFlushesUnfinishedSynchronizedUpdate(t *testing.T) {
	src := "{\"version\":3,\"term\":{\"cols\":10,\"rows\":2}}\n[0,\"o\",\"\\u001b[?2026h\\u001b[?25l\\u001b[41m  \"]\n[0.1,\"m\",\"\"]"
	c := tape.DefaultConfig()
	c.Output = filepath.Join(t.TempDir(), "frames") + "/"
	if err := RenderCast(context.Background(), strings.NewReader(src), c); err != nil {
		t.Fatal(err)
	}
	frames, _ := filepath.Glob(filepath.Join(c.Output, "*.png"))
	if len(frames) != 3 {
		t.Fatalf("final held output was not flushed: %d frames", len(frames))
	}
	f, err := os.Open(frames[2])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(img.At(15, 0)); got == (color.RGBA{24, 26, 32, 255}) {
		t.Fatal("final held frame is blank")
	}
}
