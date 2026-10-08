package playback

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"

	"nimbleterminal.dev/ntrecord/asciicast"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

func TestSeekReconstructsKittyState(t *testing.T) {
	fixture, err := os.ReadFile("../../render/testdata/kitty.apc")
	if err != nil {
		t.Fatal(err)
	}
	var cast bytes.Buffer
	w, err := asciicast.NewWriter(&cast, asciicast.Terminal{Cols: 20, Rows: 5})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct {
		at   time.Duration
		data []byte
	}{{0, []byte("\x1b[?25l")}, {time.Second, fixture}, {3 * time.Second, []byte("\x1b_Ga=d,d=A,q=2\x1b\\")}} {
		if err = w.Output(e.at, e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err = w.End(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	p, err := New(context.Background(), bytes.NewReader(cast.Bytes()), tape.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, check := range []struct {
		at   time.Duration
		want color.RGBA
	}{{2 * time.Second, color.RGBA{240, 72, 88, 255}}, {4 * time.Second, color.RGBA{24, 26, 32, 255}}, {2 * time.Second, color.RGBA{240, 72, 88, 255}}, {0, color.RGBA{24, 26, 32, 255}}, {5 * time.Second, color.RGBA{24, 26, 32, 255}}} {
		img, err := p.Frame(context.Background(), check.at)
		if err != nil {
			t.Fatal(err)
		}
		if got := img.RGBAAt(2, 2); got != check.want {
			t.Fatalf("at %s: got %v, want %v", check.at, got, check.want)
		}
	}
	if p.Duration != 5*time.Second {
		t.Fatal(p.Duration)
	}
}

func TestReplayRejectsResizeAndHonorsCancellation(t *testing.T) {
	src := "{\"version\":2,\"width\":20,\"height\":5}\n[1,\"r\",\"40x5\"]"
	if _, err := New(context.Background(), strings.NewReader(src), tape.DefaultConfig()); err == nil {
		t.Fatal("resize accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(ctx, strings.NewReader(src), tape.DefaultConfig()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestReplaySynchronizedUpdateAndIgnoredInput(t *testing.T) {
	src := "{\"version\":3,\"term\":{\"cols\":20,\"rows\":5}}\n[0,\"o\",\"\\u001b[?25l\"]\n[1,\"o\",\"\\u001b[?2026h\\u001b[41m  \"]\n[0.5,\"i\",\"NEVER EXECUTE\"]\n[0.25,\"o\",\"\\u001b[?2026l\"]\n[1,\"m\",\"\"]"
	p, err := New(context.Background(), strings.NewReader(src), tape.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	blank := color.RGBA{24, 26, 32, 255}
	for _, at := range []time.Duration{1500 * time.Millisecond, 2 * time.Second, 1500 * time.Millisecond} {
		img, err := p.Frame(context.Background(), at)
		if err != nil {
			t.Fatal(err)
		}
		if (img.RGBAAt(15, 0) == blank) != (at < 1750*time.Millisecond) {
			t.Fatalf("hold wrong at %s", at)
		}
	}
}

func TestClockPauseSeekSpeedAndEnd(t *testing.T) {
	now := time.Unix(0, 0)
	c, err := NewClock(10*time.Second, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	c.Play(now)
	c.Update(now.Add(2 * time.Second))
	if c.Position != 2*time.Second {
		t.Fatal(c.Position)
	}
	c.Pause(now.Add(3 * time.Second))
	c.Update(now.Add(20 * time.Second))
	if c.Position != 3*time.Second {
		t.Fatal(c.Position)
	}
	c.SetSpeed(2, now.Add(20*time.Second))
	c.Play(now.Add(20 * time.Second))
	c.Update(now.Add(21 * time.Second))
	if c.Position != 5*time.Second {
		t.Fatal(c.Position)
	}
	c.Seek(time.Second, now.Add(21*time.Second))
	c.Update(now.Add(22 * time.Second))
	if c.Position != 3*time.Second {
		t.Fatal(c.Position)
	}
	c.Update(now.Add(30 * time.Second))
	if c.Position != c.Duration || !c.Paused {
		t.Fatal(c)
	}
	c.Play(now.Add(30 * time.Second))
	if c.Position != 0 || c.Paused {
		t.Fatal(c)
	}
	if err := c.SetSpeed(0, now); err == nil {
		t.Fatal("accepted zero speed")
	}
}
