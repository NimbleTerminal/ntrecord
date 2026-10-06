package encode

import (
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/testvideo"
)

func TestConfiguredFramerate(t *testing.T) {
	testvideo.Require(t)
	path := filepath.Join(t.TempDir(), "rate.mp4")
	e, err := NewWithOptions(path, 16, 16, Options{Framerate: 30})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Abort()
	if err = e.Add(image.NewRGBA(image.Rect(0, 0, 16, 16)), time.Second); err != nil {
		t.Fatal(err)
	}
	if err = e.Finish(); err != nil {
		t.Fatal(err)
	}
	b, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=r_frame_rate,nb_frames", "-of", "json", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Streams []struct {
			Rate   string `json:"r_frame_rate"`
			Frames string `json:"nb_frames"`
		}
	}
	if err = json.Unmarshal(b, &info); err != nil {
		t.Fatal(err)
	}
	if len(info.Streams) != 1 || info.Streams[0].Rate != "30/1" || info.Streams[0].Frames != "30" {
		t.Fatalf("wrong rate: %s", b)
	}
}

func TestLoopOffsetPreservesDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop.gif")
	e, err := NewWithOptions(path, 4, 4, Options{Framerate: 20, LoopOffset: "50%"})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Abort()
	frame := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for _, c := range []color.RGBA{{255, 0, 0, 255}, {0, 0, 255, 255}} {
		draw.Draw(frame, frame.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
		for range 10 {
			if err = e.Add(frame, 50*time.Millisecond); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = e.Finish(); err != nil {
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
	r, _, b, _ := g.Image[0].At(1, 1).RGBA()
	if b <= r {
		t.Fatal("loop did not start on blue frame")
	}
	total := 0
	for _, d := range g.Delay {
		total += d
	}
	if total != 100 {
		t.Fatalf("lost duration: %d", total)
	}
}

func TestPNGSequencePreservesExistingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "frames")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir+"/", 2, 2); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
}
