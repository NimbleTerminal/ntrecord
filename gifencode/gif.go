// Package gifencode encodes timed RGBA frames as animated GIFs with a global
// adaptive palette, repeated-frame merging, and changed-region deltas.
// It uses only the Go standard library; no external encoder is required.
// Encoders must not be used concurrently.
package gifencode

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io"
	"math"
	"time"
)

// Encoder accumulates frames within a memory budget. Its zero value is ready
// for use. Set options before the first Add and leave them unchanged afterward.
// Input frames must be opaque; composite transparency onto a background first.
// The resulting GIF loops forever. No method closes a caller-owned writer.
type Encoder struct {
	// Seed colors (theme, decorations) are exact, alongside black and white.
	Seed color.Palette
	// FixedPalette retains seeded Plan9 quantization, for comparison/tools.
	// The default is a stable, recording-wide adaptive palette.
	FixedPalette bool
	// MaxBytes bounds owned storage and scratch space, excluding caller input.
	// Zero selects 512 MiB. It is a conservative working-set bound, not an RSS
	// limit on the Go runtime, renderer, or caller's retained frames.
	MaxBytes int64

	gif        gif.GIF // configuration and timeline; images are generated at Encode
	sources    []*sourceFrame
	previous   []byte
	histogram  *colorHistogram
	elapsed    time.Duration
	ticks      int
	bytes      int64 // full paletted canvases reserved for final encoding
	compressed int64
	failure    error
}

type sourceFrame struct{ data []byte }

const defaultMaxBytes int64 = 512 << 20

// Includes histogram, median-cut entries/boxes, palette maps, zlib, GIF/LZW,
// and allocator slack. Pixel buffers and per-frame metadata are separate.
const workspaceBytes int64 = 8 << 20
const frameOverhead int64 = 8192

func (e *Encoder) budget(pixels, retained, compressed int64, frames, width int) error {
	limit := e.MaxBytes
	if limit == 0 {
		limit = defaultMaxBytes
	}
	// Two full indexed frame sets bound originals plus deltas. Ten viewport
	// bytes cover the previous/decompressed RGBA frames and quantizer scratch.
	// Compressed slices can grow geometrically: charge capacity/growth slack.
	if workspaceBytes+10*pixels+64*int64(width)+2*retained+3*compressed+frameOverhead*int64(frames+1) > limit {
		return fmt.Errorf("GIF encoding exceeds %d byte memory budget (sources, palettes, frames and scratch space); use a smaller viewport or shorter session", limit)
	}
	return nil
}

// Add copies frame pixels before returning, so callers may reuse the input.
// Frames must have equal dimensions and zero-origin bounds. Nonpositive durations
// are ignored; positive durations accumulate and round to GIF centiseconds.
func (e *Encoder) Add(frame *image.RGBA, duration time.Duration) error {
	if e.failure != nil {
		return e.failure
	}
	if duration <= 0 {
		return nil
	}
	if frame == nil || frame.Rect.Min != (image.Point{}) || frame.Rect.Empty() || frame.Rect.Dx() > 65535 || frame.Rect.Dy() > 65535 {
		return fmt.Errorf("GIF frames require nonempty, zero-origin dimensions below 65536")
	}
	width, height := frame.Rect.Dx(), frame.Rect.Dy()
	if len(e.sources) > 0 && (width != e.gif.Config.Width || height != e.gif.Config.Height) {
		return fmt.Errorf("GIF frame dimensions changed")
	}
	if frame.Stride < width*4 || int64(frame.Stride)*int64(height-1)+int64(width)*4 > int64(len(frame.Pix)) {
		return fmt.Errorf("invalid GIF frame storage")
	}
	if len(e.Seed) > 256 {
		return fmt.Errorf("GIF palette seed exceeds 256 colors")
	}
	if duration > time.Duration(math.MaxInt64)-e.elapsed-5*time.Millisecond {
		return fmt.Errorf("GIF duration overflow")
	}
	elapsed := e.elapsed + duration
	total := int((elapsed + 5*time.Millisecond) / (10 * time.Millisecond))
	delay := total - e.ticks
	if delay <= 0 {
		e.elapsed = elapsed
		return nil
	}
	pixels := int64(width) * int64(height)
	same := len(e.previous) == int(pixels)*4
	if same {
		for y := 0; y < height; y++ {
			if !bytes.Equal(e.previous[y*width*4:(y+1)*width*4], frame.Pix[y*frame.Stride:y*frame.Stride+width*4]) {
				same = false
				break
			}
		}
	}
	retained := e.bytes
	if !same {
		retained += pixels
	}
	extra := (delay + 65534) / 65535
	if err := e.budget(pixels, retained, e.compressed, len(e.sources)+extra, width); err != nil {
		return err
	}
	var source *sourceFrame
	if same {
		source = e.sources[len(e.sources)-1]
	} else {
		// Compression is bounded before each write, including buffer growth; a
		// noisy frame cannot bypass the budget while constructing its source data.
		storage := limitedSource{check: func(n int64) error { return e.budget(pixels, retained, e.compressed+n, len(e.sources)+extra, width) }}
		zw, _ := zlib.NewWriterLevel(&storage, zlib.BestSpeed)
		for y := 0; y < height; y++ {
			if _, err := zw.Write(frame.Pix[y*frame.Stride : y*frame.Stride+width*4]); err != nil {
				_ = zw.Close()
				return err
			}
		}
		if err := zw.Close(); err != nil {
			return err
		}
		source = &sourceFrame{data: storage.Bytes()}
		e.compressed += int64(len(source.data))
		e.bytes = retained
		if len(e.previous) != int(pixels)*4 {
			e.previous = make([]byte, int(pixels)*4)
		}
		for y := 0; y < height; y++ {
			copy(e.previous[y*width*4:(y+1)*width*4], frame.Pix[y*frame.Stride:y*frame.Stride+width*4])
		}
		if !e.FixedPalette {
			if e.histogram == nil {
				e.histogram = new(colorHistogram)
			}
			e.histogram.observe(frame)
		}
	}
	e.elapsed, e.ticks = elapsed, total
	e.gif.Config = image.Config{Width: width, Height: height}
	if same {
		i := len(e.gif.Delay) - 1
		n := min(65535-e.gif.Delay[i], delay)
		e.gif.Delay[i] += n
		delay -= n
	}
	for delay > 0 {
		n := min(delay, 65535)
		e.sources = append(e.sources, source)
		e.gif.Delay = append(e.gif.Delay, n)
		delay -= n
	}
	return nil
}

type limitedSource struct {
	bytes.Buffer
	check func(int64) error
}

func (w *limitedSource) Write(p []byte) (int, error) {
	if err := w.check(int64(w.Len()) + int64(len(p))); err != nil {
		return 0, err
	}
	return w.Buffer.Write(p)
}

// defaultSeed is ntrecord's built-in terminal background and foreground.
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

func (e *Encoder) quantized() (gif.GIF, error) {
	g := e.gif
	if e.failure != nil {
		return g, e.failure
	}
	if len(e.sources) == 0 {
		return g, fmt.Errorf("GIF encoding has no visible frames")
	}
	pixels := int64(g.Config.Width) * int64(g.Config.Height)
	if err := e.budget(pixels, e.bytes, e.compressed, len(e.sources), g.Config.Width); err != nil {
		return g, err
	}
	pal := buildPalette(e.Seed)
	if !e.FixedPalette {
		pal = e.histogram.palette(e.Seed)
	}
	g.Config.ColorModel = pal
	g.LoopCount = 0
	g.Image = make([]*image.Paletted, len(e.sources))
	exact := make(map[uint32]uint8, len(pal))
	for i, c := range pal {
		v := rgba8(c)
		exact[uint32(v.R)<<16|uint32(v.G)<<8|uint32(v.B)] = uint8(i)
	}
	raw := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	// Held frames and a LoopOffset split share the same source allocation.
	done := make(map[*sourceFrame]*image.Paletted, len(e.sources))
	for i, source := range e.sources {
		if p := done[source]; p != nil {
			g.Image[i] = p
			continue
		}
		zr, err := zlib.NewReader(bytes.NewReader(source.data))
		if err != nil {
			return g, err
		}
		_, err = io.ReadFull(zr, raw.Pix)
		closeErr := zr.Close()
		if err != nil {
			return g, err
		}
		if closeErr != nil {
			return g, closeErr
		}
		p := image.NewPaletted(raw.Bounds(), pal)
		draw.FloydSteinberg.Draw(p, p.Bounds(), raw, image.Point{})
		// Exact input colors must remain exact even when neighboring quantization
		// error diffuses into them (flat backgrounds, text, black/white QR modules).
		for j := range p.Pix {
			k := j * 4
			if v, ok := exact[uint32(raw.Pix[k])<<16|uint32(raw.Pix[k+1])<<8|uint32(raw.Pix[k+2])]; ok {
				p.Pix[j] = v
			}
		}
		g.Image[i] = p
		done[source] = p
	}
	return g, nil
}

// Encode writes the collected animation to w without closing it. It may be
// called again with another writer. It reports palette/encoding errors and any
// deferred error from Rotate. The encoder retains its frames until released.
func (e *Encoder) Encode(w io.Writer) error {
	g, err := e.quantized()
	if err != nil {
		return err
	}
	compactPalette(&g)
	optimized := optimize(g)
	return gif.EncodeAll(w, &optimized)
}

// Duration is the encoded timeline after centisecond rounding.
func (e *Encoder) Duration() time.Duration { return time.Duration(e.ticks) * 10 * time.Millisecond }

// Rotate starts the loop at a timeline fraction, splitting a held frame when
// needed so deduplication does not change the meaning of LoopOffset.
func (e *Encoder) Rotate(fraction float64) {
	if math.IsNaN(fraction) || fraction <= 0 || fraction >= 1 || len(e.sources) == 0 {
		return
	}
	pixels := int64(e.gif.Config.Width) * int64(e.gif.Config.Height)
	if err := e.budget(pixels, e.bytes, e.compressed, len(e.sources)+1, e.gif.Config.Width); err != nil {
		e.failure = err
		return
	}
	offset := int(float64(e.ticks) * fraction)
	for i, d := range e.gif.Delay {
		if offset >= d {
			offset -= d
			continue
		}
		frames := append([]*sourceFrame(nil), e.sources[i:]...)
		delays := append([]int(nil), e.gif.Delay[i:]...)
		delays[0] -= offset
		frames = append(frames, e.sources[:i]...)
		delays = append(delays, e.gif.Delay[:i]...)
		if offset > 0 {
			frames = append(frames, e.sources[i])
			delays = append(delays, offset)
		}
		e.sources, e.gif.Delay = frames, delays
		return
	}
}
