package render

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"unicode/utf8"

	ft "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/harfbuzz"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/unicode/norm"
)

type clusterKey struct {
	text         string
	fg           color.RGBA
	bold, italic bool
	span, w, h   int
}

const maxClusterRunes = 256
const maxAtlasBytes = 32 << 20

func (a *atlas) cluster(k clusterKey) *image.RGBA {
	if g, ok := a.clusters[k]; ok {
		return g
	}
	if utf8.RuneCountInString(k.text) > maxClusterRunes {
		k.text = "\ufffd"
	}
	text := []rune(norm.NFC.String(k.text))
	if len(text) == 0 {
		return nil
	}
	var result *image.RGBA
	if len(text) == 1 {
		result = cellGraphic(text[0], k.w, k.h, k.fg)
	}
	style := 0
	if k.bold {
		style++
	}
	if k.italic {
		style += 2
	}
	if result == nil && len(text) == 1 && k.span == 1 {
		if _, ok := a.faces[style].GlyphAdvance(text[0]); ok {
			result = a.glyph(glyphKey{text[0], k.fg, k.bold, k.italic})
		}
	}
	if result == nil {
		f := a.chooseFont(text, style, k.span)
		if f != nil {
			result = a.shapeCluster(f, text, k)
		}
	}
	if result == nil {
		result = a.glyph(glyphKey{'\ufffd', k.fg, k.bold, k.italic})
	}
	if len(a.clusters) >= 4096 || a.clusterBytes+len(result.Pix) > maxAtlasBytes {
		clear(a.clusters)
		a.clusterBytes = 0
	}
	if len(result.Pix) <= maxAtlasBytes {
		a.clusters[k] = result
		a.clusterBytes += len(result.Pix)
	}
	return result
}
func (a *atlas) chooseFont(text []rune, style, span int) *fallbackFont {
	emoji := span == 2
	for _, r := range text {
		if r == 0xfe0f || r == 0x20e3 {
			emoji = true
		}
	}
	for _, r := range text {
		if r == 0xfe0e {
			emoji = false
		}
	}
	if emoji {
		if a.emoji == nil {
			a.emoji = loadFallback(emojiFont, 0, a.size, "Noto Color Emoji")
			if a.emoji != nil {
				a.emoji.emoji = true
			}
		}
		if a.emoji != nil && covers(a.emoji.face.Cmap, text) {
			return a.emoji
		}
	}
	if a.shaped[style] == nil {
		src := a.sources[style]
		a.shaped[style] = loadFallback(src.data, src.index, a.size, "primary")
	}
	if f := a.shaped[style]; f != nil && covers(f.face.Cmap, text) {
		return f
	}
	if a.symbols == nil {
		a.symbols = loadFallback(symbolsFont, 0, a.size, "Noto Sans Symbols 2")
	}
	if a.symbols != nil && covers(a.symbols.face.Cmap, text) {
		return a.symbols
	}
	if f := a.fallbacks.resolve(text, a.size, style&1 != 0, style&2 != 0); f != nil {
		return f
	}
	return nil
}

func (a *atlas) shapeCluster(f *fallbackFont, text []rune, k clusterKey) *image.RGBA {
	if f.shaper == nil {
		f.shaper = harfbuzz.NewFont(f.face)
		f.shaper.XScale = int32(a.size*64 + 0.5)
		f.shaper.YScale = f.shaper.XScale
	}
	buf := harfbuzz.NewBuffer()
	buf.AddRunes(text, 0, len(text))
	buf.GuessSegmentProperties()
	buf.Shape(f.shaper, nil)
	// libghostty owns terminal width; shaping must not move neighboring cells.
	bounds := image.Rect(0, 0, k.w*k.span, k.h)
	result := image.NewRGBA(bounds)
	pen := fixed.Int26_6(0)
	for i, g := range buf.Info {
		if g.Glyph == 0 {
			return nil
		}
		if g.Glyph == ft.EmptyGlyph {
			continue
		}
		if f.emoji {
			bitmap, ok := f.face.GlyphDataBitmap(g.Glyph)
			if !ok || bitmap.Format != ft.PNG {
				return nil
			}
			config, err := png.DecodeConfig(bytes.NewReader(bitmap.Data))
			if err != nil || config.Width > 2048 || config.Height > 2048 {
				return nil
			}
			im, err := png.Decode(bytes.NewReader(bitmap.Data))
			if err != nil {
				return nil
			}
			// Emoji bitmap strikes contain their own baseline padding. Fit the whole
			// strike inside the cluster's cells without stretching its aspect ratio.
			w, h := bounds.Dx(), bounds.Dy()
			if w*config.Height > h*config.Width {
				w = h * config.Width / config.Height
			} else {
				h = w * config.Height / config.Width
			}
			if w < 1 || h < 1 {
				return nil
			}
			target := image.Rect((bounds.Dx()-w)/2, (bounds.Dy()-h)/2, (bounds.Dx()+w)/2, (bounds.Dy()+h)/2)
			xdraw.CatmullRom.Scale(result, target, im, im.Bounds(), draw.Over, nil)
			// A valid supported emoji sequence produces one visible bitmap. Never
			// stack multiple component emoji on top of each other when GSUB lacks it.
			if len(buf.Info) > 1 {
				return nil
			}
			continue
		}
		if f.outline == nil {
			return nil
		}
		pos := buf.Pos[i]
		dot := fixed.Point26_6{X: pen + fixed.Int26_6(pos.XOffset), Y: fixed.I(a.ascent) - fixed.Int26_6(pos.YOffset)}
		rect, mask, mp, _, ok := f.outline.GlyphIndex(dot, sfnt.GlyphIndex(g.Glyph))
		if !ok {
			return nil
		}
		if mask != nil {
			draw.DrawMask(result, rect, image.NewUniform(k.fg), image.Point{}, mask, mp, draw.Over)
		}
		pen += fixed.Int26_6(pos.XAdvance)
	}
	return result
}
