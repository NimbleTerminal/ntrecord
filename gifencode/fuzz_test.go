package gifencode

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"
)

// FuzzEncode builds small RGBA frame sequences from fuzz bytes, encodes them
// with adaptive or fixed palettes and optional loop rotation, and checks that
// standard image/gif decodes a valid animation with matching dimensions and timeline.
func FuzzEncode(f *testing.F) {
	f.Add(uint8(4), uint8(4), false, uint8(0), []byte{10, 255, 0, 0, 20, 0, 255, 0, 30, 0, 0, 255})
	f.Add(uint8(8), uint8(6), true, uint8(128), []byte{5, 24, 26, 32, 15, 224, 228, 235, 25, 24, 26, 32})
	f.Add(uint8(1), uint8(1), false, uint8(64), []byte{1, 0, 0, 0})

	f.Fuzz(func(t *testing.T, w, h uint8, fixed bool, rotate uint8, raw []byte) {
		if len(raw) == 0 || len(raw) > 4096 {
			return
		}
		width := int(w%16) + 1
		height := int(h%16) + 1
		e := Encoder{
			FixedPalette: fixed,
			Seed:         color.Palette{color.RGBA{24, 26, 32, 255}, color.RGBA{224, 228, 235, 255}},
		}

		frame := image.NewRGBA(image.Rect(0, 0, width, height))
		stride := 1 + width*height*3
		frames := min(4, max(1, len(raw)/4))
		pos := 0
		for i := 0; i < frames; i++ {
			dur := time.Duration(raw[pos%len(raw)]%20+1) * 10 * time.Millisecond
			pos++
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					r := raw[pos%len(raw)]
					g := raw[(pos+1)%len(raw)]
					b := raw[(pos+2)%len(raw)]
					pos += 3
					frame.SetRGBA(x, y, color.RGBA{r, g, b, 255})
				}
			}
			if err := e.Add(frame, dur); err != nil {
				t.Fatalf("Add frame %d: %v", i, err)
			}
			if pos >= stride*frames {
				break
			}
		}

		if rotate > 0 {
			e.Rotate(float64(rotate) / 256.0)
		}
		if e.Duration() <= 0 {
			return
		}

		var buf bytes.Buffer
		if err := e.Encode(&buf); err != nil {
			t.Fatalf("Encode: %v", err)
		}
		decoded, err := gif.DecodeAll(&buf)
		if err != nil {
			t.Fatalf("gif.DecodeAll: %v", err)
		}
		if decoded.Config.Width != width || decoded.Config.Height != height {
			t.Fatalf("decoded size %dx%d, want %dx%d", decoded.Config.Width, decoded.Config.Height, width, height)
		}
		if len(decoded.Image) == 0 || len(decoded.Image) != len(decoded.Delay) {
			t.Fatalf("decoded %d images and %d delays", len(decoded.Image), len(decoded.Delay))
		}
		bounds := image.Rect(0, 0, width, height)
		totalDelay := 0
		for i, im := range decoded.Image {
			if !im.Bounds().In(bounds) {
				t.Fatalf("frame %d bounds %v outside %v", i, im.Bounds(), bounds)
			}
			totalDelay += decoded.Delay[i]
		}
		wantTicks := int(e.Duration() / (10 * time.Millisecond))
		if totalDelay != wantTicks {
			t.Fatalf("decoded total delay %d, want %d", totalDelay, wantTicks)
		}
	})
}
