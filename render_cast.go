package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/NimbleMarkets/sigmamax/internal/record"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
)

func renderCast(ctx context.Context, args []string, stderr io.Writer) error {
	c := tape.DefaultConfig()
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&c.Output, "o", "", "destination GIF, MP4, WebM, text file, or frames/")
	fs.Float64Var(&c.FontSize, "font-size", c.FontSize, "font size in pixels")
	fs.StringVar(&c.FontFamily, "font", c.FontFamily, "font family or TTF/OTF path")
	fs.StringVar(&c.Theme, "theme", "", "override the recorded theme")
	fs.Float64Var(&c.Framerate, "fps", c.Framerate, "output frames per second (1–120)")
	fs.Float64Var(&c.PlaybackSpeed, "speed", 1, "playback speed multiplier")
	fs.BoolVar(&c.CursorBlink, "cursor-blink", false, "animate cursor blinking")
	// Accept both render file.cast -o out.gif and render -o out.gif file.cast.
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		args = append(append([]string(nil), args[1:]...), args[0])
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 || c.Output == "" {
		return fmt.Errorf("usage: sigmamax render <file.cast> -o <output> [options]")
	}
	src, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer src.Close()
	inputPath, _ := filepath.Abs(fs.Arg(0))
	outputPath, _ := filepath.Abs(c.Output)
	if inputPath == outputPath {
		return fmt.Errorf("cast input and output must be different files")
	}
	if info, err := os.Stat(c.Output); err == nil {
		original, err := src.Stat()
		if err != nil {
			return err
		}
		if os.SameFile(original, info) {
			return fmt.Errorf("cast input and output must be different files")
		}
	}
	if err := record.RenderCast(ctx, src, c); err != nil {
		return err
	}
	fmt.Fprintln(stderr, "Wrote", c.Output)
	return nil
}
