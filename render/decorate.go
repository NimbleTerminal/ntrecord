package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"

	vt "go.mitchellh.com/libghostty"

	"nimbleterminal.dev/ntrecord/internal/theme"
)

func (r *Renderer) applyTheme(c Config) error {
	t, err := theme.Parse(c.Theme)
	if err != nil {
		return err
	}
	for _, entry := range []struct {
		key string
		set func(*vt.ColorRGB) error
	}{{"background", r.term.SetColorBackground}, {"foreground", r.term.SetColorForeground}, {"cursor", r.term.SetColorCursor}} {
		if s := t[entry.key]; s != "" {
			v, e := theme.Color(s)
			if e != nil {
				return e
			}
			if e = entry.set(&vt.ColorRGB{R: v.R, G: v.G, B: v.B}); e != nil {
				return e
			}
		}
	}
	palette, err := r.term.ColorPalette()
	if err != nil {
		return err
	}
	names := []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite"}
	for i, name := range names {
		if s := t[name]; s != "" {
			v, e := theme.Color(s)
			if e != nil {
				return e
			}
			palette[i] = vt.ColorRGB{R: v.R, G: v.G, B: v.B}
		}
	}
	return r.term.SetColorPalette(palette)
}

// windowBarColors are the close/minimize/maximize dots, left to right.
var windowBarColors = []color.RGBA{{255, 95, 86, 255}, {255, 189, 46, 255}, {39, 201, 63, 255}}

// PaletteSeed lists the colors a palette-limited encoder should reproduce
// exactly: terminal background and foreground first, then the cursor, ANSI
// palette, margin fill and window-bar dots. Photographic Kitty images and
// antialiased text are expected to be approximated.
func (r *Renderer) PaletteSeed() []color.RGBA {
	var seed []color.RGBA
	add := func(c *vt.ColorRGB, err error) {
		if err == nil && c != nil {
			seed = append(seed, color.RGBA{c.R, c.G, c.B, 255})
		}
	}
	add(r.term.ColorBackground())
	add(r.term.ColorForeground())
	add(r.term.ColorCursor())
	if palette, err := r.term.ColorPalette(); err == nil {
		for _, c := range palette[:16] {
			add(&c, nil)
		}
	}
	if r.config.MarginFill != "" {
		if c, err := theme.Color(r.config.MarginFill); err == nil {
			seed = append(seed, c)
		}
	}
	if r.barHeight > 0 {
		seed = append(seed, windowBarColors...)
	}
	return seed
}

// OutputSize includes decoration; Width/Height remain the PTY's pixel size.
func (r *Renderer) OutputSize() (int, int) { return r.outputWidth, r.outputHeight }

func (r *Renderer) decorate(src *image.RGBA, bg color.RGBA) *image.RGBA {
	c := r.config
	if r.outputWidth == r.Width && r.outputHeight == r.Height && c.BorderRadius == 0 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, r.outputWidth, r.outputHeight))
	margin := bg
	if c.MarginFill != "" {
		margin, _ = theme.Color(c.MarginFill)
	}
	draw.Draw(dst, dst.Bounds(), image.NewUniform(margin), image.Point{}, draw.Src)
	win := image.Rect(c.Margin, c.Margin, r.outputWidth-c.Margin, r.outputHeight-c.Margin)
	draw.Draw(dst, win, image.NewUniform(bg), image.Point{}, draw.Src)
	offset := image.Pt(c.Margin+c.Padding, c.Margin+c.Padding+r.barHeight)
	draw.Draw(dst, src.Bounds().Add(offset), src, image.Point{}, draw.Src)
	if r.barHeight > 0 {
		radius := min(6, max(2, r.barHeight/6))
		gap := radius * 3
		x := win.Min.X + c.Padding + radius + 8
		if strings.HasSuffix(c.WindowBar, "right") {
			x = win.Max.X - c.Padding - 8 - radius - 2*gap
		}
		y := win.Min.Y + r.barHeight/2
		for i, cc := range windowBarColors {
			for yy := -radius; yy <= radius; yy++ {
				for xx := -radius; xx <= radius; xx++ {
					d := xx*xx + yy*yy
					if d > radius*radius || strings.HasPrefix(c.WindowBar, "rings") && d < (radius-2)*(radius-2) {
						continue
					}
					dst.SetRGBA(x+i*gap+xx, y+yy, cc)
				}
			}
		}
	}
	radius := min(c.BorderRadius, min(win.Dx(), win.Dy())/2)
	if radius > 0 {
		rr := float64(radius)
		for y := 0; y < radius; y++ {
			for x := 0; x < radius; x++ {
				// Coverage smooths the outermost pixel of each rounded corner.
				dist := math.Hypot(rr-float64(x)-0.5, rr-float64(y)-0.5)
				coverage := min(1.0, max(0.0, rr+0.5-dist))
				for _, p := range []image.Point{{win.Min.X + x, win.Min.Y + y}, {win.Max.X - 1 - x, win.Min.Y + y}, {win.Min.X + x, win.Max.Y - 1 - y}, {win.Max.X - 1 - x, win.Max.Y - 1 - y}} {
					v := dst.RGBAAt(p.X, p.Y)
					blend := func(a, b uint8) uint8 { return uint8(float64(a)*coverage + float64(b)*(1-coverage)) }
					dst.SetRGBA(p.X, p.Y, color.RGBA{blend(v.R, margin.R), blend(v.G, margin.G), blend(v.B, margin.B), 255})
				}
			}
		}
	}
	return dst
}

// TerminalColors returns the initial colors used in a portable cast header.
func (r *Renderer) TerminalColors() (foreground, background string, palette []string, err error) {
	fg, err := r.term.ColorForeground()
	if err != nil {
		return "", "", nil, err
	}
	bg, err := r.term.ColorBackground()
	if err != nil {
		return "", "", nil, err
	}
	colors, err := r.term.ColorPalette()
	if err != nil {
		return "", "", nil, err
	}
	hex := func(c vt.ColorRGB) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
	if fg == nil || bg == nil {
		return "", "", nil, fmt.Errorf("terminal has no default colors")
	}
	for _, c := range colors[:16] {
		palette = append(palette, hex(c))
	}
	return hex(*fg), hex(*bg), palette, nil
}
