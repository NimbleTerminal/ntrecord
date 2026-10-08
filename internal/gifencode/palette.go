package gifencode

import (
	"image"
	"image/color"
	"slices"
)

// A bounded 5-bit RGB histogram sampled evenly (at most 65536 pixels per
// unique frame). All arithmetic and tie breaks are deterministic across CPUs.
// Flat horizontal runs also nominate exact colors; unlike bin averages these
// preserve a solid application/Kitty background not listed in the theme.
type colorBin struct {
	r, g, b, n uint64
	flat       uint32
	votes      uint64
}
type colorHistogram [32768]colorBin

func binIndex(r, g, b uint8) int { return int(r>>3)<<10 | int(g>>3)<<5 | int(b>>3) }
func (h *colorHistogram) observe(f *image.RGBA) {
	pixels := f.Rect.Dx() * f.Rect.Dy()
	samples := min(pixels, 65536)
	for i := 0; i < samples; i++ {
		p := i * pixels / samples
		k := p/f.Rect.Dx()*f.Stride + (p%f.Rect.Dx())*4
		r, g, b := f.Pix[k], f.Pix[k+1], f.Pix[k+2]
		v := &h[binIndex(r, g, b)]
		v.r += uint64(r)
		v.g += uint64(g)
		v.b += uint64(b)
		v.n++
	}
	for y := 0; y < f.Rect.Dy(); y++ {
		for x := 0; x < f.Rect.Dx(); {
			k := y*f.Stride + x*4
			r, g, b := f.Pix[k], f.Pix[k+1], f.Pix[k+2]
			end := x + 1
			for end < f.Rect.Dx() {
				j := y*f.Stride + end*4
				if f.Pix[j] != r || f.Pix[j+1] != g || f.Pix[j+2] != b {
					break
				}
				end++
			}
			if end-x >= 8 {
				c := uint32(r)<<16 | uint32(g)<<8 | uint32(b)
				v := &h[binIndex(r, g, b)]
				n := uint64(end - x)
				// Weighted majority candidate per bin; never an unbounded exact-color map.
				if v.flat == c {
					v.votes += n
				} else if n > v.votes {
					v.flat = c
					v.votes = n - v.votes
				} else {
					v.votes -= n
				}
			}
			x = end
		}
	}
}

type paletteEntry struct {
	rgb [3]uint8
	n   uint64
}

func (h *colorHistogram) palette(seed color.Palette) color.Palette {
	colors := make(color.Palette, 0, 256)
	locked := make(map[uint32]bool, 256)
	add := func(c color.RGBA) {
		key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
		if !locked[key] && len(colors) < 256 {
			colors = append(colors, c)
			locked[key] = true
		}
	}
	add(color.RGBA{0, 0, 0, 255})
	add(color.RGBA{255, 255, 255, 255})
	if len(seed) == 0 {
		seed = defaultSeed
	}
	for _, c := range seed {
		add(rgba8(c))
	}
	// Keep at most 32 additional flat colors. Theme seeds always take priority.
	type flat struct {
		rgb   uint32
		votes uint64
	}
	flats := make([]flat, 0, 256)
	for _, b := range h {
		if b.votes > 0 {
			flats = append(flats, flat{b.flat, b.votes})
		}
	}
	slices.SortFunc(flats, func(a, b flat) int {
		if a.votes > b.votes {
			return -1
		}
		if a.votes < b.votes {
			return 1
		}
		return int(a.rgb) - int(b.rgb)
	})
	added := 0
	for _, f := range flats {
		if added == 32 {
			break
		}
		if !locked[f.rgb] {
			add(color.RGBA{uint8(f.rgb >> 16), uint8(f.rgb >> 8), uint8(f.rgb), 255})
			added++
		}
	}
	entries := make([]paletteEntry, 0, len(h))
	for _, b := range h {
		if b.n > 0 {
			v := [3]uint8{uint8(b.r / b.n), uint8(b.g / b.n), uint8(b.b / b.n)}
			key := uint32(v[0])<<16 | uint32(v[1])<<8 | uint32(v[2])
			if !locked[key] {
				entries = append(entries, paletteEntry{v, b.n})
			}
		}
	}
	if len(entries) > 0 && len(colors) < 256 {
		boxes := [][]paletteEntry{entries}
		for len(boxes) < 256-len(colors) {
			best := -1
			var score uint64
			for i, b := range boxes {
				_, s := splitAxis(b)
				if len(b) > 1 && (best < 0 || s > score) {
					best, score = i, s
				}
			}
			if best < 0 {
				break
			}
			box := boxes[best]
			axis, _ := splitAxis(box)
			slices.SortFunc(box, func(a, b paletteEntry) int {
				for j := range 3 {
					k := (axis + j) % 3
					if a.rgb[k] != b.rgb[k] {
						return int(a.rgb[k]) - int(b.rgb[k])
					}
				}
				return 0
			})
			var total, sum uint64
			for _, v := range box {
				total += v.n
			}
			cut := 1
			for cut < len(box)-1 {
				sum += box[cut-1].n
				if sum >= total/2 {
					break
				}
				cut++
			}
			boxes[best] = box[:cut]
			boxes = append(boxes, box[cut:])
		}
		for _, box := range boxes {
			var sums [3]uint64
			var n uint64
			for _, e := range box {
				n += e.n
				for j := range 3 {
					sums[j] += uint64(e.rgb[j]) * e.n
				}
			}
			add(color.RGBA{uint8(sums[0] / n), uint8(sums[1] / n), uint8(sums[2] / n), 255})
		}
	}
	// A full table keeps the GIF LZW width predictable. Duplicate padding does
	// not introduce new choices or displace a protected theme color.
	for len(colors) < 256 {
		colors = append(colors, colors[0])
	}
	return colors
}
func splitAxis(box []paletteEntry) (int, uint64) {
	lo, hi := [3]uint8{255, 255, 255}, [3]uint8{}
	var n uint64
	for _, e := range box {
		n += e.n
		for j := range 3 {
			lo[j] = min(lo[j], e.rgb[j])
			hi[j] = max(hi[j], e.rgb[j])
		}
	}
	a := 0
	for j := 1; j < 3; j++ {
		if hi[j]-lo[j] > hi[a]-lo[a] {
			a = j
		}
	}
	return a, uint64(hi[a]-lo[a]) * n
}
