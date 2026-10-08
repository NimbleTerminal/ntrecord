package render

import (
	"image"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// face draws the glyphs of one font at one size. It is golang.org/x/image's
// opentype.Face (BSD-licensed, https://golang.org/LICENSE) trimmed to what the
// atlas uses, with one change: curves are flattened here instead of in
// vector.Rasterizer.QuadTo and CubeTo.
//
// Those two call lerp, which computes px + t*(qx-px) with no explicit float32
// conversion. Go may fuse that into one rounding on arm64 (a fused multiply-add)
// and does not on amd64, so a glyph's coverage can differ by one alpha step
// between the two, and a frame's bytes with it. Here the product is rounded
// before the sum, as vector's own LineTo already does, so every platform
// computes the same coverage. Keep float32(a*b) wherever a product feeds a sum.
//
// A face is not safe for concurrent use.
type face struct {
	f       *sfnt.Font
	hinting font.Hinting
	scale   fixed.Int26_6

	metrics    font.Metrics
	metricsSet bool

	buf  sfnt.Buffer
	rast vector.Rasterizer
	mask image.Alpha
	// The rasterizer's pen is unexported, so the face tracks it for flattening.
	penX, penY float32
}

// newFace returns a face for f at size pixels (72 DPI, full hinting).
func newFace(f *sfnt.Font, size float64) *face {
	return &face{f: f, hinting: font.HintingFull, scale: fixed.Int26_6(0.5 + size*64)}
}

func (f *face) Close() error { return nil }

func (f *face) Metrics() font.Metrics {
	if !f.metricsSet {
		var err error
		if f.metrics, err = f.f.Metrics(&f.buf, f.scale, f.hinting); err != nil {
			f.metrics = font.Metrics{}
		}
		f.metricsSet = true
	}
	return f.metrics
}

func (f *face) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	x, _ := f.f.GlyphIndex(&f.buf, r)
	advance, err := f.f.GlyphAdvance(&f.buf, x, f.scale, f.hinting)
	return advance, err == nil && x != 0
}

// Glyph rasterizes r with its origin at dot. The returned mask is reused by the
// next call, so copy it first.
func (f *face) Glyph(dot fixed.Point26_6, r rune) (dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	x, err := f.f.GlyphIndex(&f.buf, r)
	if err != nil {
		return image.Rectangle{}, nil, image.Point{}, 0, false
	}
	return f.GlyphIndex(dot, x)
}

// GlyphIndex rasterizes the glyph selected by the shaping engine, retaining
// this renderer's deterministic outline flattening and hinting.
func (f *face) GlyphIndex(dot fixed.Point26_6, x sfnt.GlyphIndex) (dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	var err error
	// The advance comes first: LoadGlyph's segments are invalid once buf is reused.
	if advance, err = f.f.GlyphAdvance(&f.buf, x, f.scale, f.hinting); err != nil {
		return image.Rectangle{}, nil, image.Point{}, 0, false
	}
	segments, err := f.f.LoadGlyph(&f.buf, x, f.scale, nil)
	if err != nil {
		return image.Rectangle{}, nil, image.Point{}, 0, false
	}
	// Quantize the sub-pixel bounds, in the space containing the dot, to pixels.
	dBounds := segments.Bounds().Add(dot)
	dr.Min.X, dr.Min.Y = dBounds.Min.X.Floor(), dBounds.Min.Y.Floor()
	dr.Max.X, dr.Max.Y = dBounds.Max.X.Ceil(), dBounds.Max.Y.Ceil()
	width, height := dr.Dx(), dr.Dy()
	if width < 0 || height < 0 {
		return image.Rectangle{}, nil, image.Point{}, 0, false
	}
	// The bias moves glyph space into rasterizer space, whose origin is dr.Min.
	biasX := dot.X - fixed.Int26_6(dr.Min.X<<6)
	biasY := dot.Y - fixed.Int26_6(dr.Min.Y<<6)

	nPixels := width * height
	if cap(f.mask.Pix) < nPixels {
		f.mask.Pix = make([]uint8, 2*nPixels)
	}
	f.mask.Pix = f.mask.Pix[:nPixels]
	f.mask.Stride = width
	f.mask.Rect = image.Rect(0, 0, width, height)

	f.rast.Reset(width, height)
	f.rast.DrawOp = draw.Src
	point := func(p fixed.Point26_6) (float32, float32) {
		return float32(p.X+biasX) / 64, float32(p.Y+biasY) / 64
	}
	for _, seg := range segments {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			ax, ay := point(seg.Args[0])
			f.rast.MoveTo(ax, ay)
			f.penX, f.penY = ax, ay
		case sfnt.SegmentOpLineTo:
			ax, ay := point(seg.Args[0])
			f.lineTo(ax, ay)
		case sfnt.SegmentOpQuadTo:
			bx, by := point(seg.Args[0])
			cx, cy := point(seg.Args[1])
			f.quadTo(bx, by, cx, cy)
		case sfnt.SegmentOpCubeTo:
			bx, by := point(seg.Args[0])
			cx, cy := point(seg.Args[1])
			dx, dy := point(seg.Args[2])
			f.cubeTo(bx, by, cx, cy, dx, dy)
		}
	}
	f.rast.Draw(&f.mask, f.mask.Bounds(), image.Opaque, image.Point{})
	return dr, &f.mask, f.mask.Rect.Min, advance, x != 0
}

func (f *face) lineTo(x, y float32) {
	f.rast.LineTo(x, y)
	f.penX, f.penY = x, y
}

// quadTo flattens a quadratic Bézier from the pen via b to c, as
// vector.Rasterizer.QuadTo does, with products rounded before they are summed.
func (f *face) quadTo(bx, by, cx, cy float32) {
	ax, ay := f.penX, f.penY
	if devsq := devSquared(ax, ay, bx, by, cx, cy); devsq >= 0.333 {
		const tol = 3
		n := 1 + int(math.Sqrt(math.Sqrt(tol*float64(devsq))))
		t, nInv := float32(0), 1/float32(n)
		for i := 0; i < n-1; i++ {
			t += nInv
			abx, aby := lerp(t, ax, ay, bx, by)
			bcx, bcy := lerp(t, bx, by, cx, cy)
			f.lineTo(lerp(t, abx, aby, bcx, bcy))
		}
	}
	f.lineTo(cx, cy)
}

// cubeTo flattens a cubic Bézier from the pen via b and c to d, likewise.
func (f *face) cubeTo(bx, by, cx, cy, dx, dy float32) {
	ax, ay := f.penX, f.penY
	devsq := devSquared(ax, ay, bx, by, dx, dy)
	if alt := devSquared(ax, ay, cx, cy, dx, dy); devsq < alt {
		devsq = alt
	}
	if devsq >= 0.333 {
		const tol = 3
		n := 1 + int(math.Sqrt(math.Sqrt(tol*float64(devsq))))
		t, nInv := float32(0), 1/float32(n)
		for i := 0; i < n-1; i++ {
			t += nInv
			abx, aby := lerp(t, ax, ay, bx, by)
			bcx, bcy := lerp(t, bx, by, cx, cy)
			cdx, cdy := lerp(t, cx, cy, dx, dy)
			abcx, abcy := lerp(t, abx, aby, bcx, bcy)
			bcdx, bcdy := lerp(t, bcx, bcy, cdx, cdy)
			f.lineTo(lerp(t, abcx, abcy, bcdx, bcdy))
		}
	}
	f.lineTo(dx, dy)
}

// lerp is the point t of the way from p to q. The float32 conversions forbid
// the fused multiply-add that would make arm64 and amd64 disagree.
func lerp(t, px, py, qx, qy float32) (x, y float32) {
	return px + float32(t*(qx-px)), py + float32(t*(qy-py))
}

// devSquared measures how curved a to b to c is, which sets how many line
// segments approximate it.
func devSquared(ax, ay, bx, by, cx, cy float32) float32 {
	devx := ax - float32(2*bx) + cx
	devy := ay - float32(2*by) + cy
	return float32(devx*devx) + float32(devy*devy)
}
