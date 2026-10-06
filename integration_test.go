package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/gif"
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
