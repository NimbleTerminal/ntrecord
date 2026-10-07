// Package render owns a libghostty terminal and paints its viewport in Go.
// A Renderer is confined to one goroutine, including Close and Write.
package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/tape"
	vt "go.mitchellh.com/libghostty"
	syspng "go.mitchellh.com/libghostty/sys/png"
	xdraw "golang.org/x/image/draw"
)

var initPNG = sync.OnceValue(func() error { return vt.SysSetDecodePng(syspng.Decode) })

type placement struct {
	image    *image.NRGBA // Go-owned; no borrowed data escapes snapshotImages.
	meta     vt.KittyGraphicsPlacementInfo
	geometry vt.KittyGraphicsPlacementRenderInfo
	fragment *placeholderFragment
}

type cachedImage struct {
	generation uint64
	pixels     *image.NRGBA
}

type Renderer struct {
	config                                           tape.Config
	outputWidth, outputHeight, barHeight             int
	blinkStart                                       time.Time
	frame                                            *image.RGBA
	frameBlinks, frameBlinkOn                        bool
	frameImages, frameMissingImages                  map[uint32]struct{}
	presented                                        *image.RGBA
	presentedImages                                  map[uint32]struct{}
	imageGapUntil                                    time.Time
	term                                             *vt.Terminal
	state                                            *vt.RenderState
	rows                                             *vt.RenderStateRowIterator
	cells                                            *vt.RenderStateRowCells
	iter                                             *vt.KittyGraphicsPlacementIterator
	atlas                                            *atlas
	placements                                       []placement
	images                                           map[uint32]cachedImage
	replies                                          []byte
	holdUntil                                        time.Time
	Cols, Rows, Width, Height, CellWidth, CellHeight int
}

// Engine describes the libghostty-vt library this binary is linked against.
type Engine struct {
	Version       string
	KittyGraphics bool
	SIMD          bool
}

// EngineInfo reports the linked terminal engine's build.
func EngineInfo() (Engine, error) {
	info, err := vt.GetBuildInfo()
	if err != nil {
		return Engine{}, err
	}
	return Engine{Version: info.VersionString, KittyGraphics: info.KittyGraphics, SIMD: info.SIMD}, nil
}

func New(c tape.Config) (_ *Renderer, err error) {
	if err = initPNG(); err != nil {
		return nil, err
	}
	if !(c.FontSize >= 6 && c.FontSize <= 96) {
		return nil, fmt.Errorf("font size must be between 6 and 96")
	}
	r := &Renderer{config: c, blinkStart: time.Now()}
	if c.WindowBar != "" && c.WindowBar != "none" {
		r.barHeight = c.WindowBarSize
	}
	extrasX := 2 * (c.Padding + c.Margin)
	extrasY := extrasX + r.barHeight
	defer func() {
		if err != nil {
			r.Close()
		}
	}()
	if r.atlas, err = newAtlasFamily(c.FontSize, c.FontFamily); err != nil {
		return nil, err
	}
	lineHeight := c.LineHeight
	if lineHeight == 0 {
		lineHeight = 1
	}
	r.CellWidth = int(math.Ceil(float64(r.atlas.width) + c.LetterSpacing))
	r.CellHeight = int(math.Ceil(float64(r.atlas.height) * lineHeight))
	if r.CellWidth < 1 || r.CellHeight < 1 {
		return nil, fmt.Errorf("font spacing leaves no room for a cell")
	}
	r.atlas.ascent += (r.CellHeight - r.atlas.height) / 2
	r.Cols, r.Rows = c.Cols, c.Rows
	if c.Width != 0 {
		r.Cols = (c.Width - extrasX) / r.CellWidth
	}
	if c.Height != 0 {
		r.Rows = (c.Height - extrasY) / r.CellHeight
	}
	if r.Cols < 1 || r.Cols > 8192 || r.Rows < 1 || r.Rows > 8192 {
		return nil, fmt.Errorf("grid dimensions must be between 1 and 8192")
	}
	r.Width, r.Height = r.Cols*r.CellWidth, r.Rows*r.CellHeight
	r.outputWidth, r.outputHeight = r.Width+extrasX, r.Height+extrasY
	if c.Width > 0 {
		r.outputWidth = c.Width
	}
	if c.Height > 0 {
		r.outputHeight = c.Height
	}
	if r.outputWidth > 8192 || r.outputHeight > 8192 || r.outputWidth*r.outputHeight > 16*1024*1024 {
		return nil, fmt.Errorf("viewport must contain at least one cell, be <=8192 pixels per side and <=16 megapixels")
	}
	r.term, err = vt.NewTerminal(vt.WithSize(uint16(r.Cols), uint16(r.Rows)),
		vt.WithRenderHold(func(_ *vt.Terminal, held bool) {
			if held {
				r.holdUntil = time.Now().Add(time.Second)
			} else {
				r.holdUntil = time.Time{}
			}
		}),
		vt.WithWritePty(func(_ *vt.Terminal, b []byte) { r.replies = append(r.replies, b...) }),
		vt.WithDeviceAttributes(func(_ *vt.Terminal) (vt.DeviceAttributes, bool) {
			return vt.DeviceAttributes{Primary: vt.DeviceAttributesPrimary{ConformanceLevel: 62, Features: [64]uint16{1, 22}, NumFeatures: 2}}, true
		}),
		vt.WithSizeReport(func(_ *vt.Terminal) (vt.SizeReportSize, bool) {
			return vt.SizeReportSize{Columns: uint16(r.Cols), Rows: uint16(r.Rows), CellWidth: uint32(r.CellWidth), CellHeight: uint32(r.CellHeight)}, true
		}), vt.WithXtversion(func(_ *vt.Terminal) string { return "sigmamax " + c.ProgramVersion() }))
	if err != nil {
		return nil, err
	}
	limit := uint64(64 * 1024 * 1024)
	if err = r.term.SetKittyImageStorageLimit(&limit); err != nil {
		return nil, err
	}
	if err = r.term.Resize(uint16(r.Cols), uint16(r.Rows), uint32(r.CellWidth), uint32(r.CellHeight)); err != nil {
		return nil, err
	}
	bg, fg := vt.ColorRGB{R: 24, G: 26, B: 32}, vt.ColorRGB{R: 224, G: 228, B: 235}
	if err = r.term.SetColorBackground(&bg); err != nil {
		return nil, err
	}
	if err = r.term.SetColorForeground(&fg); err != nil {
		return nil, err
	}
	if err = r.applyTheme(c); err != nil {
		return nil, err
	}
	if err = r.term.SetDefaultCursorBlink(&c.CursorBlink); err != nil {
		return nil, err
	}
	// File, tempfile and shared-memory transports are disabled by default.
	if r.state, err = vt.NewRenderState(); err != nil {
		return nil, err
	}
	if r.rows, err = vt.NewRenderStateRowIterator(); err != nil {
		return nil, err
	}
	if r.cells, err = vt.NewRenderStateRowCells(); err != nil {
		return nil, err
	}
	if r.iter, err = vt.NewKittyGraphicsPlacementIterator(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Renderer) Close() {
	if r.iter != nil {
		r.iter.Close()
	}
	if r.cells != nil {
		r.cells.Close()
	}
	if r.rows != nil {
		r.rows.Close()
	}
	if r.state != nil {
		r.state.Close()
	}
	if r.term != nil {
		r.term.Close()
	}
	if r.atlas != nil {
		r.atlas.close()
	}
}

// Write is the only VT input path. libghostty parses APCs and produces replies
// through WithWritePty; sigmamax never parses escape protocols a second time.
func (r *Renderer) Write(b []byte) error {
	r.frame = nil
	r.term.VTWrite(b)
	return r.snapshotImages()
}

func (r *Renderer) TakeReplies() []byte {
	b := r.replies
	r.replies = nil
	return b
}

// RenderHeld honors synchronized output (2026) while an app updates its grid.
// The deadline prevents a crashed client from freezing capture indefinitely.
func (r *Renderer) RenderHeld(now time.Time) bool { return now.Before(r.holdUntil) }

func (r *Renderer) snapshotImages() error {
	r.placements = nil
	kg, err := r.term.KittyGraphics()
	if err != nil {
		return err
	}
	if kg == nil {
		r.images = nil
		return nil
	}
	if err := kg.PlacementIterator(r.iter); err != nil {
		return err
	}
	images := make(map[uint32]cachedImage)
	for r.iter.Next() {
		meta, err := r.iter.Info()
		if err != nil {
			return err
		}
		im := kg.Image(meta.ImageID)
		if im == nil {
			continue
		}
		g, err := r.iter.RenderInfo(im, r.term)
		if err != nil {
			return err
		}
		if !meta.IsVirtual && (!g.ViewportVisible || g.GridCols == 0 || g.GridRows == 0 || g.PixelWidth == 0 || g.PixelHeight == 0) {
			continue
		}
		pixels := images[meta.ImageID].pixels
		if pixels == nil {
			info, err := im.Info()
			if err != nil {
				return err
			}
			if info.DataPending || info.Width == 0 || info.Height == 0 {
				continue
			}
			// Reuse only owned copies whose content generation hasn't changed.
			// Chunked APC uploads otherwise recopy every stored image per chunk.
			if previous, ok := r.images[meta.ImageID]; ok && previous.generation == info.Generation {
				images[meta.ImageID] = previous
				pixels = previous.pixels
			} else {
				channels := 4
				if info.Format == vt.KittyImageFormatRGB {
					channels = 3
				} else if info.Format != vt.KittyImageFormatRGBA {
					continue
				}
				if uint64(info.Width)*uint64(info.Height)*uint64(channels) > uint64(len(info.Data)) {
					return fmt.Errorf("short Kitty pixel buffer")
				}
				pixels = image.NewNRGBA(image.Rect(0, 0, int(info.Width), int(info.Height)))
				if channels == 4 {
					copy(pixels.Pix, info.Data)
				} else {
					for src, dst := 0, 0; dst < len(pixels.Pix); src, dst = src+3, dst+4 {
						copy(pixels.Pix[dst:dst+3], info.Data[src:src+3])
						pixels.Pix[dst+3] = 255
					}
				}
				images[meta.ImageID] = cachedImage{info.Generation, pixels}
			}
		}
		// All geometry is copied by value before the next mutating call.
		r.placements = append(r.placements, placement{image: pixels, meta: *meta, geometry: *g})
	}
	r.images = images
	return nil
}

func sortPlacements(placements []placement) {
	sort.SliceStable(placements, func(i, j int) bool {
		a, b := placements[i].meta, placements[j].meta
		if a.Z != b.Z {
			return a.Z < b.Z
		}
		if a.ImageID != b.ImageID {
			return a.ImageID < b.ImageID
		}
		return a.PlacementID < b.PlacementID
	})
}

func rgb(c vt.ColorRGB) color.RGBA { return color.RGBA{c.R, c.G, c.B, 255} }

type cell struct {
	x, y             int
	text             []uint32
	style            vt.RenderCellStyle
	fg, bg           color.RGBA
	hasBG, invisible bool
	placeholder      *placeholderCell
}

// Frame returns a cached, immutable bitmap. Callers must not modify it.
func (r *Renderer) Frame() (*image.RGBA, error) { return r.FrameAt(time.Now()) }

func (r *Renderer) FrameAt(now time.Time) (*image.RGBA, error) {
	blinkOn := now.Sub(r.blinkStart)/(500*time.Millisecond)%2 == 0
	if r.frame != nil && (!r.frameBlinks || r.frameBlinkOn == blinkOn) {
		return r.frame, nil
	}
	if err := r.state.Update(r.term); err != nil {
		return nil, err
	}
	colors, err := r.state.Colors()
	if err != nil {
		return nil, err
	}
	dst := image.NewRGBA(image.Rect(0, 0, r.Width, r.Height))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(rgb(colors.Background)), image.Point{}, draw.Src)
	var cells []cell
	if err := r.state.RowIterator(r.rows); err != nil {
		return nil, err
	}
	for y := 0; r.rows.Next(); y++ {
		if err := r.rows.Cells(r.cells); err != nil {
			return nil, err
		}
		for x := 0; r.cells.Next(); x++ {
			c := cell{x: x * r.CellWidth, y: y * r.CellHeight, fg: rgb(colors.Foreground), bg: rgb(colors.Background)}
			if err := r.cells.StyleInto(&c.style); err != nil {
				return nil, err
			}
			c.text, err = r.cells.Graphemes()
			if err != nil {
				return nil, err
			}
			if c.style.HasForeground {
				c.fg = rgb(c.style.Foreground)
			}
			if c.style.HasBackground {
				c.bg = rgb(c.style.Background)
				c.hasBG = true
			}
			if c.style.Inverse {
				c.fg, c.bg = c.bg, c.fg
				c.hasBG = true
			}
			if c.style.Faint {
				c.fg.R /= 2
				c.fg.G /= 2
				c.fg.B /= 2
			}
			isPlaceholder := len(c.text) > 0 && c.text[0] == kittyPlaceholder
			if c.style.HasStyling || isPlaceholder {
				s, err := r.cells.Style()
				if err != nil {
					return nil, err
				}
				c.invisible = s.Invisible()
				if isPlaceholder {
					c.placeholder = decodePlaceholder(c.text, s.FgColor(), s.UnderlineColor())
				}
			}
			cells = append(cells, c)
		}
	}
	placements := r.resolvePlaceholders(cells)
	sortPlacements(placements)
	r.paintImages(dst, placements, 0)
	for _, c := range cells {
		if c.hasBG {
			draw.Draw(dst, image.Rect(c.x, c.y, c.x+r.CellWidth, c.y+r.CellHeight), image.NewUniform(c.bg), image.Point{}, draw.Src)
		}
	}
	r.paintImages(dst, placements, 1)
	for _, c := range cells {
		if c.invisible || c.placeholder != nil {
			continue
		}
		for _, cp := range c.text {
			if cp == 0 || cp == ' ' {
				continue
			}
			g := r.atlas.glyph(glyphKey{rune(cp), c.fg, c.style.Bold, c.style.Italic})
			draw.Draw(dst, g.Bounds().Add(image.Pt(c.x, c.y)), g, g.Bounds().Min, draw.Over)
		}
		if c.style.Underline {
			draw.Draw(dst, image.Rect(c.x, c.y+r.atlas.ascent+1, c.x+r.CellWidth, c.y+r.atlas.ascent+2), image.NewUniform(c.fg), image.Point{}, draw.Src)
		}
		if c.style.Strikethrough {
			draw.Draw(dst, image.Rect(c.x, c.y+r.CellHeight/2, c.x+r.CellWidth, c.y+r.CellHeight/2+1), image.NewUniform(c.fg), image.Point{}, draw.Src)
		}
	}
	cursor, err := r.state.Cursor()
	if err != nil {
		return nil, err
	}
	if cursor.Visible && cursor.ViewportHasValue && (!r.config.CursorBlink || !cursor.Blinking || blinkOn) {
		x, y := int(cursor.ViewportX)*r.CellWidth, int(cursor.ViewportY)*r.CellHeight
		rect := image.Rect(x, y, x+r.CellWidth, y+r.CellHeight)
		switch cursor.VisualStyle {
		case vt.CursorVisualStyleBar:
			rect.Max.X = x + 2
		case vt.CursorVisualStyleUnderline:
			rect.Min.Y = rect.Max.Y - 2
		}
		c := rgb(colors.Foreground)
		if colors.CursorHasValue {
			c = rgb(colors.Cursor)
		}
		// NRGBA applies straight alpha correctly when compositing a block.
		draw.Draw(dst, rect, image.NewUniform(color.NRGBA{c.R, c.G, c.B, 150}), image.Point{}, draw.Over)
	}
	r.paintImages(dst, placements, 2)
	if err := r.state.Clean(); err != nil {
		return nil, err
	}
	r.frame = r.decorate(dst, rgb(colors.Background))
	r.frameBlinks = r.config.CursorBlink && cursor.Blinking && cursor.Visible && cursor.ViewportHasValue
	r.frameBlinkOn = blinkOn
	return r.frame, nil
}

func (r *Renderer) paintImages(dst *image.RGBA, placements []placement, layer int) {
	for _, p := range placements {
		l := 2
		if p.meta.Z < -1073741824 {
			l = 0
		} else if p.meta.Z < 0 {
			l = 1
		}
		if l != layer {
			continue
		}
		if p.fragment != nil {
			f := p.fragment
			clip := f.clip.Intersect(dst.Bounds())
			if !clip.Empty() {
				xdraw.NearestNeighbor.Scale(dst.SubImage(clip).(*image.RGBA), f.dest, p.image, p.image.Bounds(), draw.Over, nil)
			}
			continue
		}
		g := p.geometry
		// Pixel dimensions preserve native size/aspect ratio; grid dimensions
		// describe cell occupancy, not an instruction to stretch to whole cells.
		x := int(g.ViewportCol)*r.CellWidth + min(int(p.meta.XOffset), r.CellWidth-1)
		y := int(g.ViewportRow)*r.CellHeight + min(int(p.meta.YOffset), r.CellHeight-1)
		dr := image.Rect(x, y, x+int(g.PixelWidth), y+int(g.PixelHeight))
		sr := image.Rect(int(g.SourceX), int(g.SourceY), int(g.SourceX+g.SourceWidth), int(g.SourceY+g.SourceHeight))
		if dr.Overlaps(dst.Bounds()) && !sr.Empty() {
			xdraw.NearestNeighbor.Scale(dst, dr, p.image, sr, draw.Over, nil)
		}
	}
}
