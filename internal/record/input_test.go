package record

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"nimbleterminal.dev/ntrecord/internal/tape"
	"nimbleterminal.dev/ntrecord/internal/testvideo"
)

func TestLiveInterrupt(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		delay               time.Duration
		want                bool
	}{
		{"double", "\x03", "\x03", 100 * time.Millisecond, true},
		{"boundary", "\x03", "\x03", time.Second, true},
		{"slow", "\x03", "\x03", time.Second + time.Nanosecond, false},
		{"intervening input", "\x03x", "\x03", 100 * time.Millisecond, false},
		{"ordinary input", "abc", "\x03", 100 * time.Millisecond, false},
		{"same read", "", "\x03\x03", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s liveInterrupt
			now := time.Now()
			if s.stop([]byte(tc.first), now) {
				t.Fatal("first press stopped recording")
			}
			if got := s.stop([]byte(tc.second), now.Add(tc.delay)); got != tc.want {
				t.Fatalf("stop = %v, want %v", got, tc.want)
			}
		})
	}
}

type readyInput struct {
	ready chan struct{}
	io.Reader
}

func (r readyInput) Read(b []byte) (int, error) {
	<-r.ready
	return r.Reader.Read(b)
}

type readyOutput struct {
	ready chan struct{}
	once  sync.Once
}

func (w *readyOutput) Write(b []byte) (int, error) {
	w.once.Do(func() { close(w.ready) })
	return len(b), nil
}

// Exercise normal finalization, including FFmpeg's container flush, while the
// interactive shell is still running. A timeout must not be what ends the run.
func TestDoubleInterruptSavesRecording(t *testing.T) {
	for _, format := range []string{"gif", "mp4"} {
		t.Run(format, func(t *testing.T) {
			if format == "mp4" {
				testvideo.Require(t)
			}
			c := tape.DefaultConfig()
			c.Cols, c.Rows = 20, 3
			c.Output = filepath.Join(t.TempDir(), "saved."+format)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ready := make(chan struct{})
			// Release blocked input even when setup fails before the shell starts.
			out := &readyOutput{ready: ready}
			defer out.once.Do(func() { close(ready) })
			err := Run(ctx, Options{Config: c, Input: readyInput{ready, strings.NewReader("\x03\x03")}, Output: out, StopOnDoubleInterrupt: true})
			if err != nil {
				t.Fatal(err)
			}
			if format == "gif" {
				assertGIFSaved(t, c.Output)
			} else if len(testvideo.Decode(t, c.Output)) == 0 {
				t.Fatal("MP4 contains no frames")
			}
		})
	}
}
