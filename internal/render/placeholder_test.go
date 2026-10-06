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

func virtualPNG(t *testing.T, r *Renderer, id uint32, placementID int, im image.Image, cols, rows int) {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	write(t, r, fmt.Sprintf("\x1b_Ga=T,t=d,f=100,U=1,c=%d,r=%d,i=%d,p=%d,q=2;%s\x1b\\", cols, rows, id, placementID, base64.StdEncoding.EncodeToString(b.Bytes())))
}

func placeholderText(row, col int) string {
	return string([]rune{kittyPlaceholder, rune(placeholderDiacritics[row]), rune(placeholderDiacritics[col])})
}

func TestVirtualPNGPlaceholderGrid(t *testing.T) {
	r := newRenderer(t)
	w, h := r.CellWidth, r.CellHeight
	im := image.NewNRGBA(image.Rect(0, 0, 2*w, 2*h))
	colors := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255}}
	for i, c := range colors {
		draw.Draw(im, image.Rect(i%2*w, i/2*h, (i%2+1)*w, (i/2+1)*h), image.NewUniform(c), image.Point{}, draw.Src)
	}
	virtualPNG(t, r, 43, 1, im, 2, 2)
	if len(r.placements) != 1 || !r.placements[0].meta.IsVirtual {
		t.Fatal("missing copied virtual placement")
	}
	blank := frame(t, r)
	// This is the exact placeholder/truecolor scheme emitted by ntcharts.
	write(t, r, "\x1b[38;2;0;0;43m"+placeholderText(0, 0)+placeholderText(0, 1)+"\r\n"+placeholderText(1, 0)+placeholderText(1, 1)+"\x1b[0m")
	f := frame(t, r)
	for i, c := range colors {
		pixel(t, f, i%2*w+w/2, i/2*h+h/2, c)
	}
	// Overwriting one cell punches out only that fragment.
	write(t, r, "\x1b[1;2H ")
	pixel(t, frame(t, r), w+w/2, h/2, blank.RGBAAt(0, 0))
	pixel(t, frame(t, r), w/2, h/2, colors[0])
	// Scrolling shifts the image with its cells, including clipping row zero.
	write(t, r, "\x1b[10;1H\r\n")
	pixel(t, frame(t, r), w/2, h/2, colors[2])
	write(t, r, "\x1b_Ga=d,d=I,i=43,q=2\x1b\\")
	if got := frame(t, r); !bytes.Equal(got.Pix, blank.Pix) {
		t.Fatal("deleted image left replacement glyphs or pixels")
	}
}

func TestPlaceholderIDsInheritanceAndUpdate(t *testing.T) {
	r := newRenderer(t)
	red := image.NewUniform(color.RGBA{255, 0, 0, 255})
	im := image.NewNRGBA(image.Rect(0, 0, 3*r.CellWidth, r.CellHeight))
	draw.Draw(im, im.Bounds(), red, image.Point{}, draw.Src)
	// Palette 43 means image ID 43, regardless of what RGB it resolves to.
	// The third diacritic supplies the high byte, and underline selects p=7.
	id := uint32(43 | 2<<24)
	virtualPNG(t, r, id, 7, im, 3, 1)
	write(t, r, fmt.Sprintf("\x1b_Ga=p,U=1,i=%d,p=8,c=1,r=1,q=2\x1b\\", id))
	write(t, r, "\x1b[38;5;43;58;2;0;0;7m"+placeholderText(0, 0)+string(rune(placeholderDiacritics[2]))+string(rune(kittyPlaceholder))+placeholderText(0, 2))
	f := frame(t, r)
	for x := 0; x < 3; x++ {
		pixel(t, f, x*r.CellWidth+r.CellWidth/2, r.CellHeight/2, color.RGBA{255, 0, 0, 255})
	}
	old := r.images[id].pixels
	draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{0, 255, 0, 255}), image.Point{}, draw.Src)
	virtualPNG(t, r, id, 7, im, 3, 1)
	pixel(t, frame(t, r), r.CellWidth/2, r.CellHeight/2, color.RGBA{0, 255, 0, 255})
	pixel(t, old, 0, 0, color.RGBA{255, 0, 0, 255})
	// A non-placeholder breaks inheritance; unresolved cells stay blank.
	write(t, r, "\x1b[2;1H "+string(rune(kittyPlaceholder)))
	pixel(t, frame(t, r), r.CellWidth+r.CellWidth/2, r.CellHeight+r.CellHeight/2, color.RGBA{24, 26, 32, 255})
}

func TestVirtualAspectFitAndAlternateScreen(t *testing.T) {
	r := newRenderer(t)
	im := image.NewNRGBA(image.Rect(0, 0, 40, 2))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
	virtualPNG(t, r, 5, 1, im, 2, 2)
	write(t, r, "\x1b[38;5;5m"+placeholderText(0, 0)+placeholderText(0, 1)+"\r\n"+placeholderText(1, 0)+placeholderText(1, 1))
	f := frame(t, r)
	pixel(t, f, 1, 1, color.RGBA{24, 26, 32, 255})
	pixel(t, f, r.CellWidth, r.CellHeight-1, color.RGBA{255, 0, 0, 255})
	write(t, r, "\x1b[?1049h\x1b[?25l")
	pixel(t, frame(t, r), r.CellWidth, r.CellHeight-1, color.RGBA{24, 26, 32, 255})
	write(t, r, "\x1b[?1049l")
	if got := frame(t, r); !bytes.Equal(f.Pix, got.Pix) {
		t.Fatal("alternate screen lost virtual placement")
	}
}

func TestSynchronizedOutputHold(t *testing.T) {
	r := newRenderer(t)
	write(t, r, "\x1b[?2026hhalf of an update")
	if !r.RenderHeld(time.Now()) {
		t.Fatal("partial synchronized update wasn't held")
	}
	if r.RenderHeld(time.Now().Add(2 * time.Second)) {
		t.Fatal("hold has no timeout")
	}
	write(t, r, "\x1b[?2026l")
	if r.RenderHeld(time.Now()) {
		t.Fatal("completed synchronized update is still held")
	}
}
