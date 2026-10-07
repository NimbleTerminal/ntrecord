// Package record runs a fixed-size PTY, serializes VT access, and samples frames.
package record

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
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
	"github.com/NimbleMarkets/sigmamax/internal/terminfo"
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
	// Warn receives advice about a recording that may not have captured what the
	// author meant, such as a deterministic tape that ended before its program
	// stopped drawing. Nil discards it.
	Warn io.Writer
}

type output struct {
	data []byte
	err  error
	at   time.Time
}
type control struct {
	action tape.Action
	ack    chan error
}

// PTY bytes and tape controls share one FIFO. A control cannot overtake output
// already queued by the reader, including at Hide/Show and end of tape.
type event struct {
	output
	control    *control
	scriptDone bool
}

type writeRequest struct {
	data []byte
	ack  chan error
}

type writeResult struct {
	request writeRequest
	err     error
}

// A deterministic recording waits for the program to settle after each step:
// quiet for settleQuiet, or settleLimit at most (a program that never stops
// drawing must use realtime mode). The first step also waits for any output,
// so a slow-starting shell is not recorded before its prompt.
const (
	settleQuiet = 30 * time.Millisecond
	settleLimit = time.Second
)

func Run(parent context.Context, opts Options) error {
	if err := validateCast(opts.Config, opts.Tape); err != nil {
		return err
	}
	virtual := opts.Config.Mode == tape.Deterministic
	if virtual && opts.Tape == nil {
		return errors.New("deterministic mode needs a tape: a live recording follows a person's clock")
	}
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
	// The recorded program's terminfo lives in a private directory, removed when
	// the recording ends. If it cannot be made, the program gets xterm-256color.
	terminfoDir, err := os.MkdirTemp("", "sigmamax-terminfo-")
	if err == nil {
		defer os.RemoveAll(terminfoDir)
		if err = terminfo.Install(terminfoDir); err != nil {
			terminfoDir = ""
		}
	} else {
		terminfoDir = ""
	}
	env := recordingEnv(opts.Config, terminfoDir)
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
	castStart := time.Now()
	exit := make(chan struct{})
	var exitErr error
	go func() { exitErr = cmd.Wait(); close(exit) }()
	events := make(chan event, 32)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			o := output{data: append([]byte(nil), buf[:n]...), err: err, at: time.Now()}
			select {
			case events <- event{output: o}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	writes := make(chan writeRequest)
	writeResults := make(chan writeResult, 1)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for {
			select {
			case w := <-writes:
				_, err := ptmx.Write(w.data)
				select {
				case writeResults <- writeResult{w, err}:
				case <-ctx.Done():
					return
				}
				if err != nil {
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
	liveInput := make(chan []byte)
	if opts.Tape != nil {
		go func() {
			err := play(ctx, opts.Tape.Actions, events, virtual)
			select {
			case events <- event{scriptDone: true, output: output{err: err}}:
			case <-ctx.Done():
			}
		}()
	} else if opts.Input != nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := opts.Input.Read(buf)
				if n > 0 {
					select {
					case liveInput <- append([]byte(nil), buf[:n]...):
					case <-ctx.Done():
						return
					}
				}
				if err != nil {
					// EOF on piped input behaves like pressing Ctrl-D.
					select {
					case liveInput <- []byte{4}:
					case <-ctx.Done():
					}
					return
				}
			}
		}()
	}
	visible := true
	last := time.Now()
	capture := func(captureCtx context.Context, now time.Time, final bool) error {
		if len(encoder.encoders) == 0 {
			last = now
			return nil
		}
		if !visible {
			last = now
			return nil
		}
		if !final && r.RenderHeld(now) {
			return nil
		}
		frame, err := r.CaptureFrameAt(now)
		if err != nil {
			return err
		}
		d := time.Duration(float64(now.Sub(last)) / speed)
		last = now
		if final {
			// Preserve a visibility boundary even when it arrives just after a
			// sample; encoder timing must not round the last viewport away.
			// GIF's minimum time unit is 10ms, even above 100 FPS.
			d = max(d, period, 10*time.Millisecond)
		}
		return encoder.Add(captureCtx, frame, d)
	}
	// Deterministic mode: frames are made when the tape asks for time to pass,
	// after the program settles, and held for exactly that long. recorded is the
	// virtual time already in the recording; it also drives the cursor's blink.
	var recorded time.Duration
	var lastEmitted *image.RGBA // the last frame held, to notice a program drawing after the tape ends
	emit := func(captureCtx context.Context, d time.Duration) error {
		if len(encoder.encoders) == 0 {
			if visible {
				recorded += d
			}
			return captureCtx.Err()
		}
		for d > 0 && visible {
			step := d
			if opts.Config.CursorBlink {
				step = min(d, render.BlinkHalfPeriod-recorded%render.BlinkHalfPeriod)
			}
			frame, err := r.FrameAfter(recorded)
			if err != nil {
				return err
			}
			if err = encoder.Add(captureCtx, frame, step); err != nil {
				return err
			}
			lastEmitted = frame
			recorded += step
			d -= step
		}
		return nil
	}
	var settle *control // a step waiting for the program to go quiet
	var settleThen func() error
	var settleQuietT, settleLimitT *time.Timer
	var settleQuietC, settleLimitC <-chan time.Time
	gotOutput := false
	defer func() {
		if settle != nil {
			settleQuietT.Stop()
			settleLimitT.Stop()
		}
	}()
	startSettle := func(c control, then func() error) {
		settle, settleThen = &c, then
		settleQuietT, settleLimitT = time.NewTimer(settleQuiet), time.NewTimer(settleLimit)
		settleQuietC, settleLimitC = settleQuietT.C, settleLimitT.C
	}
	finishSettle := func() {
		err := settleThen()
		settleQuietT.Stop()
		settleLimitT.Stop()
		settle.ack <- err
		settle, settleQuietC, settleLimitC = nil, nil, nil
	}
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	tick := ticker.C
	if virtual {
		tick = nil // no sampling: the tape's own steps make the frames
	}
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
	// Keep PTY backpressure out of the VT loop: it must keep consuming output
	// while a child writes before reading a paste. Bound queued replies so a
	// client that never reads cannot consume unbounded memory.
	var queue []writeRequest
	queuedBytes := 0
	writerClosed := false
	enqueue := func(w writeRequest) error {
		if writerClosed {
			return nil // wait for the reader's ordered EOF and child status
		}
		if queuedBytes+len(w.data) > 8*1024*1024 {
			return fmt.Errorf("PTY write queue exceeds 8 MiB: child is not reading input")
		}
		queue = append(queue, w)
		queuedBytes += len(w.data)
		return nil
	}
	// A successful tape gets a short tail drain for output generated by its
	// last input. Never wait indefinitely for an interactive shell to exit or
	// an animation to become quiet. Longer-running commands require Wait.
	var quiet, tailLimit *time.Timer
	var quietC, tailC <-chan time.Time
	defer func() {
		if quiet != nil {
			quiet.Stop()
			tailLimit.Stop()
		}
	}()
	for !finished {
		var send chan writeRequest
		var next writeRequest
		if len(queue) > 0 {
			send, next = writes, queue[0]
		}
		input := liveInput
		if writerClosed || queuedBytes >= 64*1024 {
			input = nil
		}
		select {
		case send <- next:
			queuedBytes -= len(next.data)
			queue[0] = writeRequest{}
			queue = queue[1:]
		case b := <-input:
			if err := enqueue(writeRequest{data: b}); err != nil {
				runErr, finished = err, true
			}
		case result := <-writeResults:
			if errors.Is(result.err, syscall.EIO) {
				// Slave closure can reach the writer before the reader delivers
				// its final bytes. Drain those bytes and use the child exit status.
				writerClosed = true
				queue, queuedBytes = nil, 0
				continue
			}
			if result.request.ack != nil {
				result.request.ack <- result.err
			}
			if result.err != nil {
				runErr, finished = result.err, true
			}
		case ev := <-events:
			if ev.scriptDone {
				runErr = ev.err
				if runErr != nil {
					finished = true
				} else {
					quiet = time.NewTimer(100 * time.Millisecond)
					tailLimit = time.NewTimer(time.Second)
					quietC, tailC = quiet.C, tailLimit.C
				}
				continue
			}
			if ev.control == nil {
				o := ev.output
				if len(o.data) > 0 {
					gotOutput = true
					if quiet != nil {
						quiet.Reset(100 * time.Millisecond)
					}
					if settle != nil {
						settleQuietT.Reset(settleQuiet)
					}
					at := recorded
					if !virtual {
						at = time.Duration(float64(o.at.Sub(castStart)) / speed)
					}
					castErr := encoder.Output(at, o.data)
					err := errors.Join(castErr, r.Write(o.data))
					if reply := r.TakeReplies(); len(reply) > 0 {
						err = errors.Join(err, enqueue(writeRequest{data: reply}))
					}
					if opts.Output != nil {
						n, writeErr := opts.Output.Write(o.data)
						if writeErr == nil && n != len(o.data) {
							writeErr = io.ErrShortWrite
						}
						err = errors.Join(err, writeErr)
					}
					if err != nil {
						runErr, finished = err, true
						continue
					}
				}
				checkWait()
				if o.err != nil {
					if !errors.Is(o.err, io.EOF) && !errors.Is(o.err, syscall.EIO) {
						runErr = o.err
					}
					// PTY EOF follows all final output bytes in the FIFO.
					select {
					case <-exit:
						runErr = errors.Join(runErr, exitErr)
					case <-ctx.Done():
						runErr = errors.Join(runErr, ctx.Err())
					}
					finished = true
				}
				continue
			}
			c := *ev.control
			a := c.action
			if virtual {
				handled := true
				switch a.Command {
				case "Hold":
					startSettle(c, func() error { return emit(ctx, time.Duration(float64(a.Duration)/speed)) })
				case "Hide":
					startSettle(c, func() error { visible = false; return nil })
				case "Show":
					startSettle(c, func() error { visible = true; return nil })
				case "Checkpoint":
					startSettle(c, func() error {
						if visible {
							return encoder.Text(r)
						}
						return nil
					})
				case "Screenshot":
					startSettle(c, func() error {
						frame, err := r.FrameAfter(recorded)
						if err != nil {
							return err
						}
						return writeAtomic(a.Text, func(w io.Writer) error { return png.Encode(w, frame) })
					})
				default:
					handled = false
				}
				if handled {
					continue
				}
			}
			var actionErr error
			var input []byte
			switch a.Command {
			case "Hide", "Show":
				if a.Command == "Hide" && visible {
					actionErr = capture(ctx, time.Now(), true)
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
				actionErr = enqueue(writeRequest{data: input, ack: c.ack})
				if actionErr == nil {
					// Acknowledge actual delivery, while still draining output.
					continue
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

		case <-settleQuietC:
			// Output already queued, or none yet from a program that has not
			// started, means the step is not settled.
			if len(events) > 0 || !gotOutput {
				settleQuietT.Reset(settleQuiet)
			} else {
				finishSettle()
			}
		case <-settleLimitC:
			finishSettle()
		case <-quietC:
			// Prefer bytes already queued when the quiet timer fired.
			if len(events) == 0 {
				finished = true
			} else {
				quiet.Reset(100 * time.Millisecond)
			}
		case <-tailC:
			finished = true
		case <-tick:
			if err := capture(ctx, time.Now(), false); err != nil {
				runErr, finished = err, true
			}
		case <-ctx.Done():
			runErr = ctx.Err()
			finished = true
		}
	}
	// Every runtime exit attempts every output's finalization. A failed video
	// stream must not discard healthy GIF/text outputs or their diagnostics.
	// The screen now should be the last frame held. If it is not, the program drew
	// after the tape's final step (a virtual Sleep does not wait for it).
	if virtual && visible && runErr == nil && opts.Warn != nil && !opts.Config.CursorBlink && lastEmitted != nil {
		if f, err := r.FrameAfter(recorded); err == nil && !bytes.Equal(f.Pix, lastEmitted.Pix) {
			fmt.Fprintln(opts.Warn, "sigmamax: warning: the program was still producing output when the tape ended, and a deterministic recording does not wait for it: Sleep is virtual time. End slow steps with Wait+Screen /text/.")
		}
	}
	if virtual {
		// Hold the last viewport for a beat, as realtime does.
		runErr = errors.Join(runErr, emit(context.Background(), max(period, 10*time.Millisecond)))
	} else {
		runErr = errors.Join(runErr, capture(context.Background(), time.Now().Add(period), true))
	}
	if visible {
		runErr = errors.Join(runErr, encoder.Text(r))
	}
	encoder.castTime = recorded
	if !virtual {
		encoder.castTime = time.Duration(float64(time.Since(castStart)) / speed)
	}
	return errors.Join(runErr, encoder.Finish())
}

func play(ctx context.Context, actions []tape.Action, events chan<- event, virtual bool) error {
	control := func(a tape.Action) error {
		c := control{action: a, ack: make(chan error, 1)}
		select {
		case events <- event{control: &c}:
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
	// Time passes by sleeping for real, or, in a deterministic recording, by
	// asking the recorder to hold the settled screen for that long.
	wait := func(d time.Duration) error {
		if d == 0 {
			return ctx.Err()
		}
		if virtual {
			return control(tape.Action{Command: "Hold", Duration: d})
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

// environment is the recorded program's environment. It declares Ghostty's
// terminal, the identity programs check to learn the terminal draws Kitty
// graphics. terminfoDir holds that terminal's entry (see package terminfo); a
// user's own TERMINFO stays searchable through TERMINFO_DIRS. Without a
// directory the entry cannot be found, so it falls back to xterm-256color.
func environment(base []string, version, terminfoDir string) []string {
	var env []string
	var userInfo, userDirs string
	for _, s := range base {
		key, value, _ := strings.Cut(s, "=")
		switch key {
		case "TERM", "COLORTERM", "TERM_PROGRAM", "TERM_PROGRAM_VERSION", "PS1", "PS2":
			continue
		case "TERMINFO", "TERMINFO_DIRS":
			if terminfoDir != "" {
				if key == "TERMINFO" {
					userInfo = value
				} else {
					userDirs = value
				}
				continue
			}
		}
		env = append(env, s)
	}
	term := "xterm-256color"
	if terminfoDir != "" {
		term = terminfo.Term
		env = append(env, "TERMINFO="+terminfoDir)
		if dirs := strings.Trim(userInfo+":"+userDirs, ":"); dirs != "" {
			env = append(env, "TERMINFO_DIRS="+dirs)
		}
	}
	return append(env, "TERM="+term, "COLORTERM=truecolor", "TERM_PROGRAM=sigmamax", "TERM_PROGRAM_VERSION="+version, "PS1=$ ", "PS2=> ")
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
