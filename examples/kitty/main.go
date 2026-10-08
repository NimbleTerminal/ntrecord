// A tiny direct-PNG Kitty client. Run under ntrecord or a Kitty-capable terminal.
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	virtual := flag.Bool("virtual", false, "display through Unicode placeholders, as ntcharts does")
	flag.Parse()
	// Probe both DA1 and Kitty through the PTY before emitting the image.
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(os.Stdin.Fd()), state)
	fmt.Print("\x1b[c\x1b_Ga=q,t=d,f=24,s=1,v=1,i=31;AAAA\x1b\\")
	ready := make(chan error, 1)
	go func() {
		var reply string
		buf := make([]byte, 1024)
		for !strings.Contains(reply, "\x1b[?") || !strings.Contains(reply, "i=31;OK") {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				ready <- err
				return
			}
			reply += string(buf[:n])
		}
		ready <- nil
	}()
	select {
	case err := <-ready:
		if err != nil {
			return err
		}
	case <-time.After(3 * time.Second):
		return fmt.Errorf("terminal did not answer DA1 and Kitty graphics queries")
	}
	im := image.NewNRGBA(image.Rect(0, 0, 240, 120))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.NRGBA{25, 40, 65, 255}), image.Point{}, draw.Src)
	colors := []color.NRGBA{{240, 72, 88, 255}, {255, 192, 64, 255}, {56, 208, 152, 255}, {64, 152, 248, 255}}
	for i, c := range colors {
		draw.Draw(im, image.Rect(12+i*58, 12, 54+i*58, 108), image.NewUniform(c), image.Point{}, draw.Src)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		return err
	}
	fmt.Print("\x1b[?2026h\x1b[2J\x1b[H\x1b[?25l\x1b[1;36mntrecord\x1b[0m  /  Kitty graphics in a GIF\r\n\r\n")
	options := ""
	if *virtual {
		options = ",U=1,c=12,r=6"
	}
	fmt.Printf("\x1b_Ga=T,t=d,f=100,i=1,q=2%s;%s\x1b\\", options, base64.StdEncoding.EncodeToString(b.Bytes()))
	if *virtual {
		// First twelve protocol row/column diacritics; literal full coordinates
		// match picture.Model's output without introducing an ntcharts dependency.
		diacritics := []rune{0x0305, 0x030d, 0x030e, 0x0310, 0x0312, 0x033d, 0x033e, 0x033f, 0x0346, 0x034a, 0x034b, 0x034c}
		for row := 0; row < 6; row++ {
			fmt.Print("\x1b[38;2;0;0;1m")
			for col := 0; col < 12; col++ {
				fmt.Print(string([]rune{0x10eeee, diacritics[row], diacritics[col]}))
			}
			fmt.Print("\x1b[39m\r\n")
		}
	}
	fmt.Print("\r\n\r\n\x1b[32mPNG rendered through libghostty-vt.\x1b[0m\r\n\x1b[?2026l")
	return nil
}
