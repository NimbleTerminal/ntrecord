// Package encode writes GIFs in Go or streams RGBA frames to FFmpeg for video.
package encode

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/gifencode"
)

const FramePeriod = 50 * time.Millisecond

// Format validates the output extension without requiring external tools.
func Format(path string) (string, error) {
	if strings.HasSuffix(path, "/") {
		return "frames", nil
	}
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".gif", ".mp4", ".webm", ".txt", ".ascii", ".test":
		return ext[1:], nil
	default:
		return "", fmt.Errorf("unsupported output %q: use .gif, .mp4, .webm, .txt, .ascii, or a frame directory ending in /", path)
	}
}

// Encoder owns its temporary output and subprocess. Call Abort on every path;
// Finish atomically publishes the completed file and makes Abort a no-op.
type Options struct {
	Framerate  float64
	LoopOffset string
}

type Encoder struct {
	format          string
	period          time.Duration
	loopOffset      string
	file            *os.File
	previousText    string
	hasText         bool
	path, temporary string
	width, height   int
	gif             *gifencode.Encoder
	cmd             *exec.Cmd
	stdin           *os.File
	failure         error
	stderr          errorLog
	elapsed         time.Duration
	frames          int64
}

func New(path string, width, height int) (*Encoder, error) {
	return NewWithOptions(path, width, height, Options{Framerate: 20})
}

func NewWithOptions(path string, width, height int, opts Options) (*Encoder, error) {
	if opts.Framerate < 1 || opts.Framerate > 120 || math.IsNaN(opts.Framerate) || math.IsInf(opts.Framerate, 0) {
		return nil, fmt.Errorf("framerate must be between 1 and 120")
	}
	format, err := Format(path)
	if err != nil {
		return nil, err
	}
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("invalid frame dimensions %dx%d", width, height)
	}
	e := &Encoder{path: filepath.Clean(path), width: width, height: height, format: format, period: time.Duration(float64(time.Second) / opts.Framerate), loopOffset: opts.LoopOffset}
	switch format {
	case "txt", "ascii", "test":
		e.file, err = e.tempFile()
		if err != nil {
			return nil, err
		}
		return e, nil
	case "frames":
		if files, er := os.ReadDir(e.path); er == nil && len(files) > 0 {
			return nil, fmt.Errorf("frame directory %q must be empty or absent", e.path)
		} else if er != nil && !os.IsNotExist(er) {
			return nil, er
		}
		if err = os.MkdirAll(filepath.Dir(e.path), 0755); err != nil {
			return nil, err
		}
		e.temporary, err = os.MkdirTemp(filepath.Dir(e.path), ".sigmamax-frames-*")
		if err != nil {
			return nil, err
		}
		return e, nil
	}
	if format == "gif" {
		e.gif = &gifencode.Encoder{}
		return e, nil
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf(".%s output requires ffmpeg on PATH (macOS: brew install ffmpeg; Debian/Ubuntu: sudo apt-get install ffmpeg)", format)
	}
	codec := "libx264"
	if format == "webm" {
		codec = "libvpx-vp9"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	listing, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-encoders").Output()
	if err != nil {
		return nil, fmt.Errorf("check FFmpeg encoders: %w", err)
	}
	found := false
	for _, line := range strings.Split(string(listing), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == codec {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf(".%s output requires FFmpeg built with %s", format, codec)
	}
	f, err := e.tempFile()
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		e.Abort()
		return nil, err
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-f", "rawvideo", "-pixel_format", "rgba", "-video_size", fmt.Sprintf("%dx%d", width, height),
		"-framerate", strconv.FormatFloat(opts.Framerate, 'f', -1, 64), "-i", "pipe:0", "-an",
		// yuv420p requires even dimensions. Preserve the viewport and pad only
		// its right/bottom edges when necessary, rather than rescaling text.
		"-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2", "-pix_fmt", "yuv420p", "-c:v", codec, "-threads", "2"}
	if format == "mp4" {
		args = append(args, "-preset", "veryfast", "-crf", "18", "-movflags", "+faststart")
	} else {
		args = append(args, "-deadline", "realtime", "-cpu-used", "4", "-crf", "30", "-b:v", "0")
	}
	args = append(args, "-f", format, e.temporary)
	e.cmd = exec.Command(ffmpeg, args...)
	// Recording cancellation must still allow FFmpeg to flush the container.
	// Give it its own process group so terminal signals cannot interrupt that.
	e.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	e.cmd.Stderr = &e.stderr
	e.cmd.WaitDelay = 5 * time.Second
	// Own the pipe explicitly: deadline support is a requirement, not an
	// optional type assertion on exec's io.WriteCloser.
	var reader *os.File
	reader, e.stdin, err = os.Pipe()
	if err == nil {
		err = e.stdin.SetWriteDeadline(time.Time{})
		if err == nil {
			e.cmd.Stdin = reader
			err = e.cmd.Start()
		}
		_ = reader.Close()
	}
	if err != nil {
		e.Abort()
		return nil, fmt.Errorf("start FFmpeg: %w", err)
	}
	return e, nil
}

func (e *Encoder) Add(frame *image.RGBA, duration time.Duration) error {
	return e.AddContext(context.Background(), frame, duration)
}

// AddContext interrupts a blocked video write when recording is cancelled.
// A partial raw frame cannot be finalized safely; that video fails while
// independent output formats can still be saved.
func (e *Encoder) AddContext(ctx context.Context, frame *image.RGBA, duration time.Duration) error {
	if e.failure != nil {
		return e.failure
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if frame.Bounds().Dx() != e.width || frame.Bounds().Dy() != e.height {
		return fmt.Errorf("frame dimensions changed during recording")
	}
	if duration <= 0 {
		return nil
	}
	if e.gif != nil {
		return e.gif.Add(frame, duration)
	}
	if e.file != nil {
		return nil
	}
	if e.stdin != nil {
		if err := e.stdin.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return err
		}
		interrupted := make(chan struct{})
		stop := context.AfterFunc(ctx, func() {
			_ = e.stdin.SetWriteDeadline(time.Now())
			close(interrupted)
		})
		defer func() {
			if !stop() {
				<-interrupted
			}
		}()
	}
	e.elapsed += duration
	// Round cumulative time, avoiding drift from rounding each sample. Repeat
	// held frames on the fixed 20 FPS timeline; the codec compresses duplicates.
	target := int64((e.elapsed + e.period/2) / e.period)
	for e.frames < target {
		if e.format == "frames" {
			f, err := os.Create(filepath.Join(e.temporary, fmt.Sprintf("%06d.png", e.frames+1)))
			if err != nil {
				return err
			}
			err = png.Encode(f, frame)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			e.frames++
			continue
		}
		if err := e.writeFrame(frame); err != nil {
			// Reap before reading stderr, which exec writes asynchronously.
			e.stop()
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			e.failure = fmt.Errorf("write FFmpeg frame: %w: %s", err, e.stderr.String())
			return e.failure
		}
		e.frames++
	}
	return nil
}

func (e *Encoder) writeFrame(frame *image.RGBA) error {
	if frame.Stride == e.width*4 {
		_, err := e.stdin.Write(frame.Pix[:e.width*e.height*4])
		return err
	}
	for y := 0; y < e.height; y++ {
		if _, err := e.stdin.Write(frame.Pix[y*frame.Stride : y*frame.Stride+e.width*4]); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) AddText(s string) error {
	if e.file == nil || e.hasText && s == e.previousText {
		return nil
	}
	if _, err := fmt.Fprintln(e.file, s+"\n"+strings.Repeat("─", 80)); err != nil {
		return err
	}
	e.previousText = s
	e.hasText = true
	return nil
}

func (e *Encoder) Finish() error {
	if e.failure != nil {
		return e.failure
	}
	if e.format == "frames" {
		if e.frames == 0 {
			return fmt.Errorf("recording has no visible frames")
		}
		// Only replace an empty directory, never erase existing frames.
		if info, err := os.Stat(e.path); err == nil && info.IsDir() {
			if err = os.Remove(e.path); err != nil {
				return err
			}
		}
		if err := os.Rename(e.temporary, e.path); err != nil {
			return err
		}
		e.temporary = ""
		return nil
	}
	if e.file != nil {
		if err := e.file.Close(); err != nil {
			return err
		}
		if !e.hasText {
			return fmt.Errorf("recording has no visible frames")
		}
	} else if e.gif != nil {
		f, err := e.tempFile()
		if err != nil {
			return err
		}
		if e.loopOffset != "" {
			var fraction float64
			if strings.HasSuffix(e.loopOffset, "%") {
				v, _ := strconv.ParseFloat(strings.TrimSuffix(e.loopOffset, "%"), 64)
				fraction = v / 100
			} else {
				n, _ := strconv.ParseFloat(e.loopOffset, 64)
				if e.gif.Duration() > 0 {
					fraction = n * float64(e.period) / float64(e.gif.Duration())
				}
			}
			e.gif.Rotate(fraction)
		}
		err = e.gif.Encode(f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	} else {
		_ = e.stdin.Close()
		// Bound finalization independently of the cancelled recording context.
		cmd := e.cmd
		timer := time.AfterFunc(30*time.Second, func() { _ = cmd.Process.Kill() })
		err := cmd.Wait()
		timer.Stop()
		e.cmd = nil
		if e.frames == 0 {
			return fmt.Errorf("recording has no visible frames (use Show before finishing)")
		}
		if err != nil {
			return fmt.Errorf("encode video with FFmpeg: %w: %s", err, e.stderr.String())
		}
	}
	if err := os.Chmod(e.temporary, 0644); err != nil {
		return err
	}
	if err := os.Rename(e.temporary, e.path); err != nil {
		return err
	}
	e.temporary = ""
	return nil
}

func (e *Encoder) tempFile() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(e.path), 0755); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(e.path), ".sigmamax-*")
	if err == nil {
		e.temporary = f.Name()
	}
	return f, err
}

func (e *Encoder) stop() {
	if e.stdin != nil {
		_ = e.stdin.Close()
	}
	if e.cmd != nil && e.cmd.Process != nil {
		_ = e.cmd.Process.Kill()
		_ = e.cmd.Wait()
	}
	e.cmd = nil
}

func (e *Encoder) Abort() {
	e.stop()
	if e.file != nil {
		_ = e.file.Close()
	}
	if e.temporary != "" {
		if e.format == "frames" {
			_ = os.RemoveAll(e.temporary)
		} else {
			_ = os.Remove(e.temporary)
		}
		e.temporary = ""
	}
}

// Keep the tail of diagnostics without allowing a failing subprocess to fill
// memory. Access only after Wait has joined exec's stderr-copy goroutine.
type errorLog struct{ tail []byte }

func (l *errorLog) Write(p []byte) (int, error) {
	n := len(p)
	const limit = 8192
	if n >= limit {
		l.tail = append(l.tail[:0], p[n-limit:]...)
	} else {
		if len(l.tail)+n > limit {
			l.tail = l.tail[len(l.tail)+n-limit:]
		}
		l.tail = append(l.tail, p...)
	}
	return n, nil
}

func (l *errorLog) String() string { return strings.TrimSpace(string(l.tail)) }
