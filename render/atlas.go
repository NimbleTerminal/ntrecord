package render

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/math/fixed"
)

type glyphKey struct {
	r            rune
	fg           color.RGBA
	bold, italic bool
}

type atlas struct {
	glyphBytes            int
	sources               [4]*fontSource
	size                  float64
	clusters              map[clusterKey]*image.RGBA
	clusterBytes          int
	shaped                [4]*fallbackFont
	symbols, emoji        *fallbackFont
	fallbacks             fallbackCatalog
	faces                 [4]*face
	glyphs                map[glyphKey]*image.RGBA
	width, height, ascent int
}

func newAtlasFamily(size float64, family string) (*atlas, error) {
	a := &atlas{glyphs: make(map[glyphKey]*image.RGBA), clusters: make(map[clusterKey]*image.RGBA), size: size}
	fonts, err := fontFaces(family)
	if err != nil {
		return nil, err
	}
	a.sources = fonts
	for i, parsed := range fonts {
		a.faces[i] = newFace(parsed.font, size)
	}
	advance, _ := a.faces[0].GlyphAdvance('M')
	a.width = advance.Ceil()
	m := a.faces[0].Metrics()
	a.height, a.ascent = m.Height.Ceil(), m.Ascent.Ceil()
	return a, nil
}

func (a *atlas) close() {
	for _, f := range a.faces {
		if f != nil {
			_ = f.Close()
		}
	}
}

func (a *atlas) glyph(k glyphKey) *image.RGBA {
	if g := a.glyphs[k]; g != nil {
		return g
	}
	i := 0
	if k.bold {
		i++
	}
	if k.italic {
		i += 2
	}
	f := a.faces[i]
	// Copy the face's reusable mask immediately. Keep the glyph bounds so
	// italic overhangs and combining marks aren't clipped to one cell.
	r, mask, mp, _, ok := f.Glyph(fixed.P(0, a.ascent), k.r)
	if !ok {
		r, mask, mp, _, _ = f.Glyph(fixed.P(0, a.ascent), '\uFFFD')
	}
	g := image.NewRGBA(r)
	if mask != nil {
		draw.DrawMask(g, r, image.NewUniform(k.fg), image.Point{}, mask, mp, draw.Src)
	}
	// Bound cache growth for long live sessions with arbitrary true colors.
	if len(a.glyphs) >= 8192 || a.glyphBytes+len(g.Pix) > maxAtlasBytes {
		clear(a.glyphs)
		a.glyphBytes = 0
	}
	if len(g.Pix) <= maxAtlasBytes {
		a.glyphs[k] = g
		a.glyphBytes += len(g.Pix)
	}
	return g
}
