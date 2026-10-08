package render_test

import (
	"bytes"
	"fmt"
	"image/png"

	"nimbleterminal.dev/ntrecord/render"
)

func ExampleNew() {
	c := render.DefaultConfig()
	c.Cols, c.Rows = 40, 8
	r, err := render.New(c)
	if err != nil {
		panic(err)
	}
	defer r.Close()
	if err = r.Write([]byte("\x1b[?25lHello, terminal!")); err != nil {
		panic(err)
	}
	frame, err := r.FrameAfter(0)
	if err != nil {
		panic(err)
	}
	var out bytes.Buffer
	if err = png.Encode(&out, frame); err != nil {
		panic(err)
	}
	fmt.Println(out.Len() > 0)
	// Output: true
}
