package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NimbleMarkets/sigmamax/internal/encode"
	"github.com/NimbleMarkets/sigmamax/internal/record"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
	"golang.org/x/term"
)

// version is stamped by GoReleaser and task build VERSION=... .
var version = "dev"

func main() {
	// A closed stdout pipe must return EPIPE so Run can save captured frames.
	// Notify also leaves the normal SIGPIPE disposition for exec'd children.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	defer signal.Stop(sigpipe)
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sigmamax:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		fmt.Println("sigmamax", version)
		return nil
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: sigmamax --version\n       sigmamax <file.tape>\n       sigmamax record [-o output] [-shell executable] [-cols n] [-rows n] [-font-size pixels] [-- command args...]")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opts := record.Options{Config: tape.DefaultConfig()}
	opts.Config.Version = version
	if args[0] == "record" {
		fs := flag.NewFlagSet("record", flag.ContinueOnError)
		fs.StringVar(&opts.Config.Output, "o", opts.Config.Output, encode.OutputHelp())
		fs.StringVar(&opts.Config.Shell, "shell", opts.Config.Shell, "interactive shell executable")
		fs.IntVar(&opts.Config.Cols, "cols", opts.Config.Cols, "PTY columns")
		fs.IntVar(&opts.Config.Rows, "rows", opts.Config.Rows, "PTY rows")
		fs.Float64Var(&opts.Config.FontSize, "font-size", opts.Config.FontSize, "font size in pixels")
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
		opts.Config.Version = version
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
