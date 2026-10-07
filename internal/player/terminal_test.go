package player

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/sigmamax/internal/tape"
	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestTerminalPlayerControlsAndRestoration(t *testing.T) {
	for _, exit := range []string{"quit", "cancel", "probe-rejected"} {
		t.Run(exit, func(t *testing.T) {
			master, slave, err := pty.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer master.Close()
			defer slave.Close()
			if err = pty.Setsize(slave, &pty.Winsize{Cols: 80, Rows: 24}); err != nil {
				t.Fatal(err)
			}
			original, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "test.cast")
			cast := "{\"version\":3,\"term\":{\"cols\":20,\"rows\":5}}\n[0,\"o\",\"HELLO\"]\n[30,\"m\",\"\"]"
			if err = os.WriteFile(path, []byte(cast), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			readCtx, stopRead := context.WithCancel(context.Background())
			defer stopRead()
			chunks := make(chan string, 64)
			go func() {
				b := make([]byte, 8192)
				for {
					n, err := master.Read(b)
					if n > 0 {
						select {
						case chunks <- string(b[:n]):
						case <-readCtx.Done():
							return
						}
					}
					if err != nil {
						return
					}
				}
			}()
			done := make(chan error, 1)
			go func() { done <- Run(ctx, path, Options{Config: tape.DefaultConfig(), Paused: true, Terminal: slave}) }()
			var transcript strings.Builder
			waitFor := func(pattern string) string {
				t.Helper()
				var recent strings.Builder
				deadline := time.NewTimer(5 * time.Second)
				defer deadline.Stop()
				for {
					select {
					case chunk := <-chunks:
						transcript.WriteString(chunk)
						recent.WriteString(chunk)
						if strings.Contains(recent.String(), pattern) {
							return recent.String()
						}
					case <-deadline.C:
						t.Fatalf("timeout waiting for %q", pattern)
						return ""
					}
				}
			}
			query := waitFor(";AAAA\x1b\\")
			match := regexp.MustCompile(`i=(\d+);AAAA`).FindStringSubmatch(query)
			if len(match) != 2 {
				t.Fatal("missing graphics query")
			}
			reply := "OK"
			if exit == "probe-rejected" {
				reply = "ENOTSUP"
			}
			if _, err = fmt.Fprintf(master, "\x1b_Gi=%s;%s\x1b\\", match[1], reply); err != nil {
				t.Fatal(err)
			}
			if exit != "probe-rejected" {
				waitFor("Paused")
				if _, err = master.Write([]byte("r")); err != nil {
					t.Fatal(err)
				}
				waitFor("Playing")
				if _, err = master.Write([]byte(" ")); err != nil {
					t.Fatal(err)
				}
				waitFor("Paused")
				if _, err = master.Write([]byte("+")); err != nil {
					t.Fatal(err)
				}
				waitFor("2x")
				if err = pty.Setsize(slave, &pty.Winsize{Cols: 40, Rows: 10}); err != nil {
					t.Fatal(err)
				}
				waitFor("c=40,r=8")
				if exit == "quit" {
					master.Write([]byte("q"))
				} else {
					cancel()
				}
				waitFor("\x1b[?1049l")
			}
			select {
			case err = <-done:
				if exit == "probe-rejected" {
					if err == nil || !strings.Contains(err.Error(), "rejected") {
						t.Fatal(err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("player did not stop")
			}
			restored, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, restored) {
				t.Fatal("terminal input mode was not restored")
			}
			if exit == "probe-rejected" && strings.Contains(transcript.String(), "\x1b[?1049h") {
				t.Fatal("changed screen before capability check")
			}
		})
	}
}

// The real CLI opens /dev/tty, which is a different device from a PTY slave on
// macOS. Exercise that path in a subprocess with its own controlling terminal.
func TestControllingTTYHelper(t *testing.T) {
	path := os.Getenv("SIGMAMAX_PLAYER_TTY_HELPER")
	if path == "" {
		return
	}
	// Normalize the kernel's canonical-input bookkeeping before comparing
	// opaque term.State values: macOS sets PENDIN when canonical mode resumes.
	fd := int(os.Stdin.Fd())
	saved, err := term.MakeRaw(fd)
	if err != nil {
		t.Fatal(err)
	}
	if err := term.Restore(fd, saved); err != nil {
		t.Fatal(err)
	}
	original, err := term.GetState(fd)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Run(ctx, path, Options{Config: tape.DefaultConfig(), Paused: true}); err != nil {
		t.Fatal(err)
	}
	restored, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("controlling terminal mode was not restored: before=%+v after=%+v", original, restored)
	}
}

func TestPlayerOpensControllingTTY(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controlling.cast")
	if err := os.WriteFile(path, []byte("{\"version\":3,\"term\":{\"cols\":20,\"rows\":5}}\n[0,\"o\",\"HELLO\"]\n[30,\"m\",\"\"]"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestControllingTTYHelper$")
	cmd.Env = append(os.Environ(), "SIGMAMAX_PLAYER_TTY_HELPER="+path)
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	chunks := make(chan string, 64)
	go func() {
		defer close(chunks)
		buf := make([]byte, 8192)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				select {
				case chunks <- string(buf[:n]):
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var transcript strings.Builder
	waitFor := func(pattern string) {
		t.Helper()
		for !strings.Contains(transcript.String(), pattern) {
			select {
			case chunk, ok := <-chunks:
				if !ok {
					t.Fatalf("player exited before %q: %q", pattern, transcript.String())
				}
				transcript.WriteString(chunk)
			case <-ctx.Done():
				t.Fatalf("timeout before %q: %q", pattern, transcript.String())
			}
		}
	}
	waitFor(";AAAA\x1b\\")
	match := regexp.MustCompile(`i=(\d+);AAAA`).FindStringSubmatch(transcript.String())
	if len(match) != 2 {
		t.Fatal("no graphics query")
	}
	// A real terminal reply arrives asynchronously, sometimes in multiple reads.
	time.Sleep(100 * time.Millisecond)
	if _, err := fmt.Fprintf(master, "\x1b_Gi=%s;", match[1]); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := master.Write([]byte("OK\x1b\\")); err != nil {
		t.Fatal(err)
	}
	waitFor("Paused")
	if _, err := master.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	waitFor("\x1b[?1049l")
	select {
	case err := <-done:
		if err != nil {
			for chunk := range chunks {
				transcript.WriteString(chunk)
			}
			tail := transcript.String()
			if i := strings.LastIndex(tail, "\x1b[?1049l"); i >= 0 {
				tail = tail[i+len("\x1b[?1049l"):]
			}
			t.Fatalf("player helper: %v\n%s", err, tail)
		}
	case <-ctx.Done():
		t.Fatal("player failed to exit")
	}
}
