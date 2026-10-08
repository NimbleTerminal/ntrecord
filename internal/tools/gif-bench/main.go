// gif-bench replays a fixed PNG sequence or composited GIF through the encoder.
// Decode time is excluded from encode_seconds; use /usr/bin/time for process RSS.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/gifencode"
)

func main() {
	input := flag.String("input", "", "GIF or directory of numbered PNG frames")
	output := flag.String("output", "", "output GIF")
	rate := flag.Int("fps", 12, "PNG sequence frame rate")
	seed := flag.String("seed", "", "comma-separated RRGGBB colors; defaults to input GIF palette")
	fixed := flag.Bool("fixed", false, "use seeded Plan9 palette instead of recording-wide adaptive palette")
	flag.Parse()
	if err := run(*input, *output, *seed, *rate, *fixed); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, output, seeds string, rate int, fixed bool) error {
	if rate <= 0 || input == "" || output == "" {
		return fmt.Errorf("input, output and positive fps required")
	}
	e := gifencode.Encoder{FixedPalette: fixed}
	for _, s := range strings.Split(seeds, ",") {
		if s == "" {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimPrefix(s, "#"), 16, 24)
		if err != nil {
			return err
		}
		e.Seed = append(e.Seed, color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255})
	}
	var elapsed time.Duration
	var frames int
	var peak uint64
	add := func(f *image.RGBA, d time.Duration) error {
		start := time.Now()
		err := e.Add(f, d)
		elapsed += time.Since(start)
		frames++
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		peak = max(peak, m.HeapAlloc)
		return err
	}
	info, err := os.Stat(input)
	if err != nil {
		return err
	}
	if info.IsDir() {
		paths, err := filepath.Glob(filepath.Join(input, "*.png"))
		if err != nil {
			return err
		}
		for i, p := range paths {
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			im, err := png.Decode(f)
			f.Close()
			if err != nil {
				return err
			}
			rgba := image.NewRGBA(im.Bounds())
			draw.Draw(rgba, rgba.Bounds(), im, im.Bounds().Min, draw.Src)
			d := time.Duration(i+1)*time.Second/time.Duration(rate) - time.Duration(i)*time.Second/time.Duration(rate)
			if err = add(rgba, d); err != nil {
				return err
			}
		}
	} else {
		f, err := os.Open(input)
		if err != nil {
			return err
		}
		g, err := gif.DecodeAll(f)
		f.Close()
		if err != nil {
			return err
		}
		if len(e.Seed) == 0 {
			e.Seed = g.Image[0].Palette
		}
		canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
		for i, im := range g.Image {
			saved := image.NewRGBA(canvas.Bounds())
			copy(saved.Pix, canvas.Pix)
			draw.Draw(canvas, im.Bounds(), im, im.Bounds().Min, draw.Over)
			if err = add(canvas, time.Duration(g.Delay[i])*10*time.Millisecond); err != nil {
				return err
			}
			if len(g.Disposal) > i {
				switch g.Disposal[i] {
				case gif.DisposalBackground:
					draw.Draw(canvas, im.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
				case gif.DisposalPrevious:
					canvas = saved
				}
			}
		}
	}
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	start := time.Now()
	err = e.Encode(f)
	elapsed += time.Since(start)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	peak = max(peak, m.HeapAlloc)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	info, err = os.Stat(output)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"input": input, "frames": frames, "bytes": info.Size(), "encode_seconds": elapsed.Seconds(), "sampled_heap_bytes": peak, "duration_seconds": e.Duration().Seconds()})
}
