package record

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestPTYCloseInterruptsIO(t *testing.T) {
	for _, operation := range []string{"read", "write"} {
		t.Run(operation, func(t *testing.T) {
			original, slave, err := pty.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer original.Close()
			defer slave.Close()
			if _, err := term.MakeRaw(int(slave.Fd())); err != nil {
				t.Fatal(err)
			}
			master, err := pollablePTY(original)
			if err != nil {
				t.Fatal(err)
			}
			_ = original.Close()
			defer master.Close()
			buf := make([]byte, 1024*1024)
			ioOperation := master.Read
			if operation == "write" {
				ioOperation = master.Write
			}
			// Leave the slave open and idle. Prove that I/O blocks on this
			// descriptor, filling the input queue in the write case.
			if err := master.SetDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			if _, err := ioOperation(buf); !errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatalf("expected blocked %s: %v", operation, err)
			}
			if err := master.SetDeadline(time.Time{}); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := ioOperation(buf)
				done <- err
			}()
			select {
			case err := <-done:
				t.Fatalf("%s did not block: %v", operation, err)
			case <-time.After(50 * time.Millisecond):
			}
			if err := master.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, os.ErrClosed) {
					t.Fatalf("%s after close: %v", operation, err)
				}
			case <-time.After(time.Second):
				t.Fatalf("close did not interrupt %s", operation)
			}
		})
	}
}
