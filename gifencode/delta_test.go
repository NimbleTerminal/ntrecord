package gifencode

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func solid(w, h int, c color.Color) *image.RGBA {
	f := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(f, f.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	return f
}

// FFmpeg is independent of Go's GIF reader and composites deltas/disposal.
// Force one decoded output per GIF frame, preserving source timestamps.
func decodeFFmpeg(t *testing.T, data []byte, loop bool, frames int) []byte {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		if os.Getenv("NTRECORD_REQUIRE_FFMPEG") == "1" {
			t.Fatal(err)
		}
		t.Skip("independent GIF composition check requires ffmpeg")
	}
	input := filepath.Join(t.TempDir(), "test.gif")
	if err = os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ignore := "1"
	if loop {
		ignore = "0"
	}
	cmd := exec.CommandContext(ctx, path, "-v", "error", "-ignore_loop", ignore, "-min_delay", "0", "-i", input, "-fps_mode", "passthrough", "-frames:v", strconv.Itoa(frames), "-f", "rawvideo", "-pix_fmt", "rgba", "pipe:1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, &stderr)
	}
	return out
}

func TestDeltaCompositingAndLoopOffset(t *testing.T) {
	bg := color.RGBA{30, 30, 46, 255}
	red := color.RGBA{255, 0, 0, 255}
	a := solid(40, 24, bg)
	b := solid(40, 24, bg)
	draw.Draw(b, image.Rect(2, 2, 12, 12), image.NewUniform(red), image.Point{}, draw.Src)
	c := solid(40, 24, bg)
	draw.Draw(c, image.Rect(20, 5, 32, 18), image.White, image.Point{}, draw.Src)
	// Disappearance, a different replacement, blanking, then a held blank frame.
	for _, fraction := range []float64{0, 0.125, 0.5, 0.875} {
		t.Run(fmt.Sprint(fraction), func(t *testing.T) {
			e := Encoder{Seed: color.Palette{bg, red}}
			for _, f := range []*image.RGBA{a, b, c, a} {
				if err := e.Add(f, 200*time.Millisecond); err != nil {
					t.Fatal(err)
				}
			}
			e.Rotate(fraction)
			// Reference retains full quantized canvases, independent of patch logic.
			ref, err := e.quantized()
			if err != nil {
				t.Fatal(err)
			}
			ref.LoopCount = 0
			var original, optimized bytes.Buffer
			if err := gif.EncodeAll(&original, &ref); err != nil {
				t.Fatal(err)
			}
			if err := e.Encode(&optimized); err != nil {
				t.Fatal(err)
			}
			want := decodeFFmpeg(t, original.Bytes(), true, 2*len(ref.Image))
			got := decodeFFmpeg(t, optimized.Bytes(), true, 2*len(ref.Image))
			if !bytes.Equal(got, want) {
				t.Fatal("composited pixels differ across two loops")
			}
			g, err := gif.DecodeAll(bytes.NewReader(optimized.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if g.LoopCount != 0 || !slices.Equal(g.Delay, ref.Delay) {
				t.Fatalf("loop/delays: %d %v want %v", g.LoopCount, g.Delay, ref.Delay)
			}
			if g.Image[0].Bounds() != a.Bounds() {
				t.Fatal("first frame does not reset canvas")
			}
			for _, d := range g.Disposal {
				if d != gif.DisposalNone {
					t.Fatalf("disposal %d", d)
				}
			}
		})
	}
}

func TestSparseTransparencyAndOpaqueFallback(t *testing.T) {
	for _, allColors := range []bool{false, true} {
		t.Run(fmt.Sprint(allColors), func(t *testing.T) {
			e := Encoder{}
			a := solid(128, 64, color.Black)
			// Noisy unchanged image with widely separated changes benefits from holes.
			for y := 0; y < 64; y++ {
				for x := 0; x < 128; x++ {
					a.SetRGBA(x, y, rgba8(buildPalette(nil)[(x*17+y*31)%256]))
				}
			}
			b := solid(128, 64, color.Black)
			copy(b.Pix, a.Pix)
			b.Set(0, 0, color.White)
			b.Set(127, 63, color.White)
			if allColors {
				for x, c := range buildPalette(nil) {
					b.Set(x%128, x/128, c)
				}
			}
			for _, f := range []*image.RGBA{a, b, b} {
				if err := e.Add(f, 100*time.Millisecond); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			if err := e.Encode(&out); err != nil {
				t.Fatal(err)
			}
			g, err := gif.DecodeAll(bytes.NewReader(out.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if len(g.Image) != 2 || g.Delay[1] != 20 {
				t.Fatalf("duplicate merging: %v", g.Delay)
			}
			ref, err := e.quantized()
			if err != nil {
				t.Fatal(err)
			}
			var full bytes.Buffer
			if err = gif.EncodeAll(&full, &ref); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decodeFFmpeg(t, out.Bytes(), false, 2), decodeFFmpeg(t, full.Bytes(), false, 2)) {
				t.Fatal("transparent patch changed composition")
			}
			transparent := false
			for _, c := range g.Image[1].Palette {
				_, _, _, a := c.RGBA()
				transparent = transparent || a == 0
			}
			if !allColors && !transparent {
				t.Fatal("sparse patch did not use transparency")
			}
		})
	}
}

func TestExactColorsBesideDither(t *testing.T) {
	bg := color.RGBA{32, 25, 54, 255}
	e := Encoder{Seed: color.Palette{bg}}
	f := solid(128, 32, bg)
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			f.SetRGBA(x, y, color.RGBA{uint8(x * 3), uint8(y * 7), uint8(x + y), 255})
		}
	}
	if err := e.Add(f, time.Second); err != nil {
		t.Fatal(err)
	}
	g, err := e.quantized()
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 32; y++ {
		for x := 64; x < 128; x++ {
			if got := rgba8(g.Image[0].At(x, y)); got != bg {
				t.Fatalf("flat background dithered at %d,%d: %v", x, y, got)
			}
		}
	}
}

func TestMemoryBudgetAndValidation(t *testing.T) {
	f := solid(16, 16, color.Black)
	e := Encoder{MaxBytes: workspaceBytes + 10*256 + 2*256 + frameOverhead*3 + 64*16 + 4096}
	if err := e.Add(f, time.Second); err != nil {
		t.Fatal(err)
	}
	before := e.Duration()
	// A huge hold consumes metadata even when all pixels are shared.
	if err := e.Add(f, 1000*time.Hour); err == nil || !strings.Contains(err.Error(), "memory budget") {
		t.Fatalf("hold limit: %v", err)
	}
	if e.Duration() != before {
		t.Fatal("failed Add advanced timing")
	}
	for _, bad := range []*image.RGBA{nil, {Rect: image.Rect(0, 0, 65536, 1)}, {Rect: image.Rect(0, 0, 16, 16), Stride: 64}, solid(17, 16, color.Black)} {
		if err := e.Add(bad, time.Second); err == nil {
			t.Fatal("accepted invalid frame")
		}
	}
	e = Encoder{MaxBytes: workspaceBytes}
	if err := e.Add(f, time.Second); err == nil {
		t.Fatal("did not account for scratch/palette storage")
	}
}

func TestStridedFramesAndRounding(t *testing.T) {
	large := solid(20, 20, color.White)
	f := large.SubImage(image.Rect(0, 0, 8, 8)).(*image.RGBA)
	var e Encoder
	for range 120 {
		if err := e.Add(f, time.Second/120); err != nil {
			t.Fatal(err)
		}
	}
	if e.Duration() != time.Second {
		t.Fatalf("duration %v", e.Duration())
	}
	if len(e.sources) != 1 {
		t.Fatal("strided duplicates were not merged")
	}
}

func TestRecordingWidePaletteLearnsLateFlatColors(t *testing.T) {
	early := solid(128, 32, color.RGBA{24, 26, 32, 255})
	late := solid(128, 32, color.RGBA{73, 44, 117, 255})
	// Keep the left half unchanged while a new application background appears.
	draw.Draw(late, image.Rect(0, 0, 64, 32), early, image.Point{}, draw.Src)
	var e Encoder
	for _, f := range []*image.RGBA{early, late, early} {
		if err := e.Add(f, 100*time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	g, err := e.quantized()
	if err != nil {
		t.Fatal(err)
	}
	for i, source := range []*image.RGBA{early, late, early} {
		for y := 0; y < 32; y++ {
			for x := 0; x < 128; x++ {
				if rgba8(g.Image[i].At(x, y)) != source.RGBAAt(x, y) {
					t.Fatalf("frame %d pixel %d,%d changed", i, x, y)
				}
			}
		}
		if &g.Image[i].Palette[0] != &g.Image[0].Palette[0] {
			t.Fatal("frame-local palette would permit flicker")
		}
	}
	var a, b bytes.Buffer
	if err = e.Encode(&a); err != nil {
		t.Fatal(err)
	}
	if err = e.Encode(&b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("adaptive encoding is not deterministic")
	}
}

func TestCompressedSourceBudget(t *testing.T) {
	f := solid(64, 64, color.Black)
	var state uint32 = 1
	for i := range f.Pix {
		state = state*1664525 + 1013904223
		f.Pix[i] = byte(state >> 24)
	}
	// Enough for metadata, palette, decoded/quantized frames, but not the noisy
	// compressed source. Reject while building it without changing the timeline.
	e := Encoder{MaxBytes: workspaceBytes + 12*64*64 + 64*64 + 3*frameOverhead + 128}
	if err := e.Add(f, time.Second); err == nil || !strings.Contains(err.Error(), "memory budget") {
		t.Fatalf("source budget: %v", err)
	}
	if len(e.sources) != 0 || e.Duration() != 0 || e.histogram != nil {
		t.Fatal("failed compression mutated the recording")
	}
}

func TestAllPaletteEntriesNeededByPatch(t *testing.T) {
	pal := buildPalette(nil)
	a := image.NewPaletted(image.Rect(0, 0, 256, 4), pal)
	b := image.NewPaletted(a.Rect, pal)
	for y := 0; y < 4; y++ {
		for x := 0; x < 256; x++ {
			a.SetColorIndex(x, y, uint8(x))
			b.SetColorIndex(x, y, uint8(x))
			if y == 0 || y == 3 {
				b.SetColorIndex(x, y, uint8(x+1))
			}
		}
	}
	full := gif.GIF{Config: image.Config{Width: 256, Height: 4, ColorModel: pal}, Image: []*image.Paletted{a, b}, Delay: []int{10, 10}, LoopCount: 0}
	optimized := optimize(full)
	for _, c := range optimized.Image[1].Palette {
		_, _, _, alpha := c.RGBA()
		if alpha == 0 {
			t.Fatal("stole an index needed by a changed pixel")
		}
	}
	var want, got bytes.Buffer
	if err := gif.EncodeAll(&want, &full); err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(&got, &optimized); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decodeFFmpeg(t, want.Bytes(), false, 2), decodeFFmpeg(t, got.Bytes(), false, 2)) {
		t.Fatal("opaque fallback changed colors")
	}
}

func TestTransparentInputClearsPreviousContent(t *testing.T) {
	// Renderer inputs are premultiplied RGBA. GIF output is opaque; transparent
	// source pixels become black, never transparent holes revealing an old frame.
	var e Encoder
	for _, f := range []*image.RGBA{solid(8, 8, color.White), image.NewRGBA(image.Rect(0, 0, 8, 8))} {
		if err := e.Add(f, 100*time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := e.Encode(&out); err != nil {
		t.Fatal(err)
	}
	decoded := decodeFFmpeg(t, out.Bytes(), false, 2)
	want := append(solid(8, 8, color.White).Pix, solid(8, 8, color.Black).Pix...)
	if !bytes.Equal(decoded, want) {
		t.Fatal("transparent source left stale pixels")
	}
}
