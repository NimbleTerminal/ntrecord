package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runCLI runs the command line and returns what it printed and its error.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	err = run(args, &out, &errb)
	return out.String(), errb.String(), err
}

func writeTape(t *testing.T, name, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUsageNamesEveryCommand(t *testing.T) {
	out, _, err := runCLI(t)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"validate", "doctor", "skill", "record", "--version", "<file.tape>"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage lacks %q:\n%s", want, out)
		}
	}
}

func TestValidateSignsOffOutLoud(t *testing.T) {
	good := writeTape(t, "good.tape", "Output "+t.TempDir()+"/g.gif\nSet Columns 30\nSet Rows 8\nType \"hi\"\nEnter\n")
	out, _, err := runCLI(t, "validate", good)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{good + ": ok", "2 commands", "30x8", "g.gif"} {
		if !strings.Contains(out, want) {
			t.Errorf("sign-off lacks %q:\n%s", want, out)
		}
	}
}

func TestValidateChecksEveryTapeAndFailsOnAnyProblem(t *testing.T) {
	good := writeTape(t, "good.tape", "Output "+t.TempDir()+"/g.gif\nType \"hi\"\n")
	bad := writeTape(t, "bad.tape", "Output x.gif\nBogusCommand\n")
	missing := filepath.Join(t.TempDir(), "absent.tape")
	out, errText, err := runCLI(t, "validate", good, bad, missing)
	if err == nil || !strings.Contains(err.Error(), "2 of 3") {
		t.Fatalf("want an error counting 2 of 3 failures, got %v", err)
	}
	if !strings.Contains(out, good+": ok") {
		t.Errorf("the good tape was not signed off:\n%s", out)
	}
	for _, want := range []string{bad, "BogusCommand", missing} {
		if !strings.Contains(errText, want) {
			t.Errorf("problems lack %q:\n%s", want, errText)
		}
	}
}

func TestValidateNeedsATape(t *testing.T) {
	if _, _, err := runCLI(t, "validate"); err == nil || !strings.Contains(err.Error(), "tape") {
		t.Fatalf("got %v", err)
	}
}

func TestDoctorCommand(t *testing.T) {
	out, _, err := runCLI(t, "doctor")
	if err != nil {
		t.Fatalf("doctor failed on a working host: %v\n%s", err, out)
	}
	if !strings.Contains(out, "engine") || !strings.Contains(out, "libghostty-vt") {
		t.Errorf("doctor output:\n%s", out)
	}
}

func TestModeFlagSelectsTheClock(t *testing.T) {
	dir := t.TempDir()
	path := writeTape(t, "demo.tape", "Output "+filepath.Join(dir, "demo.gif")+"\nSet Shell /bin/sh\nSet Cols 30\nSet Rows 5\nType \"hi\"\nEnter\nSleep 4s\n")
	start := time.Now()
	_, stderr, err := runCLI(t, "-mode", "deterministic", path)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("a deterministic 4s sleep took %v in real time", took)
	}
	if !strings.Contains(stderr, "Wrote") {
		t.Errorf("no confirmation: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo.gif")); err != nil {
		t.Error(err)
	}
	// The = form works too, and realtime is still the default.
	if _, _, err := runCLI(t, "--mode=realtime", writeTape(t, "r.tape", "Output "+filepath.Join(dir, "r.gif")+"\nSet Shell /bin/sh\nType \"x\"\nSleep 100ms\n")); err != nil {
		t.Fatal(err)
	}
}

func TestModeFlagRejectsUnknownModesAndLiveRecording(t *testing.T) {
	path := writeTape(t, "demo.tape", "Output x.gif\nType \"x\"\n")
	if _, _, err := runCLI(t, "-mode", "warp", path); err == nil || !strings.Contains(err.Error(), "warp") {
		t.Fatalf("unknown mode: %v", err)
	}
	// record is driven by a person: it has no -mode.
	if _, _, err := runCLI(t, "record", "-mode", "deterministic", "-o", filepath.Join(t.TempDir(), "x.gif")); err == nil {
		t.Fatal("record accepted -mode")
	}
}

func TestUsageMentionsMode(t *testing.T) {
	out, _, _ := runCLI(t)
	if !strings.Contains(out, "-mode") || !strings.Contains(out, "deterministic") {
		t.Errorf("usage does not explain -mode:\n%s", out)
	}
}
