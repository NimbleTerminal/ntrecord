package gifencode

import (
	"compress/lzw"
	"image"
	"image/color"
	"image/gif"
	"math/bits"
)

// Keep full frames until after Rotate. Every loop starts with an opaque full
// canvas; later patches overwrite changed pixels and retain everything else.
// Disappearing content is a change too, including pixels reverting to black.
func optimize(full gif.GIF) gif.GIF {
	g := gif.GIF{Config: full.Config, LoopCount: 0, Delay: full.Delay}
	g.Image = make([]*image.Paletted, len(full.Image))
	g.Disposal = make([]byte, len(g.Image))
	for i, current := range full.Image {
		g.Disposal[i] = gif.DisposalNone
		if i == 0 {
			g.Image[i] = current
			continue
		}
		previous := full.Image[i-1]
		r := changedBounds(previous, current)
		if r.Empty() {
			r = image.Rect(0, 0, 1, 1)
		} // A delay overflow still needs a frame.
		opaque := current.SubImage(r).(*image.Paletted)
		g.Image[i] = opaque
		var used [256]bool
		unchanged := 0
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				a, b := previous.ColorIndexAt(x, y), current.ColorIndexAt(x, y)
				if a == b {
					unchanged++
				} else {
					used[b] = true
				}
			}
		}
		// Scattered changes benefit most; avoid allocating for dense replacements.
		if unchanged < r.Dx()*r.Dy()/4 || unchanged == 0 {
			continue
		}
		transparent := -1
		for j, v := range used[:len(current.Palette)] {
			if !v {
				transparent = j
				break
			}
		}
		if transparent < 0 && len(current.Palette) < 256 {
			transparent = len(current.Palette)
		}
		if transparent < 0 {
			continue
		}
		pal := append(color.Palette(nil), current.Palette...)
		if transparent == len(pal) {
			pal = append(pal, color.RGBA{})
		} else {
			pal[transparent] = color.RGBA{}
		}
		patch := image.NewPaletted(r, pal)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				v := current.ColorIndexAt(x, y)
				if v == previous.ColorIndexAt(x, y) {
					v = uint8(transparent)
				}
				patch.SetColorIndex(x, y, v)
			}
		}
		// Transparency can fragment an otherwise compressible fill. Compare actual
		// LZW payloads before choosing it; both paths have the same GIF headers.
		tableCost := 0
		if bits.Len(uint(len(pal)-1)) > bits.Len(uint(len(current.Palette)-1)) {
			tableCost = 3 * (1 << bits.Len(uint(len(pal)-1)))
		}
		if compressedSize(patch)+tableCost < compressedSize(opaque) {
			g.Image[i] = patch
		}
	}
	return g
}

func changedBounds(a, b *image.Paletted) image.Rectangle {
	r := image.Rect(b.Rect.Max.X, b.Rect.Max.Y, 0, 0)
	for y := 0; y < b.Rect.Dy(); y++ {
		for x := 0; x < b.Rect.Dx(); x++ {
			if a.Pix[y*a.Stride+x] != b.Pix[y*b.Stride+x] {
				r.Min.X = min(r.Min.X, x)
				r.Min.Y = min(r.Min.Y, y)
				r.Max.X = max(r.Max.X, x+1)
				r.Max.Y = y + 1
			}
		}
	}
	return r
}

type byteCounter int

func (n *byteCounter) Write(p []byte) (int, error) { *n += byteCounter(len(p)); return len(p), nil }
func compressedSize(p *image.Paletted) int {
	var n byteCounter
	w := lzw.NewWriter(&n, lzw.LSB, max(2, bits.Len(uint(len(p.Palette)-1))))
	for y := 0; y < p.Rect.Dy(); y++ {
		_, _ = w.Write(p.Pix[y*p.Stride : y*p.Stride+p.Rect.Dx()])
	}
	_ = w.Close()
	return int(n)
}

// Remove unused global entries once, not separately per frame. Small terminal
// palettes can then use shorter LZW codes without introducing local tables or
// changing the meaning of an index between frames. Quantized frames are owned
// by this Encode invocation, so shared holds can be remapped in place once.
func compactPalette(g *gif.GIF) {
	var used [256]bool
	unique := make(map[*image.Paletted]bool, len(g.Image))
	for _, p := range g.Image {
		if unique[p] {
			continue
		}
		unique[p] = true
		for _, v := range p.Pix {
			used[v] = true
		}
	}
	var remap [256]byte
	pal := make(color.Palette, 0, 256)
	original := g.Config.ColorModel.(color.Palette)
	for i, v := range used {
		if v {
			remap[i] = byte(len(pal))
			pal = append(pal, original[i])
		}
	}
	if len(pal) == 256 {
		return
	}
	for p := range unique {
		for i, v := range p.Pix {
			p.Pix[i] = remap[v]
		}
		p.Palette = pal
	}
	g.Config.ColorModel = pal
}
