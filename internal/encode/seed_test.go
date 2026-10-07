package encode

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGIFOptionsSeedKeepsThemeBackgroundFlat(t *testing.T) {
	bg := color.RGBA{30, 30, 46, 255}
	path := filepath.Join(t.TempDir(), "themed.gif")
	e, err := NewWithOptions(path, 16, 16, Options{Framerate: 20, Seed: []color.RGBA{bg}})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Abort()
	f := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for i := 0; i < len(f.Pix); i += 4 {
		copy(f.Pix[i:], []byte{bg.R, bg.G, bg.B, 255})
	}
	if err := e.Add(f, 500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := e.Finish(); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	g, err := gif.DecodeAll(file)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if got := color.RGBAModel.Convert(g.Image[0].At(x, y)); got != bg {
				t.Fatalf("pixel %d,%d = %v, want flat %v", x, y, got, bg)
			}
		}
	}
}
