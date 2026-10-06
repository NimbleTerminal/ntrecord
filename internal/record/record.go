// Package record runs a fixed-size PTY, serializes VT access, and samples frames.
package record

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/NimbleMarkets/sigmamax/internal/render"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
	"github.com/creack/pty"
)

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
	action tape.Action
	ack    chan error
}

func Run(parent context.Context, opts Options) error {
	r, err := render.New(opts.Config)
	if err != nil {
		return err
	}
	defer r.Close()
	rate := opts.Config.Framerate
	if rate == 0 {
		rate = 20
	}
	speed := opts.Config.PlaybackSpeed
	if speed == 0 {
		speed = 1
	}
	period := time.Duration(float64(time.Second) / rate)
	env := environment(os.Environ())
	for key, value := range opts.Config.Env {
		env = replaceEnv(env, key, value)
	}
	for _, program := range opts.Config.Require {
		if _, err := lookPath(program, env); err != nil {
			return fmt.Errorf("Require %s: %w", program, err)
		}
	}
	encoder, err := newOutputs(opts.Config, r)
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
	executable, err := lookPath(args[0], env)
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, args[1:]...)
	cmd.Env = env
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
		return encoder.Add(frame, time.Duration(float64(d)/speed))
	}
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	finished := false
	var runErr error
	var pending *control
	var waitDeadline <-chan time.Time
	var waitTimer *time.Timer
	defer func() {
		if waitTimer != nil {
			waitTimer.Stop()
		}
	}()
	var waitRE *regexp.Regexp
	checkWait := func() {
		if pending == nil {
			return
		}
		text, err := r.Text(pending.action.Scope)
		if err != nil || waitRE.MatchString(text) {
			pending.ack <- err
			pending = nil
			waitDeadline = nil
			if waitTimer != nil {
				waitTimer.Stop()
			}
		}
	}
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
			checkWait()
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
			a := c.action
			var actionErr error
			var input []byte
			switch a.Command {
			case "Hide", "Show":
				if a.Command == "Hide" && visible {
					actionErr = capture(time.Now(), false)
				}
				visible = a.Command == "Show"
				last = time.Now()
			case "Checkpoint":
				if visible {
					actionErr = encoder.Text(r)
				}
			case "Wait":
				pending = &c
				waitRE = regexp.MustCompile(a.Text)
				waitTimer = time.NewTimer(a.Duration)
				waitDeadline = waitTimer.C
				checkWait()
				continue
			case "Screenshot":
				var frameErr error
				frame, frameErr := r.Frame()
				if frameErr != nil {
					actionErr = frameErr
				} else {
					actionErr = writeAtomic(a.Text, func(w io.Writer) error { return png.Encode(w, frame) })
				}
			case "Text":
				cp, _ := utf8.DecodeRuneInString(a.Text)
				input, actionErr = r.TypeRune(cp)
			case "Paste":
				input, actionErr = r.Paste(a.Text)
			case "ScrollUp":
				actionErr = r.Scroll(-1)
			case "ScrollDown":
				actionErr = r.Scroll(1)
			default:
				input, actionErr = r.Key(a.Command)
			}
			if actionErr == nil && len(input) > 0 {
				select {
				case writes <- input:
				case <-ctx.Done():
					actionErr = ctx.Err()
				}
			}
			c.ack <- actionErr
		case <-waitDeadline:
			text, _ := r.Text(pending.action.Scope)
			if len(text) > 500 {
				text = text[len(text)-500:]
			}
			pending.ack <- fmt.Errorf("Wait+%s timed out after %s waiting for /%s/; last text: %q", pending.action.Scope, pending.action.Duration, pending.action.Text, text)
			pending = nil
			waitDeadline = nil

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
	if err := capture(time.Now().Add(period), true); err != nil {
		return err
	}
	if visible {
		if err := encoder.Text(r); err != nil {
			return errors.Join(runErr, err)
		}
	}
	if err := encoder.Finish(); err != nil {
		return errors.Join(runErr, err)
	}
	return runErr
}

func play(ctx context.Context, actions []tape.Action, writes chan<- []byte, controls chan<- control) error {
	wait := func(d time.Duration) error {
		if d == 0 {
			return ctx.Err()
		}
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	control := func(a tape.Action) error {
		c := control{action: a, ack: make(chan error, 1)}
		select {
		case controls <- c:
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case err := <-c.ack:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	started := false
	clipboard := ""
	for _, a := range actions {
		if !started && a.Command != "Hide" && a.Command != "Show" {
			if err := wait(100 * time.Millisecond); err != nil {
				return a.Error(err)
			}
			started = true
		}
		var err error
		switch a.Command {
		case "Type":
			for _, ch := range a.Text {
				input := a
				input.Command = "Text"
				input.Text = string(ch)
				if err = control(input); err != nil {
					break
				}
				if err = wait(a.Interval); err != nil {
					break
				}
			}
		case "Sleep":
			err = wait(a.Duration)
		case "Copy":
			clipboard = a.Text
		case "Paste":
			a.Text = clipboard
			err = control(a)
		case "Wait", "Screenshot", "Hide", "Show":
			err = control(a)
		default:
			for range max(a.Count, 1) {
				if err = control(a); err != nil {
					break
				}
				if err = wait(a.Interval); err != nil {
					break
				}
			}
		}
		if err == nil {
			checkpoint := a
			checkpoint.Command = "Checkpoint"
			err = control(checkpoint)
		}
		if err != nil {
			return a.Error(err)
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

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, s := range env {
		if strings.HasPrefix(s, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func lookPath(name string, env []string) (string, error) {
	if strings.ContainsRune(name, '/') {
		return exec.LookPath(name)
	}
	path := ""
	for _, s := range env {
		if strings.HasPrefix(s, "PATH=") {
			path = strings.TrimPrefix(s, "PATH=")
		}
	}
	for _, dir := range filepath.SplitList(path) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("executable %q not found in recording PATH", name)
}
