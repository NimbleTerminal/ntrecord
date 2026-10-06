package render

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type glyphKey struct {
	r            rune
	fg           color.RGBA
	bold, italic bool
}

type atlas struct {
	faces                 [4]font.Face
	glyphs                map[glyphKey]*image.RGBA
	width, height, ascent int
}

func newAtlas(size float64) (*atlas, error) {
	a := &atlas{glyphs: make(map[glyphKey]*image.RGBA)}
	for i, data := range [][]byte{gomono.TTF, gomonobold.TTF, gomonoitalic.TTF, gomonobolditalic.TTF} {
		parsed, err := sfnt.Parse(data)
		if err != nil {
			a.close()
			return nil, err
		}
		a.faces[i], err = opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			a.close()
			return nil, err
		}
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
	if len(a.glyphs) >= 8192 {
		clear(a.glyphs)
	}
	a.glyphs[k] = g
	return g
}
