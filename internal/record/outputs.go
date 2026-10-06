package record

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/encode"
	"github.com/NimbleMarkets/sigmamax/internal/render"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
)

type outputs struct {
	encoders  []*encode.Encoder
	needsText bool
}

func newOutputs(c tape.Config, r *render.Renderer) (*outputs, error) {
	o := &outputs{}
	rate := c.Framerate
	if rate == 0 {
		rate = 20
	}
	w, h := r.OutputSize()
	for _, path := range c.Destinations() {
		e, err := encode.NewWithOptions(path, w, h, encode.Options{Framerate: rate, LoopOffset: c.LoopOffset})
		if err != nil {
			o.Abort()
			return nil, err
		}
		o.encoders = append(o.encoders, e)
		format, _ := encode.Format(path)
		if format == "txt" || format == "ascii" || format == "test" {
			o.needsText = true
		}
	}
	return o, nil
}
func (o *outputs) Add(frame *image.RGBA, d time.Duration) error {
	for _, e := range o.encoders {
		if err := e.Add(frame, d); err != nil {
			return err
		}
	}
	return nil
}
func (o *outputs) Text(r *render.Renderer) error {
	if !o.needsText {
		return nil
	}
	text, err := r.Text("Screen")
	if err != nil {
		return err
	}
	for _, e := range o.encoders {
		if err = e.AddText(text); err != nil {
			return err
		}
	}
	return nil
}
func (o *outputs) Finish() error {
	for _, e := range o.encoders {
		if err := e.Finish(); err != nil {
			return err
		}
	}
	return nil
}
func (o *outputs) Abort() {
	for _, e := range o.encoders {
		e.Abort()
	}
}

func writeAtomic(path string, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".sigmamax-*")
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
