// Package renderconfig adapts CLI/tape settings to the public renderer API.
// Keeping this adapter outside tape lets parsing remain independent of cgo.
package renderconfig

import (
	"nimbleterminal.dev/ntrecord/internal/tape"
	"nimbleterminal.dev/ntrecord/render"
)

func FromTape(c tape.Config) render.Config {
	return render.Config{
		Cols: c.Cols, Rows: c.Rows, Width: c.Width, Height: c.Height,
		FontSize: c.FontSize, FontFamily: c.FontFamily, LetterSpacing: c.LetterSpacing, LineHeight: c.LineHeight,
		Padding: c.Padding, Margin: c.Margin, BorderRadius: c.BorderRadius, WindowBarSize: c.WindowBarSize,
		WindowBar: c.WindowBar, MarginFill: c.MarginFill, Theme: c.Theme, CursorBlink: c.CursorBlink, Version: c.Version,
	}
}
