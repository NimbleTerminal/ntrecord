// Package player displays ntrecord-rendered cast frames inside a Kitty-capable
// terminal. Recorded control sequences never reach the host terminal directly.
package player

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"nimbleterminal.dev/ntrecord/internal/playback"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

type Options struct {
	Config tape.Config
	Paused bool
	// Terminal is normally nil: Run opens /dev/tty. Tests supply a PTY slave.
	Terminal *os.File
}

// Run restores terminal mode, screen, cursor and owned image storage on exit.
func Run(ctx context.Context, path string, opts Options) (result error) {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	replay, err := playback.New(ctx, f, opts.Config)
	if err != nil {
		return err
	}
	defer replay.Close()
	clock, err := playback.NewClock(replay.Duration, opts.Config.PlaybackSpeed, time.Now())
	if err != nil {
		return err
	}
	tty := opts.Terminal
	if tty == nil {
		tty, err = os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("play needs an interactive terminal: %w", err)
		}
		defer tty.Close()
	}
	fd := int(tty.Fd())
	saved, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("prepare playback terminal: %w", err)
	}
	defer func() { result = errors.Join(result, term.Restore(fd, saved)) }()
	var random [4]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	base := (binary.BigEndian.Uint32(random[:]) & 0x7ffffffc) + 1
	if err = probe(ctx, tty, base+2); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	display := display{writer: tty, ids: [2]uint32{base, base + 1}}
	defer func() { result = errors.Join(result, display.close()) }()
	if _, err = io.WriteString(tty, "\x1b[?1049h\x1b[?25l\x1b[2J\x1b[H"); err != nil {
		return err
	}
	if !opts.Paused {
		clock.Play(time.Now())
	}
	period := time.Duration(float64(time.Second) / opts.Config.Framerate)
	title := safeTitle(filepath.Base(path))
	var keys keyDecoder
	var lastFrame *image.RGBA
	var lastCols, lastRows int
	dirty := true
	nextDraw := time.Time{}
	renderedPosition := time.Duration(-1)
	for {
		if ctx.Err() != nil {
			return nil
		}
		clock.Update(time.Now())
		cols, rows, err := term.GetSize(fd)
		if err != nil {
			return err
		}
		resized := cols != lastCols || rows != lastRows
		if cols < 20 || rows < 4 {
			clock.Pause(time.Now())
			if resized {
				if err = display.clear(); err != nil {
					return err
				}
				if _, err = io.WriteString(tty, "\x1b[2J\x1b[HResize terminal to at least 20 columns and 4 rows."); err != nil {
					return err
				}
			}
			dirty = true
		} else {
			if dirty || resized || clock.Position != renderedPosition && !time.Now().Before(nextDraw) {
				frame, err := replay.Frame(ctx, clock.Position)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
				if frame != lastFrame || resized {
					if err = display.show(frame, cols, rows-2); err != nil {
						return err
					}
					lastFrame = frame
				}
				dirty = false
				renderedPosition = clock.Position
				nextDraw = time.Now().Add(period)
			}
			if err = display.status(title, clock, cols, rows); err != nil {
				return err
			}
		}
		lastCols, lastRows = cols, rows
		wait := 20 * time.Millisecond
		if !clock.Paused && cols >= 20 && rows >= 4 {
			wait = min(wait, max(time.Millisecond, time.Until(nextDraw)))
		}
		b, err := readInput(ctx, tty, wait)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		for _, key := range keys.feed(b, time.Now()) {
			now := time.Now()
			switch key {
			case "quit":
				return nil
			case "toggle":
				if clock.Paused {
					clock.Play(now)
				} else {
					clock.Pause(now)
				}
			case "restart":
				clock.Seek(0, now)
				clock.Play(now)
			case "left":
				clock.Update(now)
				clock.Seek(clock.Position-5*time.Second, now)
			case "right":
				clock.Update(now)
				clock.Seek(clock.Position+5*time.Second, now)
			case "home":
				clock.Seek(0, now)
			case "end":
				clock.Seek(clock.Duration, now)
			case "faster":
				clock.SetSpeed(min(16, clock.Speed*2), now)
			case "slower":
				clock.SetSpeed(max(0.1, clock.Speed/2), now)
			case "normal":
				clock.SetSpeed(1, now)
			}
			dirty = true
		}
	}
}

func readInput(ctx context.Context, tty *os.File, wait time.Duration) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// macOS poll/kqueue cannot wait on /dev/tty (the controlling-terminal
	// device), although polling a PTY slave works. select supports both.
	fd := int(tty.Fd())
	var readable unix.FdSet
	if fd < 0 || fd >= len(readable.Bits)*unix.NFDBITS {
		return nil, fmt.Errorf("terminal descriptor %d exceeds select capacity", fd)
	}
	readable.Set(fd)
	timeout := unix.NsecToTimeval(int64(max(time.Millisecond, min(wait, 50*time.Millisecond))))
	ready, err := unix.Select(fd+1, &readable, nil, nil, &timeout)
	if errors.Is(err, unix.EINTR) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ready > 0 && readable.IsSet(fd) {
		b := make([]byte, 4096)
		n, err := unix.Read(fd, b)
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, io.EOF
		}
		return b[:n], nil
	}
	return nil, nil
}

func probe(ctx context.Context, tty *os.File, id uint32) error {
	if _, err := fmt.Fprintf(tty, "\x1b_Ga=q,t=d,f=24,s=1,v=1,i=%d;AAAA\x1b\\", id); err != nil {
		return err
	}
	prefix := fmt.Sprintf("\x1b_Gi=%d;", id)
	var response string
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		b, err := readInput(ctx, tty, 50*time.Millisecond)
		if err != nil {
			return err
		}
		if strings.ContainsAny(string(b), "\x03\x04") {
			return context.Canceled
		}
		response += string(b)
		if start := strings.Index(response, prefix); start >= 0 {
			if end := strings.Index(response[start:], "\x1b\\"); end >= 0 {
				reply := response[start+len(prefix) : start+end]
				if reply == "OK" {
					return nil
				}
				return fmt.Errorf("terminal rejected Kitty graphics: %s", safeTitle(reply))
			}
		}
		if len(response) > 8192 {
			response = response[len(response)-4096:]
		}
	}
	return fmt.Errorf("play requires a terminal with Kitty graphics support; no response to the graphics probe (try Kitty or Ghostty directly, outside tmux)")
}

func safeTitle(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return '?'
		}
		return r
	}, s)
}
