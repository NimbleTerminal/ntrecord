package gifencode

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"
)

func TestDedupAndDelayOverflow(t *testing.T) {
	var e Encoder
	f := image.NewRGBA(image.Rect(0, 0, 2, 2))
	f.Set(0, 0, color.White)
	for _, d := range []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 700 * time.Second} {
		if err := e.Add(f, d); err != nil {
			t.Fatal(err)
		}
	}
	var b bytes.Buffer
	if err := e.Encode(&b); err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(&b)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) != 2 || g.Delay[0]+g.Delay[1] != 70010 {
		t.Fatalf("frames=%d delays=%v", len(g.Image), g.Delay)
	}
}

func TestPalettePreservesWhiteAndDefaults(t *testing.T) {
	colors := []color.RGBA{{255, 255, 255, 255}, {0, 0, 0, 255}, {24, 26, 32, 255}, {224, 228, 235, 255}}
	f := image.NewRGBA(image.Rect(0, 0, len(colors), 1))
	for x, c := range colors {
		f.SetRGBA(x, 0, c)
	}
	var e Encoder
	if err := e.Add(f, time.Second); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := e.Encode(&b); err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(&b)
	if err != nil {
		t.Fatal(err)
	}
	for x, want := range colors {
		if got := color.RGBAModel.Convert(g.Image[0].At(x, 0)); got != want {
			t.Errorf("pixel %d: %v, want %v", x, got, want)
		}
	}
}

func encodeDecode(t *testing.T, e *Encoder) *gif.GIF {
	t.Helper()
	var b bytes.Buffer
	if err := e.Encode(&b); err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(&b)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// A theme's flat background is not in the fixed palette. Without seeding it,
// dithering turns it into noise and bloats the file.
func TestSeededColorsAreExactAndFlat(t *testing.T) {
	bg, fg := color.RGBA{30, 30, 46, 255}, color.RGBA{205, 214, 244, 255}
	ansi := color.RGBA{243, 139, 168, 255}
	e := Encoder{Seed: color.Palette{bg, fg, ansi}}
	f := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			f.SetRGBA(x, y, bg)
		}
	}
	f.SetRGBA(1, 1, fg)
	f.SetRGBA(2, 1, ansi)
	if err := e.Add(f, time.Second); err != nil {
		t.Fatal(err)
	}
	im := encodeDecode(t, &e).Image[0]
	seen := map[color.RGBA]int{}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			seen[color.RGBAModel.Convert(im.At(x, y)).(color.RGBA)]++
		}
	}
	if len(seen) != 3 || seen[bg] != 32*32-2 || seen[fg] != 1 || seen[ansi] != 1 {
		t.Fatalf("seeded colors not preserved exactly: %v", seen)
	}
}

func TestSeedKeepsBlackWhiteAndThemeColors(t *testing.T) {
	var seed color.Palette
	for i := 0; i < 20; i++ {
		seed = append(seed, color.RGBA{uint8(i*12 + 1), uint8(i*7 + 3), uint8(255 - i*11), 255})
	}
	e := Encoder{Seed: seed}
	want := append([]color.RGBA{{0, 0, 0, 255}, {255, 255, 255, 255}}, color.RGBA{seed[3].(color.RGBA).R, seed[3].(color.RGBA).G, seed[3].(color.RGBA).B, 255})
	f := image.NewRGBA(image.Rect(0, 0, len(want), 1))
	for x, c := range want {
		f.SetRGBA(x, 0, c)
	}
	if err := e.Add(f, time.Second); err != nil {
		t.Fatal(err)
	}
	g := encodeDecode(t, &e)
	if n := len(g.Image[0].Palette); n > 256 || n < len(want) {
		t.Errorf("palette has %d entries for %d distinct colors", n, len(want))
	}
	for x, c := range want {
		if got := color.RGBAModel.Convert(g.Image[0].At(x, 0)); got != c {
			t.Errorf("pixel %d: %v, want %v", x, got, c)
		}
	}
}
