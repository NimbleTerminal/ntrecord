package player

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"strings"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/playback"
)

// Alternate image IDs: a chunked upload never removes the currently visible
// image. The old image is deleted only after the new placement is complete.
type display struct {
	writer     io.Writer
	ids        [2]uint32
	slot       int
	statusLine string
	uploading  bool
}

func (d *display) show(frame *image.RGBA, cols, rows int) error {
	var pngData bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&pngData, frame); err != nil {
		return err
	}
	payload := base64.StdEncoding.EncodeToString(pngData.Bytes())
	id := d.ids[d.slot]
	d.uploading = true
	for offset := 0; offset < len(payload); {
		end := min(offset+4096, len(payload))
		more := 0
		if end < len(payload) {
			more = 1
		}
		control := fmt.Sprintf("m=%d", more)
		if offset == 0 {
			control = fmt.Sprintf("a=t,t=d,f=100,i=%d,q=2,%s", id, control)
		}
		if _, err := fmt.Fprintf(d.writer, "\x1b_G%s;%s\x1b\\", control, payload[offset:end]); err != nil {
			return err
		}
		offset = end
	}
	d.uploading = false
	if _, err := fmt.Fprintf(d.writer, "\x1b[?2026h\x1b[H\x1b_Ga=p,i=%d,p=1,c=%d,r=%d,C=1,q=2\x1b\\\x1b_Ga=d,d=I,i=%d,q=2\x1b\\\x1b[?2026l", id, cols, rows, d.ids[1-d.slot]); err != nil {
		return err
	}
	d.slot = 1 - d.slot
	return nil
}

func (d *display) clear() error {
	d.statusLine = ""
	_, err := fmt.Fprintf(d.writer, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", d.ids[0], d.ids[1])
	return err
}
func (d *display) close() error {
	// End any interrupted chunked upload/hold before restoring the main screen.
	var first error
	if d.uploading {
		_, first = io.WriteString(d.writer, "\x1b_Gm=0,q=2;\x1b\\")
	}
	if _, err := io.WriteString(d.writer, "\x1b[?2026l"); first == nil {
		first = err
	}
	second := d.clear()
	_, third := io.WriteString(d.writer, "\x1b[0m\x1b[?25h\x1b[?1049l")
	if first != nil {
		return first
	}
	if second != nil {
		return second
	}
	return third
}
func (d *display) status(title string, c playback.Clock, cols, rows int) error {
	status := "Playing"
	if c.Paused {
		status = "Paused"
	}
	if c.Position == c.Duration {
		status = "Finished"
	}
	line := fmt.Sprintf("%s  %s / %s  %.2gx  %s", status, stamp(c.Position), stamp(c.Duration), c.Speed, title)
	help := "Space play/pause | Left/Right 5s | R restart | +/- speed | Q quit"
	// Leave the final column empty to avoid autowrap at the bottom of the screen.
	line = line[:min(len(line), cols-1)]
	help = help[:min(len(help), cols-1)]
	text := fmt.Sprintf("\x1b[%d;1H\x1b[0m\x1b[2K%s\x1b[%d;1H\x1b[2K%s", rows-1, line, rows, help)
	if text == d.statusLine {
		return nil
	}
	d.statusLine = text
	_, err := io.WriteString(d.writer, text)
	return err
}
func stamp(d time.Duration) string {
	seconds := int(d / time.Second)
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

type keyDecoder struct{ pending string }

func (k *keyDecoder) feed(b []byte) []string {
	k.pending += string(b)
	var keys []string
	for len(k.pending) > 0 {
		if k.pending[0] != 27 {
			ch := k.pending[0]
			k.pending = k.pending[1:]
			key := ""
			switch ch {
			case 'q', 'Q', 3, 4:
				key = "quit"
			case ' ', 'p', 'P':
				key = "toggle"
			case 'r', 'R':
				key = "restart"
			case 'h', 'H':
				key = "left"
			case 'l', 'L':
				key = "right"
			case '+', '=':
				key = "faster"
			case '-':
				key = "slower"
			case '0':
				key = "normal"
			}
			if key != "" {
				keys = append(keys, key)
			}
			continue
		}
		if len(k.pending) < 2 {
			break
		}
		if k.pending[1] == '_' || k.pending[1] == ']' || k.pending[1] == 'P' {
			end := strings.Index(k.pending, "\x1b\\")
			if end < 0 {
				if len(k.pending) > 8192 {
					k.pending = ""
				}
				break
			}
			k.pending = k.pending[end+2:]
			continue
		}
		if k.pending[1] != '[' && k.pending[1] != 'O' {
			k.pending = k.pending[2:]
			continue
		}
		end := 2
		for end < len(k.pending) && (k.pending[end] < 0x40 || k.pending[end] > 0x7e) {
			end++
		}
		if end == len(k.pending) {
			if end > 64 {
				k.pending = ""
			}
			break
		}
		sequence := k.pending[:end+1]
		k.pending = k.pending[end+1:]
		key := map[string]string{"\x1b[D": "left", "\x1b[C": "right", "\x1b[H": "home", "\x1b[F": "end", "\x1bOH": "home", "\x1bOF": "end", "\x1b[1~": "home", "\x1b[4~": "end", "\x1b[7~": "home", "\x1b[8~": "end"}[sequence]
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}
