package record

import (
	"context"
	"fmt"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/tape"
)

// recordedDuration records a tape and returns the GIF's total playback time.
func recordedDuration(t *testing.T, actions string) time.Duration {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.gif")
	source := fmt.Sprintf("Output %q\nSet Shell /bin/sh\nSet Cols 20\nSet Rows 4\nSet Framerate 20\n%s", path, actions)
	p, err := tape.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = Run(ctx, Options{Config: p.Config, Tape: p}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	var total time.Duration
	for _, d := range g.Delay {
		total += time.Duration(d) * 10 * time.Millisecond
	}
	return total
}

// Hide closes the visible segment with at least one frame period so its last
// viewport is never rounded away. That may overcount, but by no more than one
// period per Hide: the timeline must not drift without bound.
func TestHideAddsAtMostOneFramePeriodEach(t *testing.T) {
	const (
		cycles = 12
		sleep  = 150 * time.Millisecond
		period = 50 * time.Millisecond  // Framerate 20
		slack  = 300 * time.Millisecond // scheduler jitter on busy CI hosts
	)
	baseline := recordedDuration(t, "Sleep 100ms\nSleep 200ms\n")
	cycled := recordedDuration(t, "Sleep 100ms\n"+strings.Repeat("Hide\nShow\nSleep 150ms\n", cycles)+"Sleep 200ms\n")
	extra := cycled - baseline - cycles*sleep
	if extra < -slack || extra > cycles*period+slack {
		t.Fatalf("%d Hide/Show cycles changed the timeline by %v (baseline %v, cycled %v); want within [-%v, %v]",
			cycles, extra, baseline, cycled, slack, cycles*period+slack)
	}
}

func TestHiddenTimeIsOmitted(t *testing.T) {
	hidden := recordedDuration(t, "Sleep 100ms\nHide\nSleep 1500ms\nShow\nSleep 100ms\n")
	if hidden > 1200*time.Millisecond {
		t.Fatalf("1.5s hidden sleep leaked into the recording: %v", hidden)
	}
}
