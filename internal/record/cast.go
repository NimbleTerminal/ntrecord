package record

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nimbleterminal.dev/ntrecord/asciicast"
	"nimbleterminal.dev/ntrecord/internal/encode"
	"nimbleterminal.dev/ntrecord/internal/tape"
	"nimbleterminal.dev/ntrecord/render"
)

type castOutput struct {
	writer *asciicast.Writer
	file   *os.File
	path   string
	err    error
}

func newCastOutput(path string, r *render.Renderer) (*castOutput, error) {
	fg, bg, palette, err := r.TerminalColors()
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ntrecord-*.cast")
	if err != nil {
		return nil, err
	}
	c := &castOutput{file: f, path: path}
	c.writer, err = asciicast.NewWriter(f, asciicast.Terminal{Cols: r.Cols, Rows: r.Rows, Theme: &asciicast.Theme{FG: fg, BG: bg, Palette: strings.Join(palette, ":")}})
	if err != nil {
		c.abort()
		return nil, err
	}
	// Record the initial width convention for other mode-2027-aware viewers.
	if err = c.writer.Output(0, []byte("\x1b[?2027h")); err != nil {
		c.abort()
		return nil, err
	}
	return c, nil
}

func (c *castOutput) output(at time.Duration, b []byte) error {
	c.err = errors.Join(c.err, c.writer.Output(at, b))
	return c.err
}

func (c *castOutput) finish(at time.Duration) error {
	if c.err != nil {
		return c.err
	}
	if err := c.writer.End(at); err != nil {
		return err
	}
	if err := c.file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(c.file.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(c.file.Name(), c.path)
}

func (c *castOutput) abort() {
	if c.file != nil {
		c.file.Close()
		os.Remove(c.file.Name())
	}
}

func validateCast(c tape.Config, t *tape.Tape) error {
	hasCast := false
	for _, path := range c.Destinations() {
		format, _ := encode.Format(path)
		hasCast = hasCast || format == "cast"
	}
	if !hasCast || t == nil {
		return nil
	}
	for _, a := range t.Actions {
		switch a.Command {
		case "Hide", "ScrollUp", "ScrollDown":
			return fmt.Errorf(".cast output does not support %s: terminal event recordings cannot reproduce hidden output or viewport scrolling", a.Command)
		}
	}
	return nil
}
