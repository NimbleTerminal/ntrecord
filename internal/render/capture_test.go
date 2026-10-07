package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"
	"time"
)

func replacementPNG(t *testing.T, c color.RGBA) (string, string) {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 22, 21))
	draw.Draw(im, im.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	data := base64.StdEncoding.EncodeToString(b.Bytes())
	split := len(data) / 8 * 4
	return fmt.Sprintf("\x1b_Gf=100,q=2,i=43,U=1,c=2,r=1,a=T,m=1;%s\x1b\\", data[:split]), fmt.Sprintf("\x1b_Gq=2,m=0;%s\x1b\\", data[split:])
}

func capture(t *testing.T, r *Renderer, now time.Time) *image.RGBA {
	t.Helper()
	f, err := r.CaptureFrameAt(now)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func seedCapturedImage(t *testing.T, r *Renderer, now time.Time) *image.RGBA {
	t.Helper()
	first, last := replacementPNG(t, color.RGBA{255, 0, 0, 255})
	write(t, r, first+last+"\x1b[38;2;0;0;43m"+placeholderText(0, 0)+placeholderText(0, 1)+"\x1b[0m")
	return capture(t, r, now)
}

func TestCaptureChunkedImageReplacement(t *testing.T) {
	r := newRenderer(t)
	now := time.Now()
	old := seedCapturedImage(t, r, now)
	pixel(t, old, 5, 5, color.RGBA{255, 0, 0, 255})
	first, last := replacementPNG(t, color.RGBA{0, 255, 0, 255})
	write(t, r, first)
	// Reproduce the native state seen during the ntcharts shader capture:
	// the old placement is gone, but its placeholder cells remain on screen.
	if len(r.placements) != 0 {
		t.Fatal("fixture did not remove the old placement")
	}
	pixel(t, frame(t, r), 5, 5, color.RGBA{24, 26, 32, 255})
	if got := capture(t, r, now.Add(50*time.Millisecond)); got != old {
		t.Fatal("captured a blank between PNG chunks")
	}
	// A PTY read may also split inside the final APC, not only between APCs.
	write(t, r, last[:len(last)/2])
	if got := capture(t, r, now.Add(100*time.Millisecond)); got != old {
		t.Fatal("partial APC released previous frame")
	}
	write(t, r, last[len(last)/2:])
	next := capture(t, r, now.Add(150*time.Millisecond))
	pixel(t, next, 5, 5, color.RGBA{0, 255, 0, 255})
	pixel(t, old, 5, 5, color.RGBA{255, 0, 0, 255})
}

func TestCaptureImageGapExpires(t *testing.T) {
	for _, deletion := range []bool{false, true} {
		t.Run(fmt.Sprint("delete=", deletion), func(t *testing.T) {
			r := newRenderer(t)
			now := time.Now()
			old := seedCapturedImage(t, r, now)
			first, _ := replacementPNG(t, color.RGBA{0, 255, 0, 255})
			if deletion {
				first = "\x1b_Ga=d,d=I,i=43,q=2\x1b\\"
			}
			write(t, r, first)
			if capture(t, r, now) != old {
				t.Fatal("image gap wasn't held")
			}
			expired := capture(t, r, now.Add(imageReplacementGrace))
			pixel(t, expired, 5, 5, color.RGBA{24, 26, 32, 255})
			if capture(t, r, now.Add(2*imageReplacementGrace)) != expired {
				t.Fatal("gap restarted after timeout")
			}
		})
	}
}

func TestCaptureClearIsImmediate(t *testing.T) {
	r := newRenderer(t)
	now := time.Now()
	seedCapturedImage(t, r, now)
	first, _ := replacementPNG(t, color.RGBA{0, 255, 0, 255})
	write(t, r, first)
	capture(t, r, now)
	write(t, r, "\x1b[2J\x1b[H")
	pixel(t, capture(t, r, now.Add(time.Millisecond)), 5, 5, color.RGBA{24, 26, 32, 255})
}
