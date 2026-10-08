// Exercise an extracted release binary without compiling ntrecord or linking
// libghostty on the test machine. Only the Go standard library is needed.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: release-smoke /path/to/ntrecord")
	}
	bin, err := filepath.Abs(os.Args[1])
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	version, err := exec.CommandContext(ctx, bin, "--version").CombinedOutput()
	if err != nil || !strings.HasPrefix(string(version), "ntrecord v") {
		return fmt.Errorf("release version missing: %s (%v)", version, err)
	}
	dir, err := os.MkdirTemp("", "ntrecord-release-smoke-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			im.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, im); err != nil {
		return err
	}
	for _, kitty := range []bool{false, true} {
		name, payload := "text", "\x1b[97mRELEASE TEXT\x1b[0m"
		if kitty {
			name = "kitty"
			payload = "\x1b_Ga=T,t=d,f=100,q=2;" + base64.StdEncoding.EncodeToString(pngData.Bytes()) + "\x1b\\"
		}
		// The payload has no single quotes; printf's format is a constant.
		script := "printf '%s' '" + payload + "'; sleep 0.2"
		for _, format := range []string{"gif", "cast"} {
			path := filepath.Join(dir, name+"."+format)
			cmd := exec.CommandContext(ctx, bin, "record", "-o", path, "-cols", "40", "-rows", "8", "--", "/bin/sh", "-c", script)
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%s %s recording: %w: %s", name, format, err, output)
			}
			if format == "cast" {
				rendered := filepath.Join(dir, name+"-cast.gif")
				cmd = exec.CommandContext(ctx, bin, "render", path, "-o", rendered)
				if output, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("%s cast replay: %w: %s", name, err, output)
				}
				path = rendered
			}
			count, err := verifyGIF(path, kitty)
			if err != nil {
				return fmt.Errorf("%s %s: %w", name, format, err)
			}
			fmt.Printf("%s %s: %d GIF frames verified\n", name, format, count)
		}
	}
	if err := verifyUnicode(ctx, bin, dir); err != nil {
		return err
	}
	fmt.Print(string(version))
	return nil
}

func verifyGIF(path string, kitty bool) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	found := false
	for _, frame := range g.Image {
		pixels := 0
		for _, index := range frame.Pix {
			r, g, b, _ := frame.Palette[index].RGBA()
			if kitty && r > 0xc000 && g < 0x4000 && b < 0x4000 || !kitty && r > 0xc000 && g > 0xc000 && b > 0xc000 {
				pixels++
			}
		}
		found = found || pixels > 100
	}
	if !found {
		return 0, fmt.Errorf("recording has no expected pixels")
	}
	return len(g.Image), nil
}
