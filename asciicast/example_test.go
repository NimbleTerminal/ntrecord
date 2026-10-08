package asciicast_test

import (
	"bytes"
	"fmt"
	"time"

	"nimbleterminal.dev/ntrecord/asciicast"
)

func ExampleNewWriter() {
	var data bytes.Buffer
	w, err := asciicast.NewWriter(&data, asciicast.Terminal{Cols: 80, Rows: 24})
	if err != nil {
		panic(err)
	}
	if err = w.Output(time.Second, []byte("hello\r\n")); err != nil {
		panic(err)
	}
	if err = w.End(2 * time.Second); err != nil {
		panic(err)
	}
	r, err := asciicast.NewReader(&data)
	if err != nil {
		panic(err)
	}
	e, err := r.Next()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s %s %q\n", e.At, e.Code, e.Data)
	// Output: 1s o "hello\r\n"
}
