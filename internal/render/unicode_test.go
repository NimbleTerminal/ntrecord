package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NimbleMarkets/sigmamax/internal/tape"
)

func TestBrailleDotsAndCellEdges(t *testing.T) {
	fg := color.RGBA{255, 255, 255, 255}
	for bit, p := range [8]image.Point{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}} {
		im := cellGraphic(rune(0x2800+1<<bit), 16, 32, fg)
		for y := 0; y < 32; y++ {
			for x := 0; x < 16; x++ {
				if im.RGBAAt(x, y).A > 0 && (x/8 != p.X || y/8 != p.Y) {
					t.Fatalf("dot %d painted wrong quadrant at %d,%d", bit, x, y)
				}
			}
		}
		if im.RGBAAt((2*p.X+1)*4, (2*p.Y+1)*4).A == 0 {
			t.Fatalf("dot %d missing", bit)
		}
	}
	for _, size := range []image.Point{{4, 8}, {11, 21}, {58, 112}} {
		for _, r := range []rune{'─', '━', '═', '┼', '╬'} {
			im := cellGraphic(r, size.X, size.Y, fg)
			for _, x := range []int{0, size.X - 1} {
				found := false
				for y := 0; y < size.Y; y++ {
					found = found || im.RGBAAt(x, y).A > 0
				}
				if !found {
					t.Fatalf("%c fails to reach horizontal edge at %v", r, size)
				}
			}
		}
		for _, r := range []rune{'│', '┃', '║', '┼', '╬'} {
			im := cellGraphic(r, size.X, size.Y, fg)
			for _, y := range []int{0, size.Y - 1} {
				found := false
				for x := 0; x < size.X; x++ {
					found = found || im.RGBAAt(x, y).A > 0
				}
				if !found {
					t.Fatalf("%c fails to reach vertical edge at %v", r, size)
				}
			}
		}
		full := cellGraphic('█', size.X, size.Y, fg)
		for i := 3; i < len(full.Pix); i += 4 {
			if full.Pix[i] != 255 {
				t.Fatal("full block left a gap")
			}
		}
	}
	for r := rune(0x2500); r <= 0x259f; r++ {
		im := cellGraphic(r, 11, 21, fg)
		found := false
		for i := 3; i < len(im.Pix); i += 4 {
			found = found || im.Pix[i] > 0
		}
		if !found {
			t.Fatalf("missing cell graphic %U", r)
		}
	}
	blank := cellGraphic('\u2800', 11, 21, fg)
	for _, v := range blank.Pix {
		if v != 0 {
			t.Fatal("blank braille is not blank")
		}
	}
}
func TestClusterFallbackAndShaping(t *testing.T) {
	t.Setenv("SIGMAMAX_FONT_PATH", "")
	a, err := newAtlasFamily(24, "Go Mono")
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	key := func(s string, span int) clusterKey {
		return clusterKey{s, color.RGBA{255, 255, 255, 255}, false, false, span, a.width, a.height}
	}
	canvas := func(im *image.RGBA) *image.RGBA {
		out := image.NewRGBA(image.Rect(0, 0, a.width, a.height))
		draw.Draw(out, im.Bounds(), im, im.Bounds().Min, draw.Over)
		return out
	}
	combined := canvas(a.cluster(key("e\u0301", 1)))
	precomposed := canvas(a.cluster(key("é", 1)))
	if !bytes.Equal(combined.Pix, precomposed.Pix) {
		t.Error("combining accent differs from precomposed glyph")
	}
	if f := a.chooseFont([]rune("♞"), 0, 1); f == nil || f.name != "Noto Sans Symbols 2" {
		t.Fatalf("missing bundled symbol fallback: %+v", f)
	}
	var last []byte
	for _, s := range []string{"😀", "👍🏽", "👨‍👩‍👧‍👦", "🇯🇵", "1️⃣", "❤️", "👩🏽‍💻"} {
		im := a.cluster(key(s, 2))
		colors := map[color.RGBA]bool{}
		for y := im.Rect.Min.Y; y < im.Rect.Max.Y; y++ {
			for x := im.Rect.Min.X; x < im.Rect.Max.X; x++ {
				c := im.RGBAAt(x, y)
				if c.A > 0 {
					colors[c] = true
				}
			}
		}
		if len(colors) < 16 {
			t.Errorf("%s not rendered as color emoji (%d colors)", s, len(colors))
		}
		if im.Rect.Dx() > 2*a.width || im.Rect.Dy() > a.height {
			t.Errorf("%s escaped terminal cluster cells", s)
		}
		if bytes.Equal(last, im.Pix) {
			t.Errorf("%s reused previous emoji pixels", s)
		}
		last = append(last[:0], im.Pix...)
	}
}
func TestUnicodeRenderer(t *testing.T) {
	t.Setenv("SIGMAMAX_FONT_PATH", "")
	c := tape.DefaultConfig()
	c.Cols, c.Rows, c.FontSize = 64, 14, 24
	r, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	write(t, r, "\x1b[?25lUnicode: é ä ñ  ♞ ♞ ⌘\r\n"+
		"┌─────────┬─────────┐ ╭─────────╮ ╔════════╗\r\n"+
		"│ borders │ no gaps │ │ rounded │ ║ double ║\r\n"+
		"└─────────┴─────────┘ ╰─────────╯ ╚════════╝\r\n"+
		"Braille: ⠁⠂⠄⠈⠐⠠⡀⢀ ⣿ ⢕⡪\r\n"+
		"Blocks: ▁▂▃▄▅▆▇█ ▉▊▋▌▍▎▏ ░▒▓ ▖▗▘▙▚▛▜▝▞▟\r\n"+
		"Emoji: 😀 👍🏽 👨‍👩‍👧‍👦 🇯🇵 1️⃣ ❤️ 👩🏽‍💻 END\r\n"+
		"\x1b[1;3mBold italic: résumé é\x1b[0m\r\n")
	frame, err := r.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("SIGMAMAX_GOLDEN_DUMP"); dir != "" {
		if err = os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, "unicode.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, frame); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFallbackCatalogUsesConfiguredFonts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "symbols.ttf"), symbolsFont, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.ttf"), []byte("not a font"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SIGMAMAX_FONT_PATH", dir)
	var catalog fallbackCatalog
	f := catalog.resolve([]rune("♞"), 24, true, true)
	if f == nil || !covers(f.face.Cmap, []rune("♞")) {
		t.Fatal("configured font was not selected")
	}
	if again := catalog.resolve([]rune("♞"), 24, true, true); again != f {
		t.Fatal("fallback face was not reused")
	}
	if f := catalog.resolve([]rune("日本"), 24, false, false); f != nil {
		t.Fatal("selected font without full cluster coverage")
	}
}
func TestInstalledCJKFallback(t *testing.T) {
	path := "/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc"
	if _, err := os.Stat(path); err != nil {
		t.Skip("optional installed CJK font not available")
	}
	t.Setenv("SIGMAMAX_FONT_PATH", path)
	a, err := newAtlasFamily(24, "Go Mono")
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	f := a.chooseFont([]rune("日"), 0, 2)
	if f == nil || f.name != path {
		t.Fatal("installed CJK fallback was not used")
	}
	key := clusterKey{"日", color.RGBA{255, 255, 255, 255}, false, false, 2, a.width, a.height}
	im := a.cluster(key)
	key.text = "\ufffd"
	missing := a.cluster(key)
	if bytes.Equal(im.Pix, missing.Pix) {
		t.Fatal("CJK fell back to replacement mark")
	}
}
func TestEmojiGridAndVariationSelectors(t *testing.T) {
	t.Setenv("SIGMAMAX_FONT_PATH", "")
	c := tape.DefaultConfig()
	c.Cols, c.Rows = 32, 3
	r, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	write(t, r, "\x1b[?25lA👍🏽👨‍👩‍👧‍👦🇯🇵1️⃣❤️👩🏽‍💻B")
	cursor, err := r.stateCursorForTest()
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 14 {
		t.Fatalf("emoji sequences shifted following text: cursor=%d, want 14", cursor)
	}
	// Existing recordings/applications can explicitly request legacy width mode.
	write(t, r, "\x1b[?2027l\r\n👍🏽B")
	cursor, err = r.stateCursorForTest()
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 5 {
		t.Fatalf("explicit legacy width mode ignored: cursor=%d", cursor)
	}
	a := r.atlas
	mono := a.cluster(clusterKey{"❤︎", color.RGBA{0, 255, 0, 255}, false, false, 1, r.CellWidth, r.CellHeight})
	for i := 0; i < len(mono.Pix); i += 4 {
		if mono.Pix[i] != 0 || mono.Pix[i+2] != 0 {
			t.Fatal("text presentation selector emitted color emoji")
		}
	}
}
func (r *Renderer) stateCursorForTest() (int, error) {
	if err := r.state.Update(r.term); err != nil {
		return 0, err
	}
	c, err := r.state.Cursor()
	return int(c.ViewportX), err
}

func TestClusterStreamingWrapAndErase(t *testing.T) {
	t.Setenv("SIGMAMAX_FONT_PATH", "")
	c := tape.DefaultConfig()
	c.Cols, c.Rows = 8, 3
	whole, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer whole.Close()
	streamed, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer streamed.Close()
	input := "\x1b[?25l1234567👍🏽B é"
	write(t, whole, input)
	for _, b := range []byte(input) {
		if err := streamed.Write([]byte{b}); err != nil {
			t.Fatal(err)
		}
	}
	a, err := whole.Frame()
	if err != nil {
		t.Fatal(err)
	}
	b, err := streamed.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("UTF-8/read boundaries changed cluster rendering")
	}
	// A two-cell emoji wraps from the last column and occupies two cells.
	if err := whole.state.Update(whole.term); err != nil {
		t.Fatal(err)
	}
	cursor, err := whole.state.Cursor()
	if err != nil {
		t.Fatal(err)
	}
	if cursor.ViewportX != 5 || cursor.ViewportY != 1 {
		t.Fatalf("wrong wrapped cursor: %+v", cursor)
	}
	write(t, whole, "\x1b[H\x1b[2Jclean")
	write(t, streamed, "\x1bc\x1b[?25lclean")
	a, err = whole.Frame()
	if err != nil {
		t.Fatal(err)
	}
	b, err = streamed.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("cleared emoji left stale pixels")
	}
}

func TestUnicodeResourceLimits(t *testing.T) {
	t.Setenv("SIGMAMAX_FONT_PATH", "")
	a, err := newAtlasFamily(96, "Go Mono")
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	// Distinct colors at the maximum font size exercise the byte cap before
	// the entry cap; using blocks avoids testing font rasterization speed.
	for i := 0; i < 1500; i++ {
		k := clusterKey{"█", color.RGBA{uint8(i), uint8(i >> 8), 0, 255}, false, false, 1, a.width, a.height}
		a.cluster(k)
		if a.clusterBytes > maxAtlasBytes || len(a.clusters) > 4096 {
			t.Fatal("cluster cache exceeded limit")
		}
	}
	k := clusterKey{"a" + strings.Repeat("\u0301", maxClusterRunes), color.RGBA{255, 255, 255, 255}, false, false, 1, a.width, a.height}
	got := a.cluster(k)
	k.text = "\ufffd"
	if !bytes.Equal(got.Pix, a.cluster(k).Pix) {
		t.Fatal("oversized cluster was not bounded")
	}
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(path, symbolsFont, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFont(path, int64(len(symbolsFont)-1)); err == nil {
		t.Fatal("font source limit ignored")
	}
	t.Setenv("SIGMAMAX_FONT_PATH", path)
	var catalog fallbackCatalog
	catalog.scan()
	catalog.bytes = 128 << 20
	if catalog.resolve([]rune("♞"), 24, false, false) != nil {
		t.Fatal("fallback source budget ignored")
	}
}

func TestDoubleLineJunctionGaps(t *testing.T) {
	fg := color.RGBA{255, 255, 255, 255}
	for _, r := range []rune{'╬', '╔', '╗', '╚', '╝'} {
		im := cellGraphic(r, 11, 21, fg)
		if im.RGBAAt(5, 10).A != 0 {
			t.Fatalf("%c filled central lane gap", r)
		}
	}
	im := cellGraphic('╬', 11, 21, fg)
	for _, p := range []image.Point{{5, 9}, {5, 11}, {4, 10}, {6, 10}} {
		if im.RGBAAt(p.X, p.Y).A != 0 {
			t.Fatal("double cross junction crossed its lane gaps")
		}
	}
}

func BenchmarkUnicodeFrame(b *testing.B) {
	b.Setenv("SIGMAMAX_FONT_PATH", "")
	c := tape.DefaultConfig()
	c.Cols, c.Rows = 80, 24
	r, err := New(c)
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()
	if err := r.Write([]byte("\x1b[?25l" + strings.Repeat("┃ é ♞ ⣿ 😀 👍🏽 👨‍👩‍👧‍👦 🇯🇵 👩🏽‍💻 ┃\r\n", 20))); err != nil {
		b.Fatal(err)
	}
	if _, err := r.Frame(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r.frame = nil // Measure drawing, not the unchanged-frame fast path.
		if _, err := r.Frame(); err != nil {
			b.Fatal(err)
		}
	}
}

func TestFallbackSkipsUnreadableCandidate(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.ttf", i)), symbolsFont, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SIGMAMAX_FONT_PATH", dir)
	var c fallbackCatalog
	c.scan()
	if err := os.Remove(filepath.Join(dir, "0.ttf")); err != nil {
		t.Fatal(err)
	}
	if f := c.resolve([]rune("♞"), 24, false, false); f == nil || f.name != filepath.Join(dir, "1.ttf") {
		t.Fatal("unreadable first candidate blocked fallback")
	}
}
