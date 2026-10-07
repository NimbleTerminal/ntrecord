package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/record"
	"github.com/NimbleMarkets/sigmamax/internal/tape"
	"github.com/NimbleMarkets/sigmamax/internal/testvideo"
)

func readGIF(t *testing.T, path string) *gif.GIF {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func assertImage(t *testing.T, g *gif.GIF) {
	t.Helper()
	// The fixture's red bar must survive PTY -> APC -> PNG -> palette GIF.
	red := 0
	for _, f := range g.Image {
		for y := 0; y < f.Bounds().Dy(); y++ {
			for x := 0; x < f.Bounds().Dx(); x++ {
				r, gg, b, _ := f.At(x, y).RGBA()
				if r > 180*257 && gg < 130*257 && b < 130*257 {
					red++
				}
			}
		}
	}
	if red < 500 {
		t.Fatalf("GIF lacks the Kitty image: only %d red pixels", red)
	}
}

func TestKittyTapeAndLiveRecord(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "kitty-fixture")
	build := exec.Command("go", "build", "-o", bin, "./examples/kitty")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, out)
	}
	for _, format := range []string{"gif", "mp4", "webm"} {
		t.Run(format, func(t *testing.T) {
			if format != "gif" {
				testvideo.Require(t)
			}
			for _, scenario := range []struct{ scripted, virtual bool }{{true, false}, {false, false}, {true, true}, {false, true}} {
				scripted, virtual := scenario.scripted, scenario.virtual
				t.Run(fmt.Sprintf("tape=%v/virtual=%v", scripted, virtual), func(t *testing.T) {
					c := tape.DefaultConfig()
					c.Cols = 64
					c.Rows = 16
					c.Output = filepath.Join(t.TempDir(), "kitty."+format)
					opts := record.Options{Config: c}
					var output bytes.Buffer
					extra := ""
					if virtual {
						extra = " -virtual"
					}
					if scripted {
						p, err := tape.Parse(strings.NewReader("Hide\nType \"'" + bin + "'" + extra + "\"\nEnter\nSleep 300ms\nShow\nSleep 150ms\n"))
						if err != nil {
							t.Fatal(err)
						}
						opts.Tape = p
					} else {
						opts.Command = []string{bin}
						if virtual {
							opts.Command = append(opts.Command, "-virtual")
						}
						opts.Output = &output
					}
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := record.Run(ctx, opts); err != nil {
						t.Fatal(err)
					}
					if format == "gif" {
						assertImage(t, readGIF(t, c.Output))
					} else {
						pixels := testvideo.Decode(t, c.Output)
						red := 0
						for i := 0; i+2 < len(pixels); i += 3 {
							if pixels[i] > 180 && pixels[i+1] < 130 && pixels[i+2] < 130 {
								red++
							}
						}
						if red < 500 {
							t.Fatalf("video lacks Kitty image: %d red pixels", red)
						}
					}
					if !scripted && !strings.Contains(output.String(), "PNG rendered through libghostty-vt") {
						t.Fatal("live fixture failed to complete its query handshake")
					}
				})
			}
		})
	}
}

func TestTextOnlyTape(t *testing.T) {
	for _, format := range []string{"gif", "mp4", "webm"} {
		t.Run(format, func(t *testing.T) {
			if format != "gif" {
				testvideo.Require(t)
			}
			p, err := tape.Parse(strings.NewReader("Hide\nType `printf '\\033[2J\\033[H\\033[?25l\\033[38;2;0;255;0mTEXT-ONLY\\033[0m\\n'`\nEnter\nSleep 100ms\nShow\nSleep 150ms\n"))
			if err != nil {
				t.Fatal(err)
			}
			p.Config.Output = filepath.Join(t.TempDir(), "text."+format)
			p.Config.Cols = 32
			p.Config.Rows = 8
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := record.Run(ctx, record.Options{Config: p.Config, Tape: p}); err != nil {
				t.Fatal(err)
			}
			green := 0
			if format == "gif" {
				g := readGIF(t, p.Config.Output)
				for _, f := range g.Image {
					for y := 0; y < 24; y++ {
						for x := 0; x < f.Bounds().Dx(); x++ {
							r, gg, b, _ := f.At(x, y).RGBA()
							if gg > r*2 && gg > b*2 {
								green++
							}
						}
					}
				}
			} else {
				pixels := testvideo.Decode(t, p.Config.Output)
				for i := 0; i+2 < len(pixels); i += 3 {
					if int(pixels[i+1]) > int(pixels[i])*2 && int(pixels[i+1]) > int(pixels[i+2])*2 {
						green++
					}
				}
			}
			if green < 20 {
				t.Fatal("text-only recording lost its green glyphs")
			}
		})
	}
}

func TestRecordExitAndCancellation(t *testing.T) {
	for _, format := range []string{"gif", "mp4", "webm"} {
		t.Run(format, func(t *testing.T) {
			if format != "gif" {
				testvideo.Require(t)
			}
			for _, cancelled := range []bool{false, true} {
				t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
					c := tape.DefaultConfig()
					c.Cols = 20
					c.Rows = 5
					c.Output = filepath.Join(t.TempDir(), "exit."+format)
					command := "printf '\\033[?2026hdone'; exit 7"
					timeout := 3 * time.Second
					if cancelled {
						command = "printf started; sleep 10"
						timeout = 150 * time.Millisecond
					}
					ctx, cancel := context.WithTimeout(context.Background(), timeout)
					defer cancel()
					err := record.Run(ctx, record.Options{Config: c, Command: []string{"/bin/sh", "-c", command}})
					if err == nil {
						t.Fatal("missing exit error")
					}
					if cancelled && !errors.Is(err, context.DeadlineExceeded) {
						t.Fatal(err)
					}
					if format == "gif" {
						readGIF(t, c.Output)
					} else {
						if len(testvideo.Decode(t, c.Output)) == 0 {
							t.Fatal("video has no frames")
						}
					}
				})
			}
		})
	}
}

// Run the real main entry point in a subprocess so Go's special handling of
// SIGPIPE on stdout is exercised, rather than only a synthetic Writer error.
func TestBrokenStdoutStillSavesGIF(t *testing.T) {
	if os.Getenv("SIGMAMAX_MAIN_HELPER") == "1" {
		os.Args = []string{"sigmamax", "record", "-o", os.Getenv("SIGMAMAX_TEST_OUTPUT"), "-cols", "20", "-rows", "3", "--", "/bin/sh", "-c", "printf 'first\\n'; sleep 0.2; printf 'second\\n'; sleep 2"}
		main()
		os.Exit(0)
	}
	path := filepath.Join(t.TempDir(), "pipe.gif")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBrokenStdoutStillSavesGIF$")
	cmd.Env = append(os.Environ(), "SIGMAMAX_MAIN_HELPER=1", "SIGMAMAX_TEST_OUTPUT="+path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 32)
	if _, err = stdout.Read(b); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	err = cmd.Wait()
	if err == nil || !strings.Contains(stderr.String(), "broken pipe") {
		t.Fatalf("expected EPIPE, got %v: %s", err, stderr.String())
	}
	readGIF(t, path)
}

// Reproduce ntcharts' same-ID virtual PNG updates without its GPU dependency.
// Pauses force capture ticks between APC chunks and inside a partial APC.
func TestChunkedKittyAnimationHelper(t *testing.T) {
	if os.Getenv("SIGMAMAX_CHUNKED_HELPER") != "1" {
		return
	}
	fmt.Print("\x1b[?25l")
	colors := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}}
	for i := 0; i < 6; i++ {
		im := image.NewRGBA(image.Rect(0, 0, 22, 21))
		draw.Draw(im, im.Bounds(), image.NewUniform(colors[i%3]), image.Point{}, draw.Src)
		var b bytes.Buffer
		if err := png.Encode(&b, im); err != nil {
			panic(err)
		}
		data := base64.StdEncoding.EncodeToString(b.Bytes())
		split := len(data) / 8 * 4
		fmt.Printf("\x1b_Gf=100,q=2,i=43,U=1,c=2,r=1,a=T,m=1;%s\x1b\\", data[:split])
		if i > 0 {
			time.Sleep(60 * time.Millisecond)
		}
		final := "\x1b_Gq=2,m=0;" + data[split:] + "\x1b\\"
		fmt.Print(final[:len(final)/2])
		if i > 0 {
			time.Sleep(60 * time.Millisecond)
		}
		fmt.Print(final[len(final)/2:])
		if i == 0 {
			fmt.Print("\x1b[H\x1b[38;2;0;0;43m\U0010eeee\u0305\u0305\U0010eeee\u0305\u030d\x1b[0m")
		}
		time.Sleep(120 * time.Millisecond)
	}
	// A genuine clear is part of the recording and must not be filtered out.
	fmt.Print("\x1b[2J\x1b[H")
	time.Sleep(120 * time.Millisecond)
	os.Exit(0)
}

func TestChunkedKittyAnimationContinuity(t *testing.T) {
	for _, format := range []string{"gif", "mp4", "webm"} {
		t.Run(format, func(t *testing.T) {
			if format != "gif" {
				testvideo.Require(t)
			}
			c := tape.DefaultConfig()
			c.Cols, c.Rows = 20, 3
			c.Output = filepath.Join(t.TempDir(), "animation."+format)
			c.Env = map[string]string{"SIGMAMAX_CHUNKED_HELPER": "1"}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := record.Run(ctx, record.Options{Config: c, Command: []string{os.Args[0], "-test.run=^TestChunkedKittyAnimationHelper$"}}); err != nil {
				t.Fatal(err)
			}
			var samples []color.RGBA
			if format == "gif" {
				for _, f := range readGIF(t, c.Output).Image {
					samples = append(samples, color.RGBAModel.Convert(f.At(5, 5)).(color.RGBA))
				}
			} else {
				// Crop one interior pixel per decoded frame; codec chroma differences
				// are tolerated by classifying its dominant channel.
				data := testvideo.Decode(t, c.Output, "-vf", "crop=2:2:4:4")
				for i := 0; i+11 < len(data); i += 12 {
					samples = append(samples, color.RGBA{data[i], data[i+1], data[i+2], 255})
				}
			}
			var transitions []string
			started := false
			for _, p := range samples {
				state := "blank"
				switch {
				case p.R > 150 && p.G < 100 && p.B < 100:
					state = "red"
				case p.G > 150 && p.R < 100 && p.B < 100:
					state = "green"
				case p.B > 150 && p.R < 100 && p.G < 100:
					state = "blue"
				}
				if state != "blank" {
					started = true
				}
				if started && (len(transitions) == 0 || transitions[len(transitions)-1] != state) {
					transitions = append(transitions, state)
				}
			}
			want := "red green blue red green blue blank"
			if got := strings.Join(transitions, " "); got != want {
				t.Fatalf("animation lost a frame or captured an upload gap: %s; want %s", got, want)
			}
		})
	}
}

// This opt-in check captures the actual GPU application. CI always runs the
// deterministic chunked-PNG regression above; it does not need a GPU/ntcharts.
func TestNTChartsShaderContinuity(t *testing.T) {
	bin := os.Getenv("SIGMAMAX_SHADER_BIN")
	if bin == "" {
		t.Skip("set SIGMAMAX_SHADER_BIN or run task test:shaders")
	}
	path := os.Getenv("SIGMAMAX_SHADER_OUTPUT")
	if path == "" {
		path = filepath.Join(t.TempDir(), "shaders.gif")
	}
	c := tape.DefaultConfig()
	c.Output = path
	c.Env = map[string]string{"NTCHARTS_KITTY": "supported", "NTCHARTS_TMUX_PASSTHROUGH": "false"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var output bytes.Buffer
	err := record.Run(ctx, record.Options{Config: c, Command: []string{"env", "-u", "NO_COLOR", bin, "-medium=direct", "-fullscreen", "-fps=20", "-duration=8s", "-preset=plasma"}, Output: &output})
	if err != nil {
		t.Fatalf("shader: %v (first output bytes: %q)", err, output.Bytes()[:min(output.Len(), 500)])
	}
	g := readGIF(t, path)
	first, last := -1, -1
	blanks := make([]bool, len(g.Image))
	for i, f := range g.Image {
		counts := make([]int, len(f.Palette))
		peak := 0
		for _, index := range f.Pix {
			counts[index]++
			peak = max(peak, counts[index])
		}
		blanks[i] = float64(peak)/float64(len(f.Pix)) > 0.99
		if !blanks[i] {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 || last == first {
		t.Fatal("shader produced no moving image")
	}
	elapsed := 0
	for i, d := range g.Delay {
		if i > first && i < last && blanks[i] {
			t.Errorf("blank frame %d at %.2fs, held %.2fs", i, float64(elapsed)/100, float64(d)/100)
		}
		elapsed += d
	}
	t.Logf("inspected %d frames over %.2fs; active image frames %d..%d; saved %s", len(g.Image), float64(elapsed)/100, first, last, path)
}
