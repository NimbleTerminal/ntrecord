package record

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"nimbleterminal.dev/ntrecord/internal/tape"
)

func TestLanguagePlayback(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "out.txt")
	gif := filepath.Join(dir, "out.gif")
	shot := filepath.Join(dir, "shot.png")
	frames := filepath.Join(dir, "frames") + "/"
	// A real shell makes Ctrl+U, backspace, paste, and Enter observable. Wait
	// gates on output rather than assuming a machine-specific startup delay.
	source := fmt.Sprintf(`Output %q
Output %q
Output %q
Require sh
Env MESSAGE "hello from environment"
Set Shell /bin/sh
Set Cols 40
Set Rows 8
Set Theme "Catppuccin Mocha"
Set Padding 4
Set Margin 4
Set WindowBar RingsRight
Set TypingSpeed 1ms
Set Framerate 10
Set WaitTimeout 10s
Hide
Type "junk"
Ctrl+U
Copy "printf '%%s READY\\n'"
Paste
Space
Type '"$MESSAGE"'
Enter
Wait+Screen /hello from environment READY/
Type "printf FINISHZ"
Backspace
Type X
Enter
Wait+Screen /FINISHX/
Show
Sleep 0.2
Screenshot %q
`, gif, txt, frames, shot)
	p, err := tape.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = Run(ctx, Options{Config: p.Config, Tape: p}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(txt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "hello from environment READY") || !strings.Contains(string(content), "FINISHX") {
		t.Fatalf("missing actual shell output: %s", content)
	}
	f, err := os.Open(shot)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() <= 40*11 || img.Bounds().Dy() <= 8*21 {
		t.Fatalf("screenshot omitted decorations: %v", img.Bounds())
	}
	paths, err := filepath.Glob(filepath.Join(frames, "*.png"))
	if err != nil || len(paths) < 2 {
		t.Fatalf("missing PNG frames %v %v", paths, err)
	}
	if _, err = os.Stat(gif); err != nil {
		t.Fatal(err)
	}
}

func TestWaitTimeoutAndHiddenCapture(t *testing.T) {
	dir := t.TempDir()
	source := "Output '" + filepath.Join(dir, "timeout.gif") + "'\nSet Cols 20\nSet Rows 3\nWait+Screen@50ms /impossible-output/"
	p, err := tape.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	err = Run(context.Background(), Options{Config: p.Config, Tape: p})
	if err == nil || !strings.Contains(err.Error(), "<tape>:4:1") || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("bad Wait error: %v", err)
	}
	if _, err = os.Stat(p.Config.Output); err != nil {
		t.Fatal("timeout should still save frames", err)
	}
}

func TestRequireUsesTapePATHBeforeSpawning(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	c := tape.DefaultConfig()
	c.Env = map[string]string{"PATH": dir}
	c.Require = []string{"missing-required-program"}
	c.Output = filepath.Join(dir, "out.gif")
	err := Run(context.Background(), Options{Config: c, Command: []string{"/bin/sh", "-c", "touch '" + marker + "'"}})
	if err == nil || !strings.Contains(err.Error(), "Require") {
		t.Fatal(err)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("child ran before Require failed")
	}
}

func TestEnvironmentAdvertisesVersion(t *testing.T) {
	for version, want := range map[string]string{"v1.2.3": "TERM_PROGRAM_VERSION=1.2.3", "": "TERM_PROGRAM_VERSION=dev"} {
		env := environment([]string{"TERM_PROGRAM_VERSION=stale"}, tape.Config{Version: version}.ProgramVersion(), "")
		if !slices.Contains(env, want) || slices.Contains(env, "TERM_PROGRAM_VERSION=stale") {
			t.Errorf("version %q: %v", version, env)
		}
	}
}

func TestEnvironmentDeclaresGhosttyTerminal(t *testing.T) {
	env := environment([]string{"TERM=dumb", "TERMINFO=/users/own", "TERMINFO_DIRS=/other"}, "dev", "/our/db")
	for _, want := range []string{"TERM=xterm-ghostty", "TERMINFO=/our/db", "TERMINFO_DIRS=/users/own:/other"} {
		if !slices.Contains(env, want) {
			t.Errorf("missing %s in %v", want, env)
		}
	}
	for _, stale := range []string{"TERM=dumb", "TERMINFO=/users/own", "TERMINFO_DIRS=/other"} {
		if slices.Contains(env, stale) {
			t.Errorf("stale %s survived in %v", stale, env)
		}
	}
	// Without a directory the entry cannot be found: do not claim it.
	fallback := environment([]string{"TERMINFO=/users/own"}, "dev", "")
	if !slices.Contains(fallback, "TERM=xterm-256color") || !slices.Contains(fallback, "TERMINFO=/users/own") {
		t.Errorf("fallback environment: %v", fallback)
	}
}

// A real recorded program sees the identity programs use to detect Kitty
// graphics, and its terminfo reader finds the entry on a host without Ghostty.
func TestRecordedProgramSeesGhosttyTerminal(t *testing.T) {
	if _, err := exec.LookPath("tput"); err != nil {
		t.Skip("tput unavailable")
	}
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("HOME", filepath.Join(tmp, "no-home")) // a host Ghostty cannot answer
	txt := filepath.Join(t.TempDir(), "out.txt")
	source := fmt.Sprintf("Output %q\nSet Shell /bin/sh\nSet Cols 60\nSet Rows 5\nSet TypingSpeed 1ms\nType 'echo term=$TERM colors=$(tput colors)'\nEnter\nWait+Screen /colors=256/\n", txt)
	p, err := tape.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = Run(ctx, Options{Config: p.Config, Tape: p}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(txt)
	if err != nil || !strings.Contains(string(b), "term=xterm-ghostty colors=256") {
		t.Fatalf("terminal identity not visible to the program: %v\n%s", err, b)
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "ntrecord-terminfo-*")); len(left) != 0 {
		t.Errorf("terminfo directory not removed: %v", left)
	}
}
