package asciicast

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestUTF8ChunksAndTrailingSilence(t *testing.T) {
	var dst bytes.Buffer
	w, err := NewWriter(&dst, Terminal{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	text := "hello 🐈\x1b_Ga=T;YWJj\x1b\\" // graphics bytes stay intact
	for _, b := range []byte(text) {
		if err := w.Output(time.Second, []byte{b}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.End(3 * time.Second); err != nil {
		t.Fatal(err)
	}
	r, err := NewReader(&dst)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	var last Event
	for {
		e, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if e.Code == "o" {
			got.WriteString(e.Data)
			if e.At != time.Second {
				t.Fatal(e)
			}
		}
		last = e
	}
	if got.String() != text || last.At != 3*time.Second || last.Code != "m" {
		t.Fatalf("%q %+v", got.String(), last)
	}
}

func TestVersionClocksAndUnknownEvents(t *testing.T) {
	for _, tc := range []struct {
		header, events string
		want           time.Duration
	}{
		{`{"version":2,"width":80,"height":24}`, "[1,\"o\",\"a\"]\n[2,\"future\",\"b\"]", 2 * time.Second},
		{`{"version":3,"term":{"cols":80,"rows":24}}`, "# comment\n[1,\"o\",\"a\"]\n[2,\"future\",\"b\"]", 3 * time.Second},
	} {
		r, err := NewReader(strings.NewReader(tc.header + "\n" + tc.events))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = r.Next(); err != nil {
			t.Fatal(err)
		}
		e, err := r.Next()
		if err != nil || e.At != tc.want || e.Code != "future" {
			t.Fatalf("%+v %v", e, err)
		}
	}
}

func TestRejectMalformedCasts(t *testing.T) {
	for _, header := range []string{"", "null", `{"version":1}`, `{"version":3}`, `{"version":2,"width":0,"height":24}`, `{"version":2,"width":80,"height":24,"duration":-1}`} {
		if _, err := NewReader(strings.NewReader(header)); err == nil {
			t.Fatalf("accepted %s", header)
		}
	}
	for _, event := range []string{`[0,"o"]`, `[null,"o","x"]`, `[0,null,"x"]`, `[0,"o",null]`, `[-1,"o","x"]`, `[1e30,"o","x"]`, `[1,"o","a"]` + "\n" + `[0,"o","b"]`} {
		r, err := NewReader(strings.NewReader("{\"version\":2,\"width\":80,\"height\":24}\n" + event))
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = r.Next()
		}
		if errors.Is(err, io.EOF) {
			t.Fatalf("accepted %s", event)
		}
	}
}

func TestLargeGraphicsEvent(t *testing.T) {
	text := strings.Repeat("A", 128*1024)
	r, err := NewReader(strings.NewReader("{\"version\":3,\"term\":{\"cols\":80,\"rows\":24}}\n[0,\"o\",\"" + text + "\"]"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := r.Next()
	if err != nil || e.Data != text {
		t.Fatalf("large event lost: %v", err)
	}
}

func TestOutputRejectsBackwardsTimeWhileBuffering(t *testing.T) {
	// An incomplete UTF-8 sequence writes nothing yet, but its time still
	// counts: a later call may not go back before it.
	w, err := NewWriter(io.Discard, Terminal{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Output(2*time.Second, []byte("\xe2")); err != nil {
		t.Fatal(err)
	}
	if err := w.Output(time.Second, []byte("\x82\xac")); err == nil {
		t.Fatal("accepted output before the time of the buffered bytes")
	}
	w, _ = NewWriter(io.Discard, Terminal{Cols: 80, Rows: 24})
	if err := w.Output(2*time.Second, []byte("\xe2")); err != nil {
		t.Fatal(err)
	}
	if err := w.End(time.Second); err == nil {
		t.Fatal("ended before the time of the buffered bytes")
	}
}
