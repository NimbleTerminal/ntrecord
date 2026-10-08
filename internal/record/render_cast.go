package record

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"nimbleterminal.dev/ntrecord/internal/asciicast"
	"nimbleterminal.dev/ntrecord/internal/encode"
	"nimbleterminal.dev/ntrecord/internal/render"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

// RenderCast renders events without running a shell or waiting for wall time.
// The cast owns the terminal grid and (unless overridden) its initial theme.
// Fixed-size exports reject resizes instead of silently losing terminal state.
func RenderCast(ctx context.Context, src io.Reader, c tape.Config) error {
	reader, err := asciicast.NewReader(src)
	if err != nil {
		return err
	}
	for _, path := range c.Destinations() {
		format, err := encode.Format(path)
		if err != nil {
			return err
		}
		if format == "cast" {
			return fmt.Errorf("render requires a pixel or text output, not .cast")
		}
	}
	c.Cols, c.Rows = reader.Header.Size()
	c.Width, c.Height = 0, 0
	if c.Theme == "" {
		c.Theme, err = reader.Header.Colors().ThemeJSON()
		if err != nil {
			return err
		}
	}

	if !(c.Framerate >= 1 && c.Framerate <= 120) {
		return fmt.Errorf("framerate must be between 1 and 120")
	}
	speed := c.PlaybackSpeed

	if speed <= 0 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		return fmt.Errorf("speed must be finite and positive")
	}
	r, err := render.New(c)
	if err != nil {
		return err
	}
	defer r.Close()
	out, err := newOutputs(c, r)
	if err != nil {
		return err
	}
	defer out.Abort()
	period := time.Duration(float64(time.Second) / c.Framerate)
	var next asciicast.Event
	eof := false
	var end time.Duration
	previous, err := r.FrameAfter(0)
	if err != nil {
		return err
	}
	advance := func() error {
		e, err := reader.Next()
		if errors.Is(err, io.EOF) {
			duration := reader.Header.Duration * float64(time.Second) / speed
			if duration >= float64(math.MaxInt64)-float64(period) {
				return fmt.Errorf("scaled cast duration is too large")
			}
			eof = true
			end = max(end, time.Duration(duration))
			return nil
		}
		if err != nil {
			return err
		}
		scaled := float64(e.At) / speed
		if scaled >= float64(math.MaxInt64)-float64(period) {
			return fmt.Errorf("scaled cast duration is too large")
		}
		e.At = time.Duration(scaled)
		next, end = e, e.At
		return nil
	}
	if err = advance(); err != nil {
		return err
	}
	pendingOutput := false
	for pos := time.Duration(0); ; pos += period {
		if err := ctx.Err(); err != nil {
			return err
		}
		for !eof && next.At <= pos {
			switch next.Code {
			case "o":
				if err := r.WriteAfter([]byte(next.Data), next.At); err != nil {
					return err
				}
				r.TakeReplies() // replay never forwards terminal replies or input
				pendingOutput = true
			case "r":
				expected := strconv.Itoa(c.Cols) + "x" + strconv.Itoa(c.Rows)
				if next.Data != expected {
					return fmt.Errorf("asciicast resize to %q is unsupported: exports use the initial %s grid", next.Data, expected)
				}
			}
			if err = advance(); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		// A final output event needs at least one displayed frame. A trailing marker
		// already accounts for the hold; don't add an extra frame for that marker.
		if eof && pos >= end && !pendingOutput && pos > 0 {
			break
		}
		frame := previous
		if frame == nil || !r.RenderHeldAfter(pos) || eof && pos >= end {
			frame, err = r.CaptureFrameAfter(pos)
			pendingOutput = false
		}
		if err != nil {
			return err
		}
		d := period
		if eof && end > pos {
			d = min(d, end-pos)
		}
		previous = frame
		if err = out.Add(ctx, frame, d); err != nil {
			return err
		}
		if eof && pos >= end {
			break
		}
	}
	if err = out.Text(r); err != nil {
		return err
	}
	return out.Finish()
}
