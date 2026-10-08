package terminal_test

import (
	"bytes"
	"image/gif"
	"testing"
	"time"

	"nimbleterminal.dev/ntrecord/gifencode"
	"nimbleterminal.dev/ntrecord/render"
)

func TestRenderToGIF(t *testing.T) {
	t.Setenv("NTRECORD_FONT_PATH", "")
	c := render.DefaultConfig()
	c.Cols, c.Rows = 20, 4
	r, err := render.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = r.Write([]byte("\x1b[?25lé ♞ ┌─┐ ⣿ 👍🏽")); err != nil {
		t.Fatal(err)
	}
	f, err := r.FrameAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	pixels := append([]byte(nil), f.Pix...)
	enc := gifencode.Encoder{}
	if err = enc.Add(f, time.Second); err != nil {
		t.Fatal(err)
	}
	if err = r.Write([]byte("\x1b[2J\x1b[Hreplacement\x1b[6n")); err != nil {
		t.Fatal(err)
	}
	if len(r.TakeReplies()) == 0 || len(r.TakeReplies()) != 0 {
		t.Fatal("replies must drain")
	}
	if _, err = r.FrameAfter(time.Second); err != nil {
		t.Fatal(err)
	}
	r.Close()
	r.Close()
	if !bytes.Equal(pixels, f.Pix) {
		t.Fatal("retained frame changed after write/close")
	}
	var data bytes.Buffer
	if err = enc.Encode(&data); err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(&data)
	if err != nil {
		t.Fatal(err)
	}
	if g.Config.Width != f.Bounds().Dx() || g.Config.Height != f.Bounds().Dy() {
		t.Fatal("encoded dimensions changed")
	}
}
