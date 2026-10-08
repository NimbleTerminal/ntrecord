package render

import (
	"image/color"
	"slices"
	"testing"

	"nimbleterminal.dev/ntrecord/internal/tape"
)

func TestPaletteSeedCarriesThemeColors(t *testing.T) {
	c := tape.DefaultConfig()
	c.Theme = "Catppuccin Mocha"
	c.MarginFill = "#112233"
	c.Margin = 4
	c.WindowBar, c.WindowBarSize = "colorful", 30
	r, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	seed := r.PaletteSeed()
	// The background and foreground lead, so they win when the palette is full.
	if len(seed) < 2 || seed[0] != (color.RGBA{0x1e, 0x1e, 0x2e, 255}) || seed[1] != (color.RGBA{0xcd, 0xd6, 0xf4, 255}) {
		t.Fatalf("seed must start with theme background and foreground: %v", seed)
	}
	for name, want := range map[string]color.RGBA{
		"ANSI red":    {0xf3, 0x8b, 0xa8, 255},
		"cursor":      {0xf5, 0xe0, 0xdc, 255},
		"MarginFill":  {0x11, 0x22, 0x33, 255},
		"window dot":  {255, 95, 86, 255},
		"window dot2": {39, 201, 63, 255},
	} {
		if !slices.Contains(seed, want) {
			t.Errorf("seed lacks %s %v", name, want)
		}
	}
}

func TestPaletteSeedDefaultsWithoutTheme(t *testing.T) {
	r, err := New(tape.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	seed := r.PaletteSeed()
	if len(seed) < 2 || seed[0] != (color.RGBA{24, 26, 32, 255}) || seed[1] != (color.RGBA{224, 228, 235, 255}) {
		t.Fatalf("default background/foreground not first: %v", seed)
	}
	if slices.Contains(seed, color.RGBA{255, 95, 86, 255}) {
		t.Error("window-bar colors included although no window bar is drawn")
	}
}
