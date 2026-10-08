package codecs_test

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"io"
	"testing"
	"time"

	"nimbleterminal.dev/ntrecord/asciicast"
	"nimbleterminal.dev/ntrecord/gifencode"
)

func TestCastStreams(t *testing.T) {
	var data bytes.Buffer
	w, err := asciicast.NewWriter(&data, asciicast.Terminal{Cols: 20, Rows: 4})
	if err != nil {
		t.Fatal(err)
	}
	emoji := []byte("👍🏽")
	if err = w.Output(0, emoji[:2]); err != nil {
		t.Fatal(err)
	}
	if err = w.Output(time.Second, emoji[2:]); err != nil {
		t.Fatal(err)
	}
	if err = w.End(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	r, err := asciicast.NewReader(&data)
	if err != nil {
		t.Fatal(err)
	}
	if c, h := r.Header.Size(); c != 20 || h != 4 {
		t.Fatal("lost grid")
	}
	e, err := r.Next()
	if err != nil || e.Data != "👍🏽" || e.At != time.Second {
		t.Fatalf("%+v %v", e, err)
	}
	e, err = r.Next()
	if err != nil || e.At != 2*time.Second || e.Code != "m" {
		t.Fatalf("%+v %v", e, err)
	}
	if _, err = r.Next(); err != io.EOF {
		t.Fatal(err)
	}
}

func TestGIFOwnsFrames(t *testing.T) {
	var enc gifencode.Encoder
	frame := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			frame.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	if err := enc.Add(frame, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			frame.SetRGBA(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	if err := enc.Add(frame, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	clear(frame.Pix) // Add must already have copied both input images.
	var data bytes.Buffer
	if err := enc.Encode(&data); err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(&data)
	if err != nil {
		t.Fatal(err)
	}
	if g.LoopCount != 0 || len(g.Image) != 2 || g.Delay[0] != 10 || g.Delay[1] != 20 {
		t.Fatal("wrong GIF timeline")
	}
	if got := color.RGBAModel.Convert(g.Image[0].At(0, 0)); got != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("input mutation corrupted GIF: %v", got)
	}
}
