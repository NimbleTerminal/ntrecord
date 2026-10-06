package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NimbleMarkets/sigmamax/internal/record"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
	"golang.org/x/term"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sigmamax:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: sigmamax <file.tape>\n       sigmamax record [-o recording.gif] [-cols 80] [-rows 24] [-font-size 18] [-- command args...]")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opts := record.Options{Config: tape.DefaultConfig()}
	if args[0] == "record" {
		fs := flag.NewFlagSet("record", flag.ContinueOnError)
		fs.StringVar(&opts.Config.Output, "o", "recording.gif", "output (.gif, .mp4, .webm, .txt, .ascii, or frames/; video requires ffmpeg)")
		fs.StringVar(&opts.Config.Shell, "shell", "/bin/sh", "interactive shell executable")
		fs.IntVar(&opts.Config.Cols, "cols", 80, "PTY columns")
		fs.IntVar(&opts.Config.Rows, "rows", 24, "PTY rows")
		fs.Float64Var(&opts.Config.FontSize, "font-size", 18, "font size in pixels")
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		opts.Command, opts.Input, opts.Output = fs.Args(), os.Stdin, os.Stdout
		if term.IsTerminal(int(os.Stdin.Fd())) {
			state, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			defer term.Restore(int(os.Stdin.Fd()), state)
		}
	} else {
		if len(args) != 1 {
			return fmt.Errorf("expected one tape path")
		}
		t, err := tape.ParseFile(args[0])
		if err != nil {
			return err
		}
		opts.Config, opts.Tape = t.Config, t
	}
	err := record.Run(ctx, opts)
	if err != nil {
		return err
	}
	for _, path := range opts.Config.Destinations() {
		fmt.Fprintln(os.Stderr, "Wrote", path)
	}
	return nil
}
