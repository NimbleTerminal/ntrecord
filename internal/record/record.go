// Package record runs a fixed-size PTY, serializes VT access, and samples frames.
package record

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/encode"
	"github.com/NimbleMarkets/sigmamax/internal/render"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
	"github.com/creack/pty"
)

const framePeriod = encode.FramePeriod

type Options struct {
	Config  tape.Config
	Command []string
	// A non-nil Tape selects scripted mode, even when it contains no actions.
	Tape *tape.Tape
	// Live input/output are forwarded unchanged to/from the PTY.
	Input  io.Reader
	Output io.Writer
}

type output struct {
	data []byte
	err  error
}
type control struct {
	command string
	ack     chan struct{}
}

func Run(parent context.Context, opts Options) error {
	r, err := render.New(opts.Config)
	if err != nil {
		return err
	}
	defer r.Close()
	encoder, err := encode.New(opts.Config.Output, r.Width, r.Height)
	if err != nil {
		return err
	}
	defer encoder.Abort()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	args := opts.Command
	if len(args) == 0 {
		args = []string{opts.Config.Shell, "-i"}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = environment(os.Environ())
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(r.Cols), Rows: uint16(r.Rows), X: uint16(r.Width), Y: uint16(r.Height)})
	if err != nil {
		return fmt.Errorf("start PTY: %w", err)
	}
	exit := make(chan struct{})
	var exitErr error
	go func() { exitErr = cmd.Wait(); close(exit) }()
	read := make(chan output, 32)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			o := output{data: append([]byte(nil), buf[:n]...), err: err}
			select {
			case read <- o:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	writes := make(chan []byte, 128)
	writeErrors := make(chan error, 1)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for {
			select {
			case b := <-writes:
				if _, err := ptmx.Write(b); err != nil {
					select {
					case writeErrors <- err:
					case <-ctx.Done():
					}
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	defer func() {
		cancel()
		_ = ptmx.Close() // sends SIGHUP to the controlling foreground group
		_ = cmd.Process.Kill()
		<-readDone
		<-writeDone
		// Wait always reaps the child, including a tape's still-running shell.
		select {
		case <-exit:
		case <-time.After(time.Second):
		}
	}()
	controls := make(chan control)
	scriptDone := make(chan error, 1)
	if opts.Tape != nil {
		go func() { scriptDone <- play(ctx, opts.Tape.Actions, writes, controls) }()
	} else if opts.Input != nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := opts.Input.Read(buf)
				if n > 0 {
					select {
					case writes <- append([]byte(nil), buf[:n]...):
					case <-ctx.Done():
						return
					}
				}
				if err != nil {
					// EOF on piped input behaves like pressing Ctrl-D.
					select {
					case writes <- []byte{4}:
					case <-ctx.Done():
					}
					return
				}
			}
		}()
	}
	visible := true
	last := time.Now()
	capture := func(now time.Time, final bool) error {
		if !visible {
			last = now
			return nil
		}
		if !final && r.RenderHeld(now) {
			return nil
		}
		frame, err := r.Frame()
		if err != nil {
			return err
		}
		d := now.Sub(last)
		last = now
		return encoder.Add(frame, d)
	}
	ticker := time.NewTicker(framePeriod)
	defer ticker.Stop()
	finished := false
	var runErr error
	for !finished {
		select {
		case o := <-read:
			if len(o.data) > 0 {
				if err := r.Write(o.data); err != nil {
					return err
				}
				if reply := r.TakeReplies(); len(reply) > 0 {
					select {
					case writes <- reply:
					case <-ctx.Done():
						finished = true
					}
				}
				if opts.Output != nil {
					if _, err := opts.Output.Write(o.data); err != nil {
						return err
					}
				}
			}
			if o.err != nil {
				if !errors.Is(o.err, io.EOF) && !errors.Is(o.err, syscall.EIO) {
					runErr = o.err
				}
				// PTY EOF is processed after every final output byte.
				select {
				case <-exit:
					if runErr == nil {
						runErr = exitErr
					}
				case <-ctx.Done():
					runErr = ctx.Err()
				}
				finished = true
			}
		case c := <-controls:
			if c.command == "Hide" && visible {
				if err := capture(time.Now(), false); err != nil {
					return err
				}
			}
			visible = c.command == "Show"
			last = time.Now()
			close(c.ack)
		case err := <-scriptDone:
			runErr = err
			finished = true
		case err := <-writeErrors:
			runErr = err
			finished = true
		case <-ticker.C:
			if err := capture(time.Now(), false); err != nil {
				return err
			}
		case <-ctx.Done():
			runErr = ctx.Err()
			finished = true
		}
	}
	// A client may exit during a hold. Still finalize its last viewport.
	if err := capture(time.Now().Add(framePeriod), true); err != nil {
		return err
	}
	if err := encoder.Finish(); err != nil {
		return err
	}
	return runErr
}

func play(ctx context.Context, actions []tape.Action, writes chan<- []byte, controls chan<- control) error {
	wait := func(d time.Duration) error {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-t.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	write := func(b []byte) error {
		select {
		case writes <- b:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	started := false
	for _, a := range actions {
		// Apply leading Hide immediately; allow the shell to initialize before
		// the first input, without leaking hidden setup into an initial frame.
		if !started && a.Command != "Hide" && a.Command != "Show" {
			if err := wait(100 * time.Millisecond); err != nil {
				return err
			}
			started = true
		}
		switch a.Command {
		case "Type":
			for _, ch := range a.Text {
				if err := write([]byte(string(ch))); err != nil {
					return err
				}
				if err := wait(25 * time.Millisecond); err != nil {
					return err
				}
			}
		case "Enter":
			if err := write([]byte{'\r'}); err != nil {
				return err
			}
			if err := wait(100 * time.Millisecond); err != nil {
				return err
			}
		case "Sleep":
			if err := wait(a.Duration); err != nil {
				return err
			}
		case "Hide", "Show":
			c := control{a.Command, make(chan struct{})}
			select {
			case controls <- c:
			case <-ctx.Done():
				return ctx.Err()
			}
			select {
			case <-c.ack:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

func environment(base []string) []string {
	var env []string
	for _, s := range base {
		key, _, _ := strings.Cut(s, "=")
		switch key {
		case "TERM", "COLORTERM", "TERM_PROGRAM", "TERM_PROGRAM_VERSION", "PS1", "PS2":
			continue
		}
		env = append(env, s)
	}
	return append(env, "TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=sigmamax", "TERM_PROGRAM_VERSION=0.1", "PS1=$ ", "PS2=> ")
}
