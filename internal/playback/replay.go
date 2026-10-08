// Package playback reconstructs cast state for interactive viewing. It never
// executes recorded input and keeps image data in the existing terminal renderer.
package playback

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"strconv"
	"time"

	"nimbleterminal.dev/ntrecord/internal/asciicast"
	"nimbleterminal.dev/ntrecord/internal/render"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

// Replay owns a renderer, but not src. The caller serializes access. Seeking
// backwards rebuilds state from the start, including all earlier image uploads.
// Events are streamed, so recording size does not determine memory use.
type Replay struct {
	src      io.ReadSeeker
	config   tape.Config
	reader   *asciicast.Reader
	renderer *render.Renderer
	next     asciicast.Event
	eof      bool
	position time.Duration
	frame    *image.RGBA
	Duration time.Duration
}

func New(ctx context.Context, src io.ReadSeeker, c tape.Config) (*Replay, error) {
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	reader, err := asciicast.NewReader(src)
	if err != nil {
		return nil, err
	}
	c.Cols, c.Rows = reader.Header.Size()
	c.Width, c.Height = 0, 0
	if c.Theme == "" {
		c.Theme, err = reader.Header.Colors().ThemeJSON()
		if err != nil {
			return nil, err
		}
	}
	if !(c.Framerate >= 1 && c.Framerate <= 120) {
		return nil, fmt.Errorf("framerate must be between 1 and 120")
	}
	p := &Replay{src: src, config: c}
	duration := time.Duration(reader.Header.Duration * float64(time.Second))
	lastOutput := time.Duration(-1)
	// Validate the complete stream and find its duration before opening a viewer.
	// This also avoids discovering an unsupported resize halfway through a show.
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if err = p.validate(e); err != nil {
			return nil, err
		}
		duration = max(duration, e.At)
		if e.Code == "o" {
			lastOutput = e.At
		}
	}
	period := time.Duration(float64(time.Second) / c.Framerate)
	if duration == 0 || lastOutput == duration {
		if duration > time.Duration(math.MaxInt64)-period {
			return nil, fmt.Errorf("cast duration is too large")
		}
		duration += period
	}
	p.Duration = duration
	if err = p.reset(); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func (p *Replay) validate(e asciicast.Event) error {
	if e.Code == "r" {
		expected := strconv.Itoa(p.config.Cols) + "x" + strconv.Itoa(p.config.Rows)
		if e.Data != expected {
			return fmt.Errorf("asciicast resize to %q is unsupported: playback uses the initial %s grid", e.Data, expected)
		}
	}
	return nil
}

func (p *Replay) reset() error {
	if p.renderer != nil {
		p.renderer.Close()
		p.renderer = nil
	}
	if _, err := p.src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader, err := asciicast.NewReader(p.src)
	if err != nil {
		return err
	}
	cols, rows := reader.Header.Size()
	if cols != p.config.Cols || rows != p.config.Rows {
		return fmt.Errorf("cast header changed during playback")
	}
	p.reader = reader
	p.renderer, err = render.New(p.config)
	if err != nil {
		return err
	}
	p.position = 0
	p.frame, err = p.renderer.FrameAfter(0)
	if err != nil {
		return err
	}
	return p.advance()
}

func (p *Replay) advance() error {
	e, err := p.reader.Next()
	if errors.Is(err, io.EOF) {
		p.eof = true
		return nil
	}
	if err != nil {
		return err
	}
	if err = p.validate(e); err != nil {
		return err
	}
	p.next, p.eof = e, false
	return nil
}

func (p *Replay) sample(at time.Duration, final bool) error {
	if !final && p.renderer.RenderHeldAfter(at) {
		return nil
	}
	frame, err := p.renderer.CaptureFrameAfter(at)
	if err != nil {
		return err
	}
	p.frame = frame
	return nil
}

// Frame reconstructs a position in recorded time. Returned pixels are immutable.
func (p *Replay) Frame(ctx context.Context, at time.Duration) (*image.RGBA, error) {
	at = min(max(at, 0), p.Duration)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if at < p.position {
		if err := p.reset(); err != nil {
			return nil, err
		}
	}
	for !p.eof && p.next.At <= at {
		when := p.next.At
		// Preserve the last complete screen before a chunked replacement or hold.
		if err := p.sample(when, false); err != nil {
			return nil, err
		}
		for !p.eof && p.next.At == when {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if p.next.Code == "o" {
				if err := p.renderer.WriteAfter([]byte(p.next.Data), when); err != nil {
					return nil, err
				}
				p.renderer.TakeReplies()
			}
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
		if err := p.sample(when, false); err != nil {
			return nil, err
		}
	}
	if err := p.sample(at, at == p.Duration); err != nil {
		return nil, err
	}
	p.position = at
	return p.frame, nil
}

func (p *Replay) Close() {
	if p.renderer != nil {
		p.renderer.Close()
		p.renderer = nil
	}
}
