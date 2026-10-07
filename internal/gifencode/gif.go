// Package gifencode quantizes frames and merges identical adjacent bitmaps.
package gifencode

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io"
	"time"
)

type Encoder struct {
	// Seed lists colors (theme background, foreground, ANSI palette) that must
	// be represented exactly rather than approximated by the fixed palette.
	Seed color.Palette

	gif      gif.GIF
	palette  color.Palette // built from Seed on first use
	previous []byte
	elapsed  time.Duration
	ticks    int
	bytes    int64
}

func (e *Encoder) Add(frame *image.RGBA, duration time.Duration) error {
	e.elapsed += duration
	total := int((e.elapsed + 5*time.Millisecond) / (10 * time.Millisecond))
	delay := total - e.ticks
	if delay <= 0 {
		return nil
	}
	e.ticks += delay
	if len(e.gif.Image) > 0 && bytes.Equal(e.previous, frame.Pix) {
		i := len(e.gif.Delay) - 1
		room := 65535 - e.gif.Delay[i]
		n := min(room, delay)
		e.gif.Delay[i] += n
		delay -= n
		for delay > 0 {
			n = min(delay, 65535)
			e.gif.Image = append(e.gif.Image, e.gif.Image[i])
			e.gif.Delay = append(e.gif.Delay, n)
			delay -= n
		}
		return nil
	}
	if e.bytes+int64(frame.Rect.Dx()*frame.Rect.Dy()) > 512*1024*1024 {
		return fmt.Errorf("recording exceeds 512 MiB of unique GIF frames; use a smaller viewport or shorter session")
	}
	if e.palette == nil {
		e.palette = buildPalette(e.Seed)
	}
	p := image.NewPaletted(frame.Bounds(), e.palette)
	draw.FloydSteinberg.Draw(p, p.Bounds(), frame, frame.Bounds().Min)
	e.bytes += int64(len(p.Pix))
	e.previous = append(e.previous[:0], frame.Pix...)
	for delay > 0 {
		n := min(delay, 65535)
		e.gif.Image = append(e.gif.Image, p)
		e.gif.Delay = append(e.gif.Delay, n)
		delay -= n
	}
	return nil
}

// defaultSeed is sigmamax's built-in terminal background and foreground.
var defaultSeed = color.Palette{color.RGBA{24, 26, 32, 255}, color.RGBA{224, 228, 235, 255}}

// buildPalette returns the 256-color Plan9 palette with each seed color made
// exact: a seed missing from the palette replaces the nearest entry that is
// not pure black, pure white, or an earlier seed. Earlier seeds win, so list
// the background and foreground first. Exact entries dither to flat regions.
func buildPalette(seed color.Palette) color.Palette {
	if len(seed) == 0 {
		seed = defaultSeed
	}
	colors := append(color.Palette(nil), palette.Plan9...)
	locked := make([]bool, len(colors))
	locked[0], locked[len(colors)-1] = true, true // Plan9 black and white
	for _, c := range seed {
		want := rgba8(c)
		best, bestDist := -1, 0
		for i, existing := range colors {
			have := rgba8(existing)
			if have == want {
				locked[i], best = true, -1
				break
			}
			if locked[i] {
				continue
			}
			dr, dg, db := int(have.R)-int(want.R), int(have.G)-int(want.G), int(have.B)-int(want.B)
			if d := dr*dr + dg*dg + db*db; best < 0 || d < bestDist {
				best, bestDist = i, d
			}
		}
		if best >= 0 {
			colors[best], locked[best] = want, true
		}
	}
	return colors
}

func rgba8(c color.Color) color.RGBA {
	r, g, b, _ := c.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
}

func (e *Encoder) Encode(w io.Writer) error {
	if len(e.gif.Image) == 0 {
		return fmt.Errorf("recording has no visible frames (use Show before finishing)")
	}
	e.gif.LoopCount = 0
	return gif.EncodeAll(w, &e.gif)
}

// Duration is the encoded timeline after centisecond rounding.
func (e *Encoder) Duration() time.Duration { return time.Duration(e.ticks) * 10 * time.Millisecond }

// Rotate starts the loop at a timeline fraction, splitting a held frame when
// needed so deduplication does not change the meaning of LoopOffset.
func (e *Encoder) Rotate(fraction float64) {
	if fraction <= 0 || fraction >= 1 || len(e.gif.Image) == 0 {
		return
	}
	offset := int(float64(e.ticks) * fraction)
	for i, d := range e.gif.Delay {
		if offset >= d {
			offset -= d
			continue
		}
		frames := append([]*image.Paletted(nil), e.gif.Image[i:]...)
		delays := append([]int(nil), e.gif.Delay[i:]...)
		delays[0] -= offset
		frames = append(frames, e.gif.Image[:i]...)
		delays = append(delays, e.gif.Delay[:i]...)
		if offset > 0 {
			frames = append(frames, e.gif.Image[i])
			delays = append(delays, offset)
		}
		e.gif.Image, e.gif.Delay = frames, delays
		return
	}
}
