package record

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"time"

	"nimbleterminal.dev/ntrecord/internal/encode"
	"nimbleterminal.dev/ntrecord/internal/render"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

type outputs struct {
	encoders  []*encode.Encoder
	needsText bool
	casts     []*castOutput
	castTime  time.Duration
}

func newOutputs(c tape.Config, r *render.Renderer) (*outputs, error) {
	o := &outputs{}
	rate := c.Framerate
	if rate == 0 {
		rate = 20
	}
	w, h := r.OutputSize()
	seed := r.PaletteSeed()
	for _, path := range c.Destinations() {
		format, _ := encode.Format(path)
		if format == "cast" {
			cast, err := newCastOutput(path, r)
			if err != nil {
				o.Abort()
				return nil, err
			}
			o.casts = append(o.casts, cast)
			continue
		}
		e, err := encode.NewWithOptions(path, w, h, encode.Options{Framerate: rate, LoopOffset: c.LoopOffset, Seed: seed})
		if err != nil {
			o.Abort()
			return nil, err
		}
		o.encoders = append(o.encoders, e)
		if format == "txt" || format == "ascii" || format == "test" {
			o.needsText = true
		}
	}
	return o, nil
}
func (o *outputs) Add(ctx context.Context, frame *image.RGBA, d time.Duration) error {
	var errs []error
	for _, e := range o.encoders {
		errs = append(errs, e.AddContext(ctx, frame, d))
	}
	return errors.Join(errs...)
}
func (o *outputs) Text(r *render.Renderer) error {
	if !o.needsText {
		return nil
	}
	text, err := r.Text("Screen")
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range o.encoders {
		errs = append(errs, e.AddText(text))
	}
	return errors.Join(errs...)
}
func (o *outputs) Output(at time.Duration, b []byte) error {
	var errs []error
	for _, c := range o.casts {
		errs = append(errs, c.output(at, b))
	}
	o.castTime = at
	return errors.Join(errs...)
}
func (o *outputs) Finish() error {
	var errs []error
	for _, e := range o.encoders {
		errs = append(errs, e.Finish())
	}
	for _, c := range o.casts {
		errs = append(errs, c.finish(o.castTime))
	}
	return errors.Join(errs...)
}
func (o *outputs) Abort() {
	for _, c := range o.casts {
		c.abort()
	}
	for _, e := range o.encoders {
		e.Abort()
	}
}

func writeAtomic(path string, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ntrecord-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = write(f); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(f.Name(), 0644); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	return nil
}
