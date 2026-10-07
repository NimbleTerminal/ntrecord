package render

import (
	"image"
	"math"

	vt "go.mitchellh.com/libghostty"
)

const kittyPlaceholder = 0x10eeee

// This decodes the semantic content of already-parsed libghostty cells, not
// PTY escape sequences. See https://sw.kovidgoyal.net/kitty/graphics-protocol/#unicode-placeholders.
type placeholderCell struct {
	low, placement uint32
	row, col, high int // -1 means omitted
}

type placeholderFragment struct {
	dest image.Rectangle // whole aspect-fitted image, anchored to this run
	clip image.Rectangle // only cells actually containing placeholders
}

func placeholderColor(c vt.StyleColor) uint32 {
	switch c.Tag {
	case vt.StyleColorPalette:
		return uint32(c.Palette) // index, NOT palette RGB
	case vt.StyleColorRGB:
		return uint32(c.RGB.R)<<16 | uint32(c.RGB.G)<<8 | uint32(c.RGB.B)
	default:
		return 0
	}
}

func decodePlaceholder(text []uint32, fg, underline vt.StyleColor) *placeholderCell {
	p := &placeholderCell{low: placeholderColor(fg), placement: placeholderColor(underline), row: -1, col: -1, high: -1}
	indices := []*int{&p.row, &p.col, &p.high}
	for i := 1; i < len(text) && i <= 3; i++ {
		if n, ok := placeholderDiacriticIndex[text[i]]; ok {
			*indices[i-1] = n
		}
	}
	if p.high > 255 {
		p.high = -1
	}
	return p
}

func (p *placeholderCell) inherit(previous *placeholderCell) {
	if previous != nil && p.low == previous.low && p.placement == previous.placement &&
		(p.row < 0 || p.row == previous.row) && (p.col < 0 || p.col == previous.col+1) {
		if p.row < 0 {
			p.row = previous.row
		}
		if p.col < 0 {
			p.col = previous.col + 1
		}
		if p.high < 0 {
			p.high = previous.high
		}
	}
	if p.row < 0 {
		p.row = 0
	}
	if p.col < 0 {
		p.col = 0
	}
	if p.high < 0 {
		p.high = 0
	}
}

// resolvePlaceholders keeps ordinary placements and materializes the visible
// pieces of virtual placements. Each piece is clipped to a horizontal run;
// erased/overwritten cells therefore leave holes, and scrolling needs no extra
// protocol state. All pixels, styles, and coordinates here are Go-owned.
func (r *Renderer) resolvePlaceholders(cells []cell) []placement {
	type key struct{ image, placement uint32 }
	targets := make(map[key]placement)
	r.frameImages = make(map[uint32]struct{})
	r.frameMissingImages = make(map[uint32]struct{})
	var out []placement
	for _, p := range r.placements {
		if !p.meta.IsVirtual {
			out = append(out, p)
			r.frameImages[p.meta.ImageID] = struct{}{}
			continue
		}
		targets[key{p.meta.ImageID, p.meta.PlacementID}] = p
		k := key{p.meta.ImageID, 0}
		// Placement ID zero lets the terminal select any virtual placement.
		if old, ok := targets[k]; !ok || p.meta.PlacementID < old.meta.PlacementID {
			targets[k] = p
		}
	}
	var previous *placeholderCell
	lastX, lastY := -1, -1
	for _, c := range cells {
		if c.placeholder == nil {
			previous = nil
			continue
		}
		if c.y != lastY || c.x != lastX+r.CellWidth {
			previous = nil
		}
		p := *c.placeholder
		p.inherit(previous)
		previous, lastX, lastY = &p, c.x, c.y
		id := p.low | uint32(p.high)<<24
		target, ok := targets[key{id, p.placement}]
		if !ok {
			r.frameMissingImages[id] = struct{}{}
			continue
		} // still suppress missing-image placeholder glyphs
		cols, rows := int(target.meta.Columns), int(target.meta.Rows)
		w, h := target.image.Bounds().Dx(), target.image.Bounds().Dy()
		if cols == 0 {
			cols = (w + r.CellWidth - 1) / r.CellWidth
		}
		if rows == 0 {
			rows = (h + r.CellHeight - 1) / r.CellHeight
		}
		if cols > 65535 || rows > 65535 || p.col >= cols || p.row >= rows {
			continue
		}
		gridW, gridH := cols*r.CellWidth, rows*r.CellHeight
		scale := math.Min(float64(gridW)/float64(w), float64(gridH)/float64(h))
		dw, dh := int(math.Round(float64(w)*scale)), int(math.Round(float64(h)*scale))
		x := c.x - p.col*r.CellWidth + (gridW-dw)/2
		y := c.y - p.row*r.CellHeight + (gridH-dh)/2
		f := placeholderFragment{dest: image.Rect(x, y, x+dw, y+dh), clip: image.Rect(c.x, c.y, c.x+r.CellWidth, c.y+r.CellHeight)}
		// Adjacent cells use a single clipped scale, avoiding resampling seams
		// and thousands of calls for a fullscreen placeholder grid.
		if len(out) > 0 {
			last := &out[len(out)-1]
			if last.fragment != nil && last.image == target.image && last.meta.PlacementID == target.meta.PlacementID &&
				last.fragment.dest == f.dest && last.fragment.clip.Min.Y == c.y && last.fragment.clip.Max.X == c.x {
				last.fragment.clip.Max.X += r.CellWidth
				continue
			}
		}
		r.frameImages[id] = struct{}{}
		target.fragment = &f
		out = append(out, target)
	}
	return out
}
