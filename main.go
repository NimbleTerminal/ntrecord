package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"
	"nimbleterminal.dev/ntrecord/internal/doctor"
	"nimbleterminal.dev/ntrecord/internal/encode"
	"nimbleterminal.dev/ntrecord/internal/record"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

// version is stamped by GoReleaser and task build VERSION=... .
var version = "dev"

func main() {
	// A closed stdout pipe must return EPIPE so Run can save captured frames.
	// Notify also leaves the normal SIGPIPE disposition for exec'd children.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	defer signal.Stop(sigpipe)
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ntrecord:", err)
		os.Exit(1)
	}
}

const usage = `Usage: ntrecord [-mode realtime|deterministic] <file.tape>
       ntrecord record [-o output] [-shell executable] [-cols n] [-rows n] [-font-size pixels] [-- command args...]
       ntrecord play <file.cast> [-paused] [-speed n]
       ntrecord render <file.cast> -o <output> [-font-size pixels] [-fps n] [-speed n]
       ntrecord validate <file.tape>...
       ntrecord doctor
       ntrecord skill show|install [FOLDER]
       ntrecord --version

-mode picks the clock for a tape. realtime (the default) samples the screen as time
passes and captures animations. deterministic runs a virtual clock: each step is
held as long as the tape says, once the program settles, so the same tape gives
the same recording and Sleep costs no real time. Use Wait for anything slow.

validate checks tapes (syntax, fonts, themes, Require, outputs) without recording.
doctor checks that this machine can record.
skill show prints the skill that teaches an agent to use ntrecord; skill install
installs it for the agents on this machine.`

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		fmt.Fprintln(stdout, "ntrecord", version)
		return nil
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(stdout, usage)
		return nil
	}
	switch args[0] {
	case "validate":
		return validate(args[1:], stdout, stderr)
	case "skill":
		return skillCommand(args[1:], stdout, stderr)
	case "doctor":
		if !doctor.Run(stdout, doctor.Options{Version: version}) {
			return errors.New("doctor found problems")
		}
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if args[0] == "play" {
		return playCast(ctx, args[1:], stderr)
	}
	if args[0] == "render" {
		return renderCast(ctx, args[1:], stderr)
	}
	opts := record.Options{Config: tape.DefaultConfig(), Warn: stderr}
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
		opts.Command, opts.Input, opts.Output = fs.Args(), os.Stdin, stdout
		if term.IsTerminal(int(os.Stdin.Fd())) {
			state, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			defer term.Restore(int(os.Stdin.Fd()), state)
		}
	} else {
		fs := flag.NewFlagSet("ntrecord", flag.ContinueOnError)
		fs.SetOutput(stderr)
		modeFlag := fs.String("mode", string(tape.Realtime), "clock: realtime or deterministic")
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("expected one tape path")
		}
		mode, err := tape.ParseMode(*modeFlag)
		if err != nil {
			return err
		}
		t, err := tape.ParseFile(fs.Arg(0))
		if err != nil {
			return err
		}
		opts.Config, opts.Tape = t.Config, t
		opts.Config.Version = version
		opts.Config.Mode = mode
	}
	err := record.Run(ctx, opts)
	if err != nil {
		return err
	}
	for _, path := range opts.Config.Destinations() {
		fmt.Fprintln(stderr, "Wrote", path)
	}
	return nil
}

// validate checks each tape without recording and reports every problem, so one
// run shows all that needs fixing.
func validate(paths []string, stdout, stderr io.Writer) error {
	if len(paths) == 0 {
		return errors.New("validate: expected one or more tape paths")
	}
	failed := 0
	for _, path := range paths {
		t, err := tape.ParseFile(path)
		if err != nil { // already names the file, line and column
			fmt.Fprintln(stderr, err)
			failed++
			continue
		}
		t.Config.Version = version
		s, err := record.Validate(t)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", path, err)
			failed++
			continue
		}
		fmt.Fprintf(stdout, "%s: ok — %d commands · %dx%d cells, %dx%d px · outputs: %s\n",
			path, s.Commands, s.Cols, s.Rows, s.Width, s.Height, strings.Join(s.Outputs, ", "))
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d tapes invalid", failed, len(paths))
	}
	return nil
}
