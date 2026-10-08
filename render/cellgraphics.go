package render

import (
	"image"
	"image/color"
	"image/draw"
)

// Cell graphics are geometric, independent of font coverage and font bearings.
// Their edges meet at the actual cell boundary, including configured spacing.
func cellGraphic(r rune, w, h int, fg color.RGBA) *image.RGBA {
	if w < 1 || h < 1 || !(r >= 0x2500 && r <= 0x259f || r >= 0x2800 && r <= 0x28ff) {
		return nil
	}
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(x0, y0, x1, y1 int) {
		draw.Draw(im, image.Rect(x0, y0, x1, y1), image.NewUniform(fg), image.Point{}, draw.Src)
	}
	if r >= 0x2800 {
		bits := uint8(r - 0x2800)
		positions := [8]image.Point{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}
		for i, p := range positions {
			if bits&(1<<i) == 0 {
				continue
			}
			cx := (2*p.X + 1) * w / 4
			cy := (2*p.Y + 1) * h / 8
			rx, ry := max(1, w/5), max(1, h/10)
			fill(cx-rx/2, cy-ry/2, cx+(rx+1)/2, cy+(ry+1)/2)
		}
		return im
	}
	if r >= 0x2580 {
		switch {
		case r == 0x2580:
			fill(0, 0, w, (h+1)/2)
		case r >= 0x2581 && r <= 0x2588:
			fill(0, h-(h*int(r-0x2580)+7)/8, w, h)
		case r >= 0x2589 && r <= 0x258f:
			fill(0, 0, (w*int(0x2590-r)+7)/8, h)
		case r == 0x2590:
			fill(w/2, 0, w, h)
		case r >= 0x2591 && r <= 0x2593:
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					threshold := [4]int{0, 2, 3, 1}[(y%2)*2+x%2]
					if threshold < int(r-0x2590) {
						im.SetRGBA(x, y, fg)
					}
				}
			}
		case r == 0x2594:
			fill(0, 0, w, (h+7)/8)
		case r == 0x2595:
			fill(w-(w+7)/8, 0, w, h)
		default:
			masks := [10]uint8{4, 8, 1, 13, 9, 7, 11, 2, 6, 14}
			m := masks[r-0x2596]
			for i, p := range [4]image.Point{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				if m&(1<<i) != 0 {
					fill(p.X*w/2, p.Y*h/2, (p.X+1)*w/2, (p.Y+1)*h/2)
				}
			}
		}
		return im
	}
	d := boxDrawings[r-0x2500]
	cx, cy := w/2, h/2
	thin := max(1, h/16)
	if r >= 0x2571 && r <= 0x2573 {
		for y := 0; y < h; y++ {
			x := y * (w - 1) / max(1, h-1)
			if r != 0x2571 {
				fill(x-thin/2, y, x+(thin+1)/2, y+1)
			}
			if r != 0x2572 {
				fill(w-1-x-thin/2, y, w-x+(thin-1)/2, y+1)
			}
		}
		return im
	}
	if d.round {
		hx, vy := 1, 1
		if d.arms[0] != 0 {
			hx = -1
		}
		if d.arms[2] != 0 {
			vy = -1
		}
		radius := max(1, min(w, h)/3)
		ox, oy := cx+hx*radius, cy+vy*radius
		if hx > 0 {
			fill(ox, cy-thin/2, w, cy+(thin+1)/2)
		} else {
			fill(0, cy-thin/2, ox+1, cy+(thin+1)/2)
		}
		if vy > 0 {
			fill(cx-thin/2, oy, cx+(thin+1)/2, h)
		} else {
			fill(cx-thin/2, 0, cx+(thin+1)/2, oy+1)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dx, dy := x-ox, y-oy
				if dx*hx > 0 || dy*vy > 0 {
					continue
				}
				dist := dx*dx + dy*dy
				lo, hi := max(0, radius-thin/2), radius+(thin+1)/2
				if dist >= lo*lo && dist <= hi*hi {
					im.SetRGBA(x, y, fg)
				}
			}
		}
		return im
	}
	for arm, weight := range d.arms {
		if weight == 0 {
			continue
		}
		thickness := thin
		if weight == 2 {
			thickness = max(thin+1, thin*2)
		}
		offsets := []int{0}
		if weight == 3 {
			offsets = []int{-thin, thin}
		}
		for _, offset := range offsets {
			// Each double-line lane turns into the adjacent lane, rather than
			// crossing the gap between lanes at corners and T/cross junctions.
			end := 0
			if weight == 3 {
				near, far := 2, 3
				if arm >= 2 {
					near, far = 0, 1
				}
				if offset > 0 {
					near, far = far, near
				}
				if d.arms[near] == 3 {
					end = thin
				} else if d.arms[near] == 0 && d.arms[far] == 3 {
					end = -thin
				}
			} else {
				// A single stem must reach both lanes of a perpendicular double.
				if arm < 2 && (d.arms[2] == 3 || d.arms[3] == 3) || arm >= 2 && (d.arms[0] == 3 || d.arms[1] == 3) {
					end = -thin
				}
			}
			if arm < 2 {
				x0, x1 := 0, cx-end+(thickness+1)/2
				if arm == 1 {
					x0, x1 = cx+end-thickness/2, w
				}
				fill(x0, cy+offset-thickness/2, x1, cy+offset+(thickness+1)/2)
			} else {
				y0, y1 := 0, cy-end+(thickness+1)/2
				if arm == 3 {
					y0, y1 = cy+end-thickness/2, h
				}
				fill(cx+offset-thickness/2, y0, cx+offset+(thickness+1)/2, y1)
			}
		}
	}
	if d.dashes > 0 {
		horizontal := d.arms[0] != 0
		length := h
		if horizontal {
			length = w
		}
		for i := 1; i < int(d.dashes); i++ {
			pos := i * length / int(d.dashes)
			gap := max(1, length/(int(d.dashes)*3))
			if horizontal {
				draw.Draw(im, image.Rect(pos, 0, pos+gap, h), image.Transparent, image.Point{}, draw.Src)
			} else {
				draw.Draw(im, image.Rect(0, pos, w, pos+gap), image.Transparent, image.Point{}, draw.Src)
			}
		}
	}
	return im
}

type boxDrawing struct {
	arms   [4]uint8
	dashes uint8
	round  bool
}
