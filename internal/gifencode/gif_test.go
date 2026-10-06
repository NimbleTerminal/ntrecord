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
