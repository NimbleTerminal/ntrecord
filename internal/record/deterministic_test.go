package record

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/tape"
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

// The point of the mode: the same tape gives the same bytes, and virtual time
// (here 5s of sleeping) costs no real time.
func TestDeterministicRecordingsAreByteIdentical(t *testing.T) {
	var sums [3][32]byte
	for i := range sums {
		b, real := runDeterministic(t, sessionTape)
		sums[i] = sha256.Sum256(b)
		if real > 4*time.Second {
			t.Errorf("run %d took %v in real time; 5s of Sleep should be virtual", i, real)
		}
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
	late := record("Type \"(sleep 0.12; echo late) &\"\nEnter\nSleep 50ms\n")
	if !strings.Contains(late, "still producing output") || !strings.Contains(late, "Wait") {
		t.Errorf("no warning for a program that outlasted the tape: %q", late)
	}
	if quiet := record("Type \"echo done\"\nEnter\nWait+Screen /(?m)^done$/\nSleep 200ms\n"); quiet != "" {
		t.Errorf("warned about a tape that waited properly: %q", quiet)
	}
}
