package doctor

import (
	"bytes"
	"strings"
	"testing"
)

func run(t *testing.T, o Options) (string, bool) {
	t.Helper()
	var b bytes.Buffer
	ok := Run(&b, o)
	return b.String(), ok
}

func TestPassesOnAHealthyHost(t *testing.T) {
	out, ok := run(t, Options{Version: "v1.2.3"})
	if !ok {
		t.Fatalf("healthy host reported failure:\n%s", out)
	}
	for _, name := range []string{"ntrecord", "engine", "renderer", "pty", "terminfo", "ffmpeg"} {
		if !strings.Contains(out, name) {
			t.Errorf("no %q check in:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "1.2.3") || !strings.Contains(out, "Kitty graphics") {
		t.Errorf("version and engine details missing:\n%s", out)
	}
}

// ffmpeg is optional: without it only video is lost, so it warns and passes.
func TestWarnsButPassesWithoutFFmpeg(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	out, ok := run(t, Options{})
	if !ok {
		t.Fatalf("missing ffmpeg must not fail:\n%s", out)
	}
	if !strings.Contains(out, "warn") || !strings.Contains(out, "GIF") {
		t.Errorf("expected a warning that names what still works:\n%s", out)
	}
}

func TestFailsWhenTheShellIsMissing(t *testing.T) {
	out, ok := run(t, Options{Shell: "/nonexistent/sh"})
	if ok || !strings.Contains(out, "FAIL") || !strings.Contains(out, "/nonexistent/sh") {
		t.Fatalf("ok=%v\n%s", ok, out)
	}
}

func TestNoColorWarnsAboutPlaceholderImages(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out, ok := run(t, Options{})
	if !ok || !strings.Contains(out, "NO_COLOR") {
		t.Fatalf("ok=%v\n%s", ok, out)
	}
}
