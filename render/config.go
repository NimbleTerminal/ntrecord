package render

import (
	"fmt"
	"math"
	"strings"

	"nimbleterminal.dev/ntrecord/internal/theme"
)

// Config describes a fixed-size terminal and its rendered appearance. Start
// with DefaultConfig; the zero value is not a valid configuration. New copies
// the configuration. Later changes to it do not affect the renderer.
type Config struct {
	// Cols and Rows are the grid dimensions, each between 1 and 8192.
	Cols, Rows int
	// Width and Height, when nonzero, override the corresponding grid dimension
	// with an output size in pixels, including decorations. Output is limited to
	// 8192 pixels per side and 16 megapixels.
	Width, Height int
	// FontSize is in pixels, from 6 to 96. FontFamily is an installed family or
	// font file path; empty selects bundled Go Mono.
	FontSize   float64
	FontFamily string
	// LetterSpacing adds pixels to each cell. LineHeight multiplies its height;
	// zero means 1. Both must leave a positive cell size.
	LetterSpacing, LineHeight float64
	// Decoration dimensions are in pixels and must be nonnegative.
	Padding, Margin, BorderRadius, WindowBarSize int
	// WindowBar is empty, none, colorful, colorfulright, rings, or ringsright.
	WindowBar string
	// MarginFill is a color accepted by the theme parser, e.g. "#44475a".
	MarginFill string
	// Theme is a bundled theme name or a JSON object containing theme colors.
	Theme string
	// CursorBlink enables cursor blinking on the supplied render timeline.
	CursorBlink bool
	// Version is advertised as "ntrecord <version>" in terminal queries.
	// A leading v is stripped; empty reports dev.
	Version string
}

// DefaultConfig returns an 80x24 Go Mono terminal at 18 pixels, without decorations.
func DefaultConfig() Config {
	return Config{Cols: 80, Rows: 24, FontSize: 18, FontFamily: "Go Mono", LineHeight: 1, WindowBarSize: 30}
}

func (c Config) programVersion() string {
	if v := strings.TrimPrefix(c.Version, "v"); v != "" {
		return v
	}
	return "dev"
}

func (c Config) validate() error {
	if !(c.FontSize >= 6 && c.FontSize <= 96) {
		return fmt.Errorf("font size must be between 6 and 96")
	}
	for name, v := range map[string]int{"width": c.Width, "height": c.Height, "padding": c.Padding, "margin": c.Margin, "border radius": c.BorderRadius, "window bar size": c.WindowBarSize} {
		if v < 0 || v > 8192 {
			return fmt.Errorf("%s must be between 0 and 8192", name)
		}
	}
	if math.IsNaN(c.LineHeight) || math.IsInf(c.LineHeight, 0) || c.LineHeight < 0 || c.LineHeight > 8192 || math.IsNaN(c.LetterSpacing) || math.IsInf(c.LetterSpacing, 0) || math.Abs(c.LetterSpacing) > 8192 {
		return fmt.Errorf("font spacing must be finite and within pixel limits")
	}
	switch c.WindowBar {
	case "", "none", "colorful", "colorfulright", "rings", "ringsright":
	default:
		return fmt.Errorf("unknown window bar %q", c.WindowBar)
	}
	if c.MarginFill != "" {
		if _, err := theme.Color(c.MarginFill); err != nil {
			return err
		}
	}
	return nil
}
