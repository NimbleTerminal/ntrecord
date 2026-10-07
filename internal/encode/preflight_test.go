package encode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeFFmpeg puts an ffmpeg stand-in first on PATH. Its -encoders listing holds
// the given encoders; -version prints a fixed banner.
func fakeFFmpeg(t *testing.T, encoders ...string) {
	t.Helper()
	dir := t.TempDir()
	listing := ""
	for _, e := range encoders {
		listing += " V....D " + e + "  codec name\n"
	}
	script := "#!/bin/sh\ncase \"$*\" in\n*-encoders*) printf '%s' '" + listing + "';;\n*-version*) echo 'ffmpeg version 9.9-fake Copyright';;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestPreflightNeedsNothingForSelfContainedOutputs(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no ffmpeg anywhere
	for _, out := range []string{"a.gif", "a.txt", "a.ascii", "a.test", "frames/"} {
		if _, err := Preflight(out); err != nil {
			t.Errorf("%s: %v", out, err)
		}
	}
}

func TestPreflightRejectsUnknownExtension(t *testing.T) {
	if _, err := Preflight("a.bogus"); err == nil || !strings.Contains(err.Error(), "unsupported output") {
		t.Fatalf("got %v", err)
	}
}

func TestPreflightVideoRequiresFFmpegWithTheEncoder(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Preflight("a.mp4"); err == nil || !strings.Contains(err.Error(), "requires ffmpeg on PATH") {
		t.Fatalf("missing ffmpeg: %v", err)
	}
	fakeFFmpeg(t, "libvpx-vp9")
	if _, err := Preflight("a.mp4"); err == nil || !strings.Contains(err.Error(), "libx264") {
		t.Fatalf("mp4 without libx264: %v", err)
	}
	if path, err := Preflight("a.webm"); err != nil || filepath.Base(path) != "ffmpeg" {
		t.Fatalf("webm with libvpx-vp9: %q, %v", path, err)
	}
}

func TestFFmpegInfoReportsVersionAndEncoders(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := FFmpegInfo(); err == nil {
		t.Fatal("no ffmpeg should be an error")
	}
	fakeFFmpeg(t, "libx264")
	info, err := FFmpegInfo()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info.Version, "9.9-fake") || !info.Encoders["libx264"] || info.Encoders["libvpx-vp9"] {
		t.Fatalf("%+v", info)
	}
}
