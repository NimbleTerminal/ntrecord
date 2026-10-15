package player

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"
)

func TestGraphicsChunksSwapAndCleanup(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// Incompressible enough to exercise multiple Kitty transport chunks.
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x * y), uint8(x*17 + y), uint8(y*13 + x), 255})
		}
	}
	var output bytes.Buffer
	d := display{writer: &output, ids: [2]uint32{101, 102}}
	if err := d.show(img, 80, 22); err != nil {
		t.Fatal(err)
	}
	first := output.String()
	var payload strings.Builder
	chunks := 0
	for _, part := range strings.Split(first, "\x1b_G")[1:] {
		end := strings.Index(part, "\x1b\\")
		if end < 0 {
			t.Fatal("unterminated APC")
		}
		control, data, ok := strings.Cut(part[:end], ";")
		if !ok {
			continue
		}
		if !strings.Contains(control, "m=") {
			continue
		}
		if len(data) > 4096 || len(data)%4 != 0 {
			t.Fatal("invalid chunk length", len(data))
		}
		chunks++
		payload.WriteString(data)
	}
	if chunks < 2 {
		t.Fatal("did not chunk image")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.String())
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != img.Bounds() || got.At(30, 30) != img.At(30, 30) {
		t.Fatal("pixels changed in transport")
	}
	placement := strings.Index(first, "a=p,i=101")
	deletion := strings.Index(first, "a=d,d=I,i=102")
	if placement < 0 || deletion < placement {
		t.Fatal("deleted old frame before placing new frame")
	}
	output.Reset()
	if err := d.show(img, 80, 22); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "a=p,i=102") || !strings.Contains(output.String(), "a=d,d=I,i=101") {
		t.Fatal("did not swap buffers")
	}
	output.Reset()
	if err := d.close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\x1b[?25h\x1b[?1049l") || strings.Contains(output.String(), "d=A") {
		t.Fatal("unsafe cleanup")
	}
}

func TestKeysAcrossReadsIgnoreTerminalReplies(t *testing.T) {
	var k keyDecoder
	if got := k.feed([]byte("\x1b["), time.Time{}); len(got) != 0 {
		t.Fatal(got)
	}
	got := k.feed([]byte("D \x1b_Gi=12;some reply q r\x1b\\+\x1b[Fq"), time.Time{})
	if strings.Join(got, ",") != "left,toggle,faster,end,quit" {
		t.Fatal(got)
	}
	if title := safeTitle("bad\x1b]52;c;payload\a.cast"); strings.ContainsAny(title, "\x1b\a") {
		t.Fatal(title)
	}
}

func TestLoneEscapeDoesNotSwallowTheNextKey(t *testing.T) {
	// Escape on its own is a key, not the start of a sequence, once nothing
	// follows it for a moment; the key after it still counts.
	var k keyDecoder
	start := time.Now()
	if got := k.feed([]byte("\x1b"), start); len(got) != 0 {
		t.Fatal(got)
	}
	k.feed(nil, start.Add(escapeTimeout))
	if got := k.feed([]byte("q"), start.Add(escapeTimeout+time.Millisecond)); strings.Join(got, ",") != "quit" {
		t.Fatalf("q after a lone Escape: %v", got)
	}
	// A sequence split across reads within the timeout still decodes.
	k.feed([]byte("\x1b"), start)
	if got := k.feed([]byte("[D"), start.Add(escapeTimeout/2)); strings.Join(got, ",") != "left" {
		t.Fatalf("split arrow: %v", got)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestDisplayWriteFailure(t *testing.T) {
	d := display{writer: failingWriter{}, ids: [2]uint32{1, 2}}
	if err := d.show(image.NewRGBA(image.Rect(0, 0, 2, 2)), 80, 22); err == nil {
		t.Fatal("write failure hidden")
	}
}
