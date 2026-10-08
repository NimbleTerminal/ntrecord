// Package testvideo contains helpers for tests that exercise real video codecs.
package testvideo

import (
	"os"
	"os/exec"
	"testing"
)

func Require(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			if os.Getenv("NTRECORD_REQUIRE_FFMPEG") == "1" {
				t.Fatalf("video tests require %s on PATH", name)
			}
			t.Skipf("%s unavailable; use task test:video to require video tests", name)
		}
	}
}

func Decode(t *testing.T, path string, args ...string) []byte {
	t.Helper()
	cmdArgs := append([]string{"-v", "error", "-i", path}, args...)
	cmdArgs = append(cmdArgs, "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
	cmd := exec.Command("ffmpeg", cmdArgs...)
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return b
}
