package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// These match the renderer's reviewed golden scenarios at 40x12, Go Mono 18.
// Keep the smoke runner standard-library-only: it must test the extracted binary
// without compiling the renderer or needing native build tools/system fonts.
var unicodeCases = []struct{ name, payload, hash string }{
	{
		"unicode-clusters",
		"NFC: é ä ñ  ♞ ⌘\r\n😀 👍🏽 👨‍👩‍👧‍👦 🇯🇵 1️⃣ ❤️ 👩🏽‍💻 END\r\nText: ❤︎  Emoji: ❤️\r\n\x1b[1;3mStyled: é ♞\x1b[0m",
		"12395982da228328a76a78ef986288ac1b4fff297545b37948dddcfb27755d81",
	},
	{
		"box-blocks-braille-wide",
		"┌──┬──┐ ╔══╦══╗ ╭──╮\r\n│  │  │ ║  ║  ║ │  │\r\n└──┴──┘ ╚══╩══╝ ╰──╯\r\n" +
			"▀▄█▌▐ ░▒▓ ▁▂▃▄▅▆▇█ ⠁⠃⠇⡇⣿⢕⡪\r\n日本語 한국어 中文 combining: é ä ñ\r\n" +
			"fallbacks:   ☃ éè üñ ΔΣ ЖЩ\r\n",
		"e09212d59f28011b82b24d0c77a4d7db70d111bf2654131e9c2b187d67db8123",
	},
}

func verifyUnicode(ctx context.Context, bin, dir string) error {
	for _, test := range unicodeCases {
		payload := filepath.Join(dir, test.name+".txt")
		if err := os.WriteFile(payload, []byte("\x1b[?25l"+test.payload), 0600); err != nil {
			return err
		}
		for _, format := range []string{"png", "cast"} {
			frames := filepath.Join(dir, test.name+"-"+format)
			output := frames + string(os.PathSeparator)
			if format == "cast" {
				output = filepath.Join(dir, test.name+".cast")
			}
			// Disable newline translation: the input already has terminal CRLFs.
			cmd := exec.CommandContext(ctx, bin, "record", "-o", output, "-cols", "40", "-rows", "12", "-font-size", "18", "--",
				"/bin/sh", "-c", `stty -opost; cat "$1"; sleep 0.2`, "unicode-smoke", payload)
			if err := runUnicodeCommand(cmd); err != nil {
				return fmt.Errorf("%s %s recording: %w", test.name, format, err)
			}
			if format == "cast" {
				cmd = exec.CommandContext(ctx, bin, "render", output, "-o", frames+string(os.PathSeparator), "-font", "Go Mono", "-font-size", "18")
				if err := runUnicodeCommand(cmd); err != nil {
					return fmt.Errorf("%s replay: %w", test.name, err)
				}
			}
			paths, err := filepath.Glob(filepath.Join(frames, "*.png"))
			if err != nil || len(paths) == 0 {
				return fmt.Errorf("%s %s produced no PNG frames (%v)", test.name, format, err)
			}
			got, err := pngFrameHash(paths[len(paths)-1])
			if err != nil {
				return err
			}
			if got != test.hash {
				return fmt.Errorf("%s %s pixels changed: got %s, want %s; review renderer golden PNGs before updating", test.name, format, got, test.hash)
			}
			fmt.Printf("%s %s: final PNG matches reviewed golden (%d frames)\n", test.name, format, len(paths))
		}
	}
	return nil
}

func runUnicodeCommand(cmd *exec.Cmd) error {
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "SIGMAMAX_FONT_PATH=") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "SIGMAMAX_FONT_PATH=")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

func pngFrameHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		return "", err
	}
	bounds := decoded.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), decoded, bounds.Min, draw.Src)
	h := sha256.New()
	// Same canonical representation as render.frameHash: dimensions, then RGBA.
	_ = binary.Write(h, binary.LittleEndian, [2]int32{int32(bounds.Dx()), int32(bounds.Dy())})
	h.Write(rgba.Pix)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
