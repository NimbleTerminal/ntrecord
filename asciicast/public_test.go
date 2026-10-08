package asciicast_test

import (
	"bytes"
	"nimbleterminal.dev/ntrecord/asciicast"
	"testing"
)

func TestWriterRejectsInvalidGridBeforeWriting(t *testing.T) {
	for _, term := range []asciicast.Terminal{{}, {Cols: -1, Rows: 2}, {Cols: 2, Rows: 8193}} {
		var out bytes.Buffer
		if _, err := asciicast.NewWriter(&out, term); err == nil {
			t.Fatal("invalid grid accepted")
		}
		if out.Len() != 0 {
			t.Fatal("invalid header was written")
		}
	}
}
