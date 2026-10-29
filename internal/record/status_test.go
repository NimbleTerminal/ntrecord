package record

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/term"
	"nimbleterminal.dev/ntrecord/asciicast"
	"nimbleterminal.dev/ntrecord/internal/tape"
)

func TestStatusHelper(t *testing.T) {
	if os.Getenv("NTRECORD_STATUS_HELPER") != "1" {
		return
	}
	if _, err := term.MakeRaw(int(os.Stdin.Fd())); err != nil {
		panic(err)
	}
	query := "\x1b]7501;?\x1b\\"
	fmt.Print(query)
	reply := "\x1b]7501;?:states=idle,working,done,blocked,error:kinds=permission,question,auth\x1b\\"
	buf := make([]byte, len(reply))
	if _, err := io.ReadFull(os.Stdin, buf); err != nil || string(buf) != reply {
		panic("missing status capability reply")
	}
	fmt.Print("\x1b]7501;state=idle:app=demo\x1b\\")
	fmt.Print("\x1b]7501;state=blocked:id=job:kind=question:msg=Q29udGludWU/\x1b\\")
	if _, err := io.ReadFull(os.Stdin, buf[:1]); err != nil || buf[0] != 'x' {
		panic("missing scripted answer")
	}
	fmt.Print("\x1b]7501;state=done:id=job\x1b\\FINISHED")
	// The recorder ends the tape and closes the PTY; keep the child alive until
	// then so success demonstrates that both status waits actually completed.
	time.Sleep(10 * time.Second)
	os.Exit(0)
}

func TestStatusWaitCapture(t *testing.T) {
	for _, mode := range []string{"Realtime", "Deterministic"} {
		t.Run(mode, func(t *testing.T) {
			p, err := tape.Parse(strings.NewReader("Wait+Status@3s blocked app=demo id=job kind=question\nType x\nWait+Status@3s done app=demo id=job\nWait+Screen@3s /FINISHED/"))
			if err != nil {
				t.Fatal(err)
			}
			p.Config.Cols, p.Config.Rows = 20, 3
			if mode == "Deterministic" {
				p.Config.Mode = tape.Deterministic
			}
			dir := t.TempDir()
			p.Config.Outputs = []string{filepath.Join(dir, "status.cast"), filepath.Join(dir, "status.txt")}
			p.Config.Env = map[string]string{"NTRECORD_STATUS_HELPER": "1"}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := Run(ctx, Options{Config: p.Config, Tape: p, Command: []string{os.Args[0], "-test.run=^TestStatusHelper$"}}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(p.Config.Outputs[0])
			if err != nil {
				t.Fatal(err)
			}
			r, err := asciicast.NewReader(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			var output strings.Builder
			var last time.Duration
			for {
				e, err := r.Next()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if e.At < last {
					t.Fatal("timestamp went backwards")
				}
				last = e.At
				if e.Code == "o" {
					output.WriteString(e.Data)
				}
			}
			for _, want := range []string{"\x1b]7501;?\x1b\\", "state=blocked:id=job:kind=question:msg=Q29udGludWU/", "state=done:id=job", "FINISHED"} {
				if !strings.Contains(output.String(), want) {
					t.Fatalf("cast lost %q", want)
				}
			}
			text, err := os.ReadFile(p.Config.Outputs[1])
			if err != nil || !bytes.Contains(text, []byte("FINISHED")) || bytes.Contains(text, []byte("Continue?")) {
				t.Fatalf("text %q: %v", text, err)
			}
		})
	}
}

func TestStatusWaitFailures(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{"timeout", `printf '\033]7501;state=working:app=demo\033\\'; sleep 5`, "timed out"},
		{"exit", `sleep 0.1; exit 0`, "program exited before"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := tape.Parse(strings.NewReader("Wait+Status@300ms done app=demo"))
			if err != nil {
				t.Fatal(err)
			}
			p.Config.Cols, p.Config.Rows = 20, 3
			p.Config.Output = filepath.Join(t.TempDir(), "status.gif")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err = Run(ctx, Options{Config: p.Config, Tape: p, Command: []string{"/bin/sh", "-c", tc.script}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%v", err)
			}
			assertGIFSaved(t, p.Config.Output)
		})
	}
}
