package record

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nimbleterminal.dev/ntrecord/internal/tape"
	"nimbleterminal.dev/ntrecord/render"
)

// runDeterministic records a tape in deterministic mode and returns the GIF
// bytes and how long the recording took in real time.
func runDeterministic(t *testing.T, actions string) (gifBytes []byte, real time.Duration) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.gif")
	source := fmt.Sprintf("Output %q\nSet Shell /bin/sh\nSet Cols 40\nSet Rows 8\nSet TypingSpeed 20ms\n%s", path, actions)
	p, err := tape.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	p.Config.Mode = tape.Deterministic
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	start := time.Now()
	if err = Run(ctx, Options{Config: p.Config, Tape: p}); err != nil {
		t.Fatal(err)
	}
	real = time.Since(start)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b, real
}

func playbackTime(t *testing.T, b []byte) time.Duration {
	t.Helper()
	g, err := gif.DecodeAll(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var total time.Duration
	for _, d := range g.Delay {
		total += time.Duration(d) * 10 * time.Millisecond
	}
	return total
}

const sessionTape = `Type "echo one"
Enter
Wait+Screen /(?m)^one$/
Sleep 3s
Type "echo two"
Enter
Sleep 2s
`

// The same tape gives the same bytes and retains its virtual duration.
// Rendering/encoding speed depends on the host and race instrumentation; the
// clock translation is checked separately without that work.
func TestDeterministicRecordingsAreByteIdentical(t *testing.T) {
	var sums [3][32]byte
	for i := range sums {
		b, real := runDeterministic(t, sessionTape)
		sums[i] = sha256.Sum256(b)
		t.Logf("run %d encoded in %v", i, real)
		if got := playbackTime(t, b); got < 5*time.Second {
			t.Errorf("run %d plays for %v; the sleeps must still be in the recording", i, got)
		}
	}
	if sums[0] != sums[1] || sums[1] != sums[2] {
		t.Fatalf("identical tapes produced different bytes: %x %x %x", sums[0][:6], sums[1][:6], sums[2][:6])
	}
}

func TestDeterministicHiddenTimeIsOmitted(t *testing.T) {
	b, _ := runDeterministic(t, "Sleep 100ms\nHide\nSleep 5s\nShow\nSleep 100ms\n")
	if got := playbackTime(t, b); got > 1500*time.Millisecond {
		t.Fatalf("5s hidden sleep leaked into the recording: %v", got)
	}
}

func TestDeterministicTimingFollowsTheTape(t *testing.T) {
	b, _ := runDeterministic(t, "Sleep 1s\nSleep 2s\n")
	got := playbackTime(t, b)
	// 3s of sleeping plus the opening beat and the closing frame.
	if got < 3*time.Second || got > 3600*time.Millisecond {
		t.Fatalf("3s of sleeps played for %v", got)
	}
}

func TestDeterministicPlaybackSpeedScalesHolds(t *testing.T) {
	b, _ := runDeterministic(t, "Set PlaybackSpeed 2\nSleep 4s\n")
	if got := playbackTime(t, b); got < 2*time.Second || got > 2600*time.Millisecond {
		t.Fatalf("4s at speed 2 played for %v", got)
	}
}

// A blinking cursor must still blink through a long virtual hold.
func TestDeterministicCursorBlinks(t *testing.T) {
	b, _ := runDeterministic(t, "Set CursorBlink true\nSleep 3s\n")
	g, err := gif.DecodeAll(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) < 5 {
		t.Fatalf("a 3s hold with a blinking cursor has %d frames; want it to alternate", len(g.Image))
	}
}

func TestDeterministicNeedsATape(t *testing.T) {
	c := tape.DefaultConfig()
	c.Output = filepath.Join(t.TempDir(), "x.gif")
	c.Mode = tape.Deterministic
	err := Run(context.Background(), Options{Config: c, Command: []string{"true"}})
	if err == nil || !strings.Contains(err.Error(), "tape") {
		t.Fatalf("a live recording cannot be deterministic: %v", err)
	}
}

// A virtual Sleep does not wait for a slow program, so a recording that ends
// while the program is still drawing has missed it. Say so.
func TestDeterministicWarnsWhenOutputOutlastsTheTape(t *testing.T) {
	// Drive the capture boundary explicitly. A delayed shell echo can land
	// before the last capture on a busy runner, correctly producing no warning.
	r, err := render.New(render.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	last, err := r.FrameAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	var warning bytes.Buffer
	warnUncapturedOutput(r, 0, last, &warning)
	if warning.Len() != 0 {
		t.Fatal("unchanged viewport warned")
	}
	if err := r.Write([]byte("late output")); err != nil {
		t.Fatal(err)
	}
	warnUncapturedOutput(r, 0, last, &warning)
	if !strings.Contains(warning.String(), "still producing output") || !strings.Contains(warning.String(), "Wait") {
		t.Fatalf("missing late-output warning: %q", warning.String())
	}
	record := func(actions string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "out.gif")
		p, err := tape.Parse(strings.NewReader(fmt.Sprintf("Output %q\nSet Shell /bin/sh\nSet Cols 40\nSet Rows 6\nSet TypingSpeed 5ms\n%s", path, actions)))
		if err != nil {
			t.Fatal(err)
		}
		p.Config.Mode = tape.Deterministic
		var warn bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err = Run(ctx, Options{Config: p.Config, Tape: p, Warn: &warn}); err != nil {
			t.Fatal(err)
		}
		return warn.String()
	}
	if quiet := record("Type \"echo done\"\nEnter\nWait+Screen /(?m)^done$/\nSleep 200ms\n"); quiet != "" {
		t.Errorf("warned about a tape that waited properly: %q", quiet)
	}
}

// Verify that even a one-hour Sleep produces a Hold immediately, rather than
// timing PTY startup and GIF encoding against an arbitrary wall-clock budget.
func TestDeterministicSleepProducesHolds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := make(chan event)
	done := make(chan error, 1)
	go func() { done <- play(ctx, []tape.Action{{Command: "Sleep", Duration: time.Hour}}, events, true) }()
	var holds []time.Duration
	for {
		select {
		case ev := <-events:
			if ev.control == nil {
				t.Fatal("expected a tape control")
			}
			if ev.control.action.Command == "Hold" {
				holds = append(holds, ev.control.action.Duration)
			}
			ev.control.ack <- nil
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			if len(holds) != 2 || holds[0] != 100*time.Millisecond || holds[1] != time.Hour {
				t.Fatalf("unexpected virtual holds: %v", holds)
			}
			return
		case <-ctx.Done():
			t.Fatal("virtual Sleep waited for wall time")
		}
	}
}

// screens composes a GIF's frames and counts the distinct pictures shown.
func screens(t *testing.T, b []byte) int {
	t.Helper()
	g, err := gif.DecodeAll(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	seen := map[[32]byte]bool{}
	for _, f := range g.Image {
		draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
		seen[sha256.Sum256(canvas.Pix)] = true
	}
	return len(seen)
}

// Hide records the screen as it was when the tape hid it, as realtime does:
// output after the last hold is not lost because the next one is hidden.
func TestDeterministicHideKeepsTheLastScreen(t *testing.T) {
	plain, _ := runDeterministic(t, "Sleep 200ms\nHide\nShow\nSleep 100ms\n")
	typed, _ := runDeterministic(t, "Sleep 200ms\nType@0ms \"x\"\nHide\nBackspace@0ms\nShow\nSleep 100ms\n")
	if p, x := screens(t, plain), screens(t, typed); x != p+1 {
		t.Fatalf("the x typed before Hide was never shown: %d pictures, %d without it", x, p)
	}
}

// A synchronized update left open holds the previous picture for a second of
// recorded time, as realtime holds it for a second of wall time.
func TestDeterministicHoldsSynchronizedUpdate(t *testing.T) {
	b, _ := runDeterministic(t, "Type@0ms \"printf '\\033[?2026h\\033[41mRED'\"\nEnter@0ms\nSleep 3s\n")
	g, err := gif.DecodeAll(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	var red time.Duration
	for i, f := range g.Image {
		draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
		for p := 0; p < len(canvas.Pix); p += 4 {
			if r, gr, b := int(canvas.Pix[p]), int(canvas.Pix[p+1]), int(canvas.Pix[p+2]); r-gr > 70 && r-b > 70 {
				red += time.Duration(g.Delay[i]) * 10 * time.Millisecond
				break
			}
		}
	}
	if red < 1500*time.Millisecond || red > 2500*time.Millisecond {
		t.Fatalf("red shown for %v of a 3s hold; want about 2s, after the 1s synchronized-update hold", red)
	}
}
