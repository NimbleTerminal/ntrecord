package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"nimbleterminal.dev/ntrecord/internal/player"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

func playCast(ctx context.Context, args []string, stderr io.Writer) error {
	opts := player.Options{Config: tape.DefaultConfig()}
	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.Paused, "paused", false, "start paused")
	fs.Float64Var(&opts.Config.FontSize, "font-size", opts.Config.FontSize, "font size in pixels")
	fs.StringVar(&opts.Config.FontFamily, "font", opts.Config.FontFamily, "font family or TTF/OTF path")
	fs.StringVar(&opts.Config.Theme, "theme", "", "override the recorded theme")
	fs.Float64Var(&opts.Config.Framerate, "fps", opts.Config.Framerate, "display refresh rate (1–120)")
	fs.Float64Var(&opts.Config.PlaybackSpeed, "speed", 1, "initial playback speed (0.1–16)")
	fs.BoolVar(&opts.Config.CursorBlink, "cursor-blink", false, "animate cursor blinking")
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		args = append(append([]string(nil), args[1:]...), args[0])
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: ntrecord play <file.cast> [options]")
	}
	return player.Run(ctx, fs.Arg(0), opts)
}
