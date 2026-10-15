package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func newRenderer(t *testing.T) *Renderer {
	t.Helper()
	c := DefaultConfig()
	c.Cols = 20
	c.Rows = 10
	r, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	write(t, r, "\x1b[?25l")
	return r
}

func write(t *testing.T, r *Renderer, s string) {
	t.Helper()
	if err := r.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
}
func frame(t *testing.T, r *Renderer) *image.RGBA {
	t.Helper()
	f, err := r.Frame()
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func pixel(t *testing.T, f image.Image, x, y int, want color.RGBA) {
	t.Helper()
	if got := color.RGBAModel.Convert(f.At(x, y)); got != want {
		t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

func TestPNGFixture(t *testing.T) {
	r := newRenderer(t)
	b, err := os.ReadFile("testdata/kitty.apc")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Write(b); err != nil {
		t.Fatal(err)
	}
	if len(r.placements) != 1 {
		t.Fatalf("placements = %d", len(r.placements))
	}
	p := r.placements[0]
	x := int(p.geometry.ViewportCol)*r.CellWidth + 2
	y := int(p.geometry.ViewportRow)*r.CellHeight + 2
	pixel(t, frame(t, r), x, y, color.RGBA{240, 72, 88, 255})
	// Deleting terminal storage cannot invalidate a copied bitmap or geometry.
	old := p.image.Pix[0]
	write(t, r, "\x1b_Ga=d,d=A,q=2\x1b\\")
	if p.image.Pix[0] != old || len(r.placements) != 0 {
		t.Fatal("image ownership or delete failed")
	}
	pixel(t, frame(t, r), x, y, color.RGBA{24, 26, 32, 255})
}

func TestReplies(t *testing.T) {
	r := newRenderer(t)
	write(t, r, "\x1b[c\x1b_Ga=q,t=d,f=24,s=1,v=1,i=42;AAAA\x1b\\")
	reply := string(r.TakeReplies())
	if !strings.Contains(reply, "\x1b[?62;1;22c") || !strings.Contains(reply, "i=42;OK") {
		t.Fatalf("missing replies: %q", reply)
	}
}

func TestRawPlaceCropOffsetAndScroll(t *testing.T) {
	for _, format := range []int{24, 32} {
		t.Run(fmt.Sprint(format), func(t *testing.T) {
			r := newRenderer(t)
			data := []byte{255, 0, 0, 0, 255, 0}
			if format == 32 {
				data = []byte{255, 0, 0, 255, 0, 255, 0, 255}
			}
			write(t, r, fmt.Sprintf("\x1b_Ga=t,t=d,f=%d,s=2,v=1,i=7,q=2;%s\x1b\\", format, base64.StdEncoding.EncodeToString(data)))
			if len(r.placements) != 0 {
				t.Fatal("transmit-only displayed")
			}
			write(t, r, "\x1b[3;4H\x1b_Ga=p,i=7,p=1,x=1,y=0,w=1,h=1,c=2,r=2,X=2,Y=3,C=1,q=2\x1b\\")
			x, y := 3*r.CellWidth+2, 2*r.CellHeight+3
			pixel(t, frame(t, r), x, y, color.RGBA{0, 255, 0, 255})
			pixel(t, frame(t, r), x-1, y, color.RGBA{24, 26, 32, 255})
			// One scroll must move the placement with its grid anchor.
			write(t, r, "\x1b[10;1H\r\n")
			pixel(t, frame(t, r), x, y-r.CellHeight, color.RGBA{0, 255, 0, 255})
			write(t, r, "\x1b_Ga=d,d=i,i=7,q=2\x1b\\")
			if len(r.placements) != 0 {
				t.Fatal("delete did not remove placement")
			}
		})
	}
}

func TestZOrderAndText(t *testing.T) {
	for _, z := range []int{0, -1, -1073741825} {
		t.Run(fmt.Sprint(z), func(t *testing.T) {
			r := newRenderer(t)
			write(t, r, "\x1b[44;97mM\x1b[0m\x1b[H")
			text := frame(t, r)
			write(t, r, fmt.Sprintf("\x1b_Ga=T,t=d,f=24,s=1,v=1,c=1,r=1,C=1,i=1,z=%d,q=2;/wAA\x1b\\", z))
			f := frame(t, r)
			red, sameText := 0, 0
			for y := 0; y < r.CellHeight; y++ {
				for x := 0; x < r.CellWidth; x++ {
					if f.RGBAAt(x, y) == (color.RGBA{255, 0, 0, 255}) {
						red++
					}
					if text.RGBAAt(x, y).R > 150 && f.RGBAAt(x, y).G > 100 {
						sameText++
					}
				}
			}
			switch z {
			case 0:
				if red != r.CellWidth*r.CellHeight {
					t.Fatal("positive z is not above text")
				}
			case -1:
				if red == 0 || sameText == 0 {
					t.Fatal("negative z lost background image or foreground glyph")
				}
			default:
				if !bytes.Equal(text.Pix, f.Pix) {
					t.Fatal("image below explicit cell background is visible")
				}
			}
		})
	}
}

func TestNativePixelsAndAlpha(t *testing.T) {
	r := newRenderer(t)
	im := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i := 0; i < len(im.Pix); i += 4 {
		copy(im.Pix[i:], []byte{255, 0, 0, 128})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	write(t, r, "\x1b_Ga=T,t=d,f=100,q=2;"+base64.StdEncoding.EncodeToString(b.Bytes())+"\x1b\\")
	f := frame(t, r)
	if f.RGBAAt(0, 0).R < 130 || f.RGBAAt(0, 0).R > 145 {
		t.Fatal("PNG alpha not composited")
	}
	pixel(t, f, 3, 0, color.RGBA{24, 26, 32, 255})
}

func TestFrameCacheInvalidation(t *testing.T) {
	r := newRenderer(t)
	first := frame(t, r)
	if got, err := r.FrameAt(time.Now().Add(time.Second)); err != nil || got != first {
		t.Fatal("idle viewport was redrawn", err)
	}
	original := append([]byte(nil), first.Pix...)
	write(t, r, "hello\r\n")
	second := frame(t, r)
	if second == first || bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("text did not invalidate frame")
	}
	if !bytes.Equal(original, first.Pix) {
		t.Fatal("previous frame was mutated")
	}
	write(t, r, strings.Repeat("scroll\r\n", 20))
	bottom := frame(t, r)
	if err := r.Scroll(-2); err != nil {
		t.Fatal(err)
	}
	if frame(t, r) == bottom {
		t.Fatal("scroll did not invalidate frame")
	}
	if n := testing.AllocsPerRun(100, func() { _, _ = r.Frame() }); n != 0 {
		t.Fatalf("idle frame allocations: %v", n)
	}
}

func TestImageCopiesReleasedOnReset(t *testing.T) {
	r := newRenderer(t)
	b, err := os.ReadFile("testdata/kitty.apc")
	if err != nil {
		t.Fatal(err)
	}
	write(t, r, string(b))
	if len(r.images) == 0 {
		t.Fatal("fixture not cached")
	}
	withImage := frame(t, r)
	write(t, r, "\x1bc")
	if len(r.images) != 0 || len(r.placements) != 0 {
		t.Fatal("reset retained image copies")
	}
	if frame(t, r) == withImage {
		t.Fatal("reset retained frame")
	}
}

func TestUnderlineStaysInItsCell(t *testing.T) {
	// The underline sits below the baseline, which at small sizes and short
	// line heights is the cell's last row or past it; it must not paint the
	// row beneath.
	for _, tc := range []struct{ size, lineHeight float64 }{{6, 1}, {6.5, 1}, {7.5, 1}, {8.5, 1}, {12, 0.5}, {18, 0.8}, {18, 1}} {
		c := DefaultConfig()
		c.Cols, c.Rows, c.FontSize, c.LineHeight = 4, 3, tc.size, tc.lineHeight
		paint := func(s string) *image.RGBA {
			r, err := New(c)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			write(t, r, "\x1b[?25l"+s)
			return frame(t, r)
		}
		plain, underlined := paint("    "), paint("\x1b[4m    \x1b[0m")
		lo, hi := -1, -1
		for y := plain.Bounds().Min.Y; y < plain.Bounds().Max.Y; y++ {
			for x := plain.Bounds().Min.X; x < plain.Bounds().Max.X; x++ {
				if plain.RGBAAt(x, y) != underlined.RGBAAt(x, y) {
					if lo < 0 {
						lo = y
					}
					hi = y
				}
			}
		}
		cellHeight := int(math.Ceil(float64(plain.Bounds().Dy()) / 3))
		if lo < 0 || hi >= cellHeight {
			t.Errorf("size %v line height %v: underline on rows %d..%d, cell is %d high", tc.size, tc.lineHeight, lo, hi, cellHeight)
		}
	}
}
