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
