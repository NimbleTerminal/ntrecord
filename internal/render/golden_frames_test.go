package render

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"nimbleterminal.dev/ntrecord/internal/tape"
)

// Golden frames pin what the renderer draws, bit for bit, on every platform CI
// runs on (macOS and Linux, amd64 and arm64). A frame is a pure function of
// terminal state, the embedded font, and integer-and-float pixel math; if any
// platform drifts by even one alpha step, its hash differs and the build fails.
//
//	NTRECORD_UPDATE_GOLDEN=1 go test ./internal/render -run Golden   rewrite the hashes
//	NTRECORD_GOLDEN_DUMP=dir go test ./internal/render -run Golden   also write PNGs
//
// Review the PNGs before committing an update: a changed hash is a changed picture.
const goldenFile = "testdata/golden/frames.sha256"

type scenario struct {
	name   string
	config func(*tape.Config)
	input  func(t *testing.T) string
}

func text(s string) func(*testing.T) string { return func(*testing.T) string { return s } }

func sgrRamp() string {
	var b strings.Builder
	for i := 0; i < 36; i++ {
		fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm█\x1b[48;2;%d;%d;%dm▀", 255-i*7, i*7, 128+i*3, i*7, 255-i*7, i*5)
	}
	b.WriteString("\x1b[0m\r\n")
	for i := 0; i < 36; i++ {
		fmt.Fprintf(&b, "\x1b[48;5;%dm \x1b[0m", 16+i*5)
	}
	return b.String() + "\r\n"
}

var scenarios = []scenario{
	{"unicode-clusters", nil, text("NFC: é ä ñ  ♞ ⌘\r\n😀 👍🏽 👨‍👩‍👧‍👦 🇯🇵 1️⃣ ❤️ 👩🏽‍💻 END\r\nText: ❤︎  Emoji: ❤️\r\n\x1b[1;3mStyled: é ♞\x1b[0m")},
	{"unicode-spacing", func(c *tape.Config) { c.FontSize, c.LineHeight, c.LetterSpacing = 14.5, 1.4, 2 }, text("┌──┬──┐ ╔══╦══╗ ╭──╮\r\n│  │  │ ║  ║  ║ │  │\r\n└──┴──┘ ╚══╩══╝ ╰──╯\r\n⣿⢕⡪ ▁▂▃▄▅▆▇█ 😀👍🏽👩🏽‍💻")},
	{"text-styles", nil, text("plain \x1b[1mbold\x1b[0m \x1b[3mitalic\x1b[0m \x1b[1;3mbold-italic\x1b[0m\r\n" +
		"\x1b[4munderline\x1b[0m \x1b[9mstrike\x1b[0m \x1b[2mfaint\x1b[0m \x1b[7minverse\x1b[0m\r\n" +
		"\x1b[31mred\x1b[32m green\x1b[33m yellow\x1b[34m blue\x1b[35m magenta\x1b[36m cyan\x1b[0m\r\n" +
		"\x1b[91mbright\x1b[92m colors\x1b[93m here\x1b[0m \x1b[38;5;208morange256\x1b[0m\r\n" + sgrRamp() +
		"The quick brown fox jumps over the lazy dog 0123456789 {}[]()<>/\\|!@#$%^&*~`'\"\r\n")},
	{"box-blocks-braille-wide", nil, text("┌──┬──┐ ╔══╦══╗ ╭──╮\r\n│  │  │ ║  ║  ║ │  │\r\n└──┴──┘ ╚══╩══╝ ╰──╯\r\n" +
		"▀▄█▌▐ ░▒▓ ▁▂▃▄▅▆▇█ ⠁⠃⠇⡇⣿⢕⡪\r\n日本語 한국어 中文 combining: é ä ñ\r\n" +
		"fallbacks:   ☃ éè üñ ΔΣ ЖЩ\r\n")},
	{"window-decorations", func(c *tape.Config) {
		c.Theme, c.Padding, c.Margin, c.MarginFill = "Dracula", 12, 16, "#44475a"
		c.WindowBar, c.WindowBarSize, c.BorderRadius = "colorful", 30, 10
	}, text("$ echo themed\r\nthemed\r\n\x1b[31mred\x1b[32m green\x1b[34m blue\x1b[0m\r\n$ ")},
	{"window-rings-right", func(c *tape.Config) {
		c.Theme, c.Margin, c.WindowBar, c.WindowBarSize, c.BorderRadius = "Catppuccin Mocha", 20, "ringsright", 36, 14
	}, text("$ ls\r\nREADME.md  main.go\r\n$ ")},
	{"font-size-fractional", func(c *tape.Config) { c.FontSize, c.Cols, c.Rows = 14.5, 40, 4 }, text("Hinting at 14.5px: iIlL1| oO0 gqy\r\n\x1b[1mBold\x1b[0m \x1b[3mItalic\x1b[0m\r\n")},
	{"font-size-large", func(c *tape.Config) { c.FontSize, c.Cols, c.Rows = 31, 24, 3 }, text("Large 31px rWgj\r\n\x1b[1;3mBold italic\x1b[0m\r\n")},
	// 96px is the largest size; curve-flattening drift grows with glyph size.
	{"font-size-max", func(c *tape.Config) { c.FontSize, c.Cols, c.Rows = 96, 8, 2 }, text("Og&@\r\n\x1b[1;3mRgQ8\x1b[0m")},
	{"spacing", func(c *tape.Config) { c.LineHeight, c.LetterSpacing, c.Cols, c.Rows = 1.4, 2, 30, 4 }, text("line height 1.4 spacing 2\r\nsecond line\r\n")},
	{"cursor-block", nil, text("$ cursor\x1b[2 q")},
	{"cursor-bar", nil, text("$ cursor\x1b[6 q")},
	{"cursor-underline", nil, text("$ cursor\x1b[4 q")},
	{"kitty-png", nil, func(t *testing.T) string {
		b, err := os.ReadFile("testdata/kitty.apc")
		if err != nil {
			t.Fatal(err)
		}
		return string(b) + "\x1b[8;1Hbeside the image\r\n"
	}},
	// Its input is built in goldenPlaceholders, which needs the renderer's cell size.
	{"kitty-placeholders", nil, nil},
}

// frameHash covers the dimensions and every pixel byte.
func frameHash(f *image.RGBA) string {
	h := sha256.New()
	b := f.Bounds()
	_ = binary.Write(h, binary.LittleEndian, [2]int32{int32(b.Dx()), int32(b.Dy())})
	for y := b.Min.Y; y < b.Max.Y; y++ {
		h.Write(f.Pix[y*f.Stride : y*f.Stride+b.Dx()*4])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func renderScenario(t *testing.T, s scenario) *image.RGBA {
	t.Helper()
	c := tape.DefaultConfig()
	c.Cols, c.Rows = 40, 12
	if s.config != nil {
		s.config(&c)
	}
	r, err := New(c)
	if err != nil {
		t.Fatalf("%s: %v", s.name, err)
	}
	defer r.Close()
	if !strings.HasPrefix(s.name, "cursor-") {
		write(t, r, "\x1b[?25l")
	}
	if s.name == "kitty-placeholders" {
		goldenPlaceholders(t, r)
	} else {
		write(t, r, s.input(t))
	}
	// A fixed instant: no blinking, no dependence on the wall clock.
	f, err := r.FrameAt(r.blinkStart)
	if err != nil {
		t.Fatalf("%s: %v", s.name, err)
	}
	return f
}

func goldenPlaceholders(t *testing.T, r *Renderer) {
	t.Helper()
	w, h := r.CellWidth, r.CellHeight
	im := image.NewNRGBA(image.Rect(0, 0, 4*w, 2*h))
	for y := 0; y < 2*h; y++ {
		for x := 0; x < 4*w; x++ {
			im.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / (4 * w)), G: uint8(y * 255 / (2 * h)), B: 200, A: uint8(128 + x*127/(4*w))})
		}
	}
	virtualPNG(t, r, 43, 1, im, 4, 2)
	write(t, r, "\x1b[38;2;0;0;43m")
	for row := 0; row < 2; row++ {
		for col := 0; col < 4; col++ {
			write(t, r, placeholderText(row, col))
		}
		write(t, r, "\r\n")
	}
	write(t, r, "\x1b[0mimage above")
}

func readGolden(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	b, err := os.ReadFile(goldenFile)
	if os.IsNotExist(err) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if name, sum, ok := strings.Cut(strings.TrimSpace(line), " "); ok && !strings.HasPrefix(name, "#") {
			out[name] = sum
		}
	}
	return out
}

func TestGoldenFrames(t *testing.T) {
	t.Setenv("NTRECORD_FONT_PATH", "")
	update := os.Getenv("NTRECORD_UPDATE_GOLDEN") == "1"
	dump := os.Getenv("NTRECORD_GOLDEN_DUMP")
	want := readGolden(t)
	got := map[string]string{}
	for _, s := range scenarios {
		f := renderScenario(t, s)
		got[s.name] = frameHash(f)
		if dump != "" {
			if err := os.MkdirAll(dump, 0o755); err != nil {
				t.Fatal(err)
			}
			out, err := os.Create(filepath.Join(dump, s.name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err = png.Encode(out, f); err != nil {
				t.Fatal(err)
			}
			out.Close()
		}
	}
	if update {
		var names []string
		for name := range got {
			names = append(names, name)
		}
		sort.Strings(names)
		var b strings.Builder
		b.WriteString("# sha256 of each golden frame (see golden_frames_test.go); regenerate with NTRECORD_UPDATE_GOLDEN=1\n")
		for _, name := range names {
			fmt.Fprintf(&b, "%s %s\n", name, got[name])
		}
		if err := os.WriteFile(goldenFile, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, s := range scenarios {
		switch w, ok := want[s.name]; {
		case !ok:
			t.Errorf("%s: no golden hash; run with NTRECORD_UPDATE_GOLDEN=1 and review the pictures", s.name)
		case w != got[s.name]:
			t.Errorf("%s: frame changed\n  want %s\n   got %s\nSet NTRECORD_GOLDEN_DUMP=dir to see the pictures. On one platform only, this is cross-architecture drift.", s.name, w, got[s.name])
		}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("golden hash for removed scenario %s", name)
		}
	}
}
