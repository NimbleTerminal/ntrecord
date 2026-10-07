package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderCommand(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "input.cast")
	if err := os.WriteFile(src, []byte("{\"version\":2,\"width\":20,\"height\":5}\n[0,\"o\",\"CLI REPLAY\"]\n[0.1,\"m\",\"\"]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, inputFirst := range []bool{true, false} {
		dst := filepath.Join(dir, "out.txt")
		args := []string{"render", "-o", dst, src}
		if inputFirst {
			args = []string{"render", src, "-o", dst}
		}
		var stderr bytes.Buffer
		if err := run(args, &bytes.Buffer{}, &stderr); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(dst)
		if err != nil || !strings.Contains(string(data), "CLI REPLAY") || !strings.Contains(stderr.String(), "Wrote") {
			t.Fatalf("render failed: %s %s %v", data, stderr.String(), err)
		}
	}
	for _, args := range [][]string{{src}, {src, "-o", src}, {src, "-o", filepath.Join(dir, "bad.gif"), "-speed", "0"}, {src, "-o", filepath.Join(dir, "bad.gif"), "-fps", "0"}} {
		if err := renderCast(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
