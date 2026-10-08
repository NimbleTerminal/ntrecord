package gifencode_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"time"

	"nimbleterminal.dev/ntrecord/gifencode"
)

func ExampleEncoder() {
	var enc gifencode.Encoder
	frame := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for _, c := range []color.RGBA{{255, 0, 0, 255}, {0, 0, 255, 255}} {
		draw.Draw(frame, frame.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
		if err := enc.Add(frame, 100*time.Millisecond); err != nil {
			panic(err)
		}
	}
	var out bytes.Buffer
	if err := enc.Encode(&out); err != nil {
		panic(err)
	}
	animation, err := gif.DecodeAll(&out)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(animation.Image), animation.Delay)
	// Output: 2 [10 10]
}
