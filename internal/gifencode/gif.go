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
	gif      gif.GIF
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
	colors := append(color.Palette(nil), palette.Plan9...)
	// Preserve the default background and foreground exactly.
	colors[254], colors[255] = color.RGBA{24, 26, 32, 255}, color.RGBA{224, 228, 235, 255}
	p := image.NewPaletted(frame.Bounds(), colors)
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
