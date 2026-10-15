package render

import (
	"image"
	"os"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// face is a rewrite of opentype.Face, so it is held to the original: same
// metrics and advances exactly, and glyph coverage within an alpha step or two (the
// original's one source of platform drift, which face exists to remove).
func TestFaceMatchesOpentype(t *testing.T) {
	// opentype's lerp is fused on arm64, which moves coverage by a step or two
	// (more at large sizes). With fusion off it must match exactly:
	//   NTRECORD_FACE_EXACT=1 go test -gcflags=all=-d=fmahash=n ./render -run FaceMatches
	tolerance := 2
	if os.Getenv("NTRECORD_FACE_EXACT") == "1" {
		tolerance = 0
	}
	for _, ttf := range [][]byte{gomono.TTF, gomonobolditalic.TTF} {
		parsed, err := sfnt.Parse(ttf)
		if err != nil {
			t.Fatal(err)
		}
		for _, size := range []float64{6, 11, 14.5, 18, 31, 96} {
			want, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
			if err != nil {
				t.Fatal(err)
			}
			got := newFace(parsed, size)
			if want.Metrics() != got.Metrics() {
				t.Fatalf("size %g metrics: %+v, want %+v", size, got.Metrics(), want.Metrics())
			}
			for r := rune(0x20); r < 0x2600; r++ {
				if r > 0x17f && r < 0x2500 && r%7 != 0 { // all of Latin and the box drawing; sample the middle
					continue
				}
				dot := fixed.P(3, 40)
				wdr, wmask, wmp, wadv, wok := want.Glyph(dot, r)
				gdr, gmask, gmp, gadv, gok := got.Glyph(dot, r)
				if wdr != gdr || wmp != gmp || wadv != gadv || wok != gok {
					t.Fatalf("size %g %U: rect/advance/ok differ: %v %v %v %v vs %v %v %v %v", size, r, gdr, gmp, gadv, gok, wdr, wmp, wadv, wok)
				}
				if wmask == nil || gmask == nil {
					if wmask != gmask {
						t.Fatalf("size %g %U: one mask is nil", size, r)
					}
					continue
				}
				wa, ga := wmask.(*image.Alpha), gmask.(*image.Alpha)
				for i := range wa.Pix {
					if d := int(wa.Pix[i]) - int(ga.Pix[i]); d < -tolerance || d > tolerance {
						t.Fatalf("size %g %U: coverage differs by %d at byte %d", size, r, d, i)
					}
				}
			}
		}
	}
}

func TestFaceAdvanceMatches(t *testing.T) {
	parsed, _ := sfnt.Parse(gomono.TTF)
	want, _ := opentype.NewFace(parsed, &opentype.FaceOptions{Size: 18, DPI: 72, Hinting: font.HintingFull})
	got := newFace(parsed, 18)
	wa, wok := want.GlyphAdvance('M')
	ga, gok := got.GlyphAdvance('M')
	if wa != ga || wok != gok {
		t.Fatalf("advance %v %v, want %v %v", ga, gok, wa, wok)
	}
}

func TestFaceSlot(t *testing.T) {
	// Each slot prefers the face whose style names it exactly: Bold over
	// Semibold or ExtraBold, Regular over Light, whatever order they are found.
	for _, tc := range []struct {
		better, worse string
		slot          int
	}{
		{"Bold", "Semibold", 1}, {"Bold", "Demi Bold", 1}, {"Bold", "ExtraBold", 1},
		{"Regular", "Light", 0}, {"Regular", "Medium", 0}, {"Book", "Thin", 0},
		{"Italic", "Light Italic", 2}, {"Oblique", "Medium Oblique", 2},
		{"Bold Italic", "Semibold Italic", 3}, {"BoldOblique", "ExtraBold Italic", 3},
	} {
		bs, br := faceSlot(tc.better)
		ws, wr := faceSlot(tc.worse)
		if bs != tc.slot || ws != tc.slot || br <= wr {
			t.Errorf("%s: slot %d rank %d; %s: slot %d rank %d; want slot %d, %s ranked higher", tc.better, bs, br, tc.worse, ws, wr, tc.slot, tc.better)
		}
	}
}
