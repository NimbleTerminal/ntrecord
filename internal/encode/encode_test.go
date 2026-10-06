package encode

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/testvideo"
)

func TestVideoTimingPixelsAndPadding(t *testing.T) {
	testvideo.Require(t)
	for _, format := range []string{"mp4", "webm"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "spaces and 'quotes'."+format)
			e, err := New(path, 63, 47)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Abort()
			// Exercise a nonzero origin and padded stride, too.
			frame := image.NewRGBA(image.Rect(0, 0, 100, 100)).SubImage(image.Rect(10, 20, 73, 67)).(*image.RGBA)
			draw.Draw(frame, frame.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, 255}}, image.Point{}, draw.Src)
			for range 50 {
				if err := e.Add(frame, 10*time.Millisecond); err != nil {
					t.Fatal(err)
				}
			}
			draw.Draw(frame, frame.Bounds(), &image.Uniform{color.RGBA{0, 0, 255, 255}}, image.Point{}, draw.Src)
			if err := e.Add(frame, 500*time.Millisecond); err != nil {
				t.Fatal(err)
			}
			if err := e.Finish(); err != nil {
				t.Fatal(err)
			}
			probe, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-show_format", "-of", "json", path).Output()
			if err != nil {
				t.Fatal(err)
			}
			var info struct {
				Streams []struct {
					Codec         string `json:"codec_name"`
					Width, Height int
				}
				Format struct{ Duration string }
			}
			if err := json.Unmarshal(probe, &info); err != nil {
				t.Fatal(err)
			}
			codec := "h264"
			if format == "webm" {
				codec = "vp9"
			}
			if len(info.Streams) != 1 || info.Streams[0].Codec != codec || info.Streams[0].Width != 64 || info.Streams[0].Height != 48 || info.Format.Duration != "1.000000" {
				t.Fatalf("incorrect video metadata: %s", probe)
			}
			pixels := testvideo.Decode(t, path)
			const size = 64 * 48 * 3
			if len(pixels) != 20*size {
				t.Fatalf("got %d bytes, want 20 frames", len(pixels))
			}
			for i := range 20 {
				p := pixels[i*size+(20*64+20)*3:]
				if i < 10 && !(p[0] > 220 && p[1] < 30 && p[2] < 30) || i >= 10 && !(p[2] > 220 && p[0] < 30 && p[1] < 30) {
					t.Fatalf("frame %d wrong color: %v", i, p[:3])
				}
			}
		})
	}
}

func TestMissingFFmpegAndInvalidOutput(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, format := range []string{"mp4", "webm", "mpeg"} {
		if _, err := New(filepath.Join(t.TempDir(), "out."+format), 2, 2); err == nil {
			t.Fatalf("accepted %s without an encoder", format)
		}
	}
	e, err := New(filepath.Join(t.TempDir(), "out.gif"), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Abort()
	if err := e.Add(image.NewRGBA(image.Rect(0, 0, 2, 2)), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := e.Finish(); err != nil {
		t.Fatal(err)
	}
}

func TestVideoFailurePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	// Simulate an encoder that passes preflight but fails during finalization.
	script := "#!/bin/sh\nif [ \"$2\" = -encoders ]; then echo ' V..... libx264 fake'; exit 0; fi\n/bin/cat >/dev/null\necho 'fixture encoder failure' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	path := filepath.Join(dir, "out.mp4")
	old := []byte("existing recording")
	if err := os.WriteFile(path, old, 0644); err != nil {
		t.Fatal(err)
	}
	e, err := New(path, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Abort()
	if err := e.Add(image.NewRGBA(image.Rect(0, 0, 2, 2)), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := e.Finish(); err == nil || !strings.Contains(err.Error(), "fixture encoder failure") {
		t.Fatalf("got %v", err)
	}
	e.Abort()
	b, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(b, old) {
		t.Fatalf("destination changed: %q, %v", b, err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".sigmamax-*"))
	if len(files) != 0 {
		t.Fatalf("leaked temporary files: %v", files)
	}
}

func TestVideoEmptyAndAbort(t *testing.T) {
	testvideo.Require(t)
	for _, finish := range []bool{false, true} {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.mp4")
		e, err := New(path, 2, 2)
		if err != nil {
			t.Fatal(err)
		}
		defer e.Abort()
		if finish {
			if err := e.Finish(); err == nil || !strings.Contains(err.Error(), "no visible frames") {
				t.Fatalf("got %v", err)
			}
		}
		e.Abort()
		files, _ := os.ReadDir(dir)
		if len(files) != 0 {
			t.Fatalf("leaked output: %v", files)
		}
	}
}
