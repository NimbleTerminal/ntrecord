package terminfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolated runs a command that can only find terminfo through dir: HOME and
// the search list are empty, so a Ghostty installed on the host cannot answer.
func isolated(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.TempDir() + "/sigmamax-no-home", "TERMINFO=" + dir}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func TestInstalledEntryIsFoundByNcurses(t *testing.T) {
	if _, err := exec.LookPath("tput"); err != nil {
		t.Skip("tput unavailable")
	}
	empty := t.TempDir()
	if _, err := isolated(empty, "tput", "-T", Term, "colors"); err == nil {
		t.Skip("this host already resolves " + Term + " without our entry")
	}
	dir := t.TempDir()
	if err := Install(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{Term, "ghostty"} {
		if out, err := isolated(dir, "tput", "-T", name, "colors"); err != nil || out != "256" {
			t.Errorf("tput -T %s colors = %q, %v; want 256", name, out, err)
		}
	}
}

func TestInstallIsIdempotentAndMakesBothLayouts(t *testing.T) {
	dir := t.TempDir()
	for range 2 {
		if err := Install(dir); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{"x/xterm-ghostty", "78/xterm-ghostty"} {
		if b, err := os.ReadFile(filepath.Join(dir, rel)); err != nil || len(b) < 1000 {
			t.Errorf("%s: %d bytes, %v", rel, len(b), err)
		}
	}
}

// The committed source and the embedded blob are produced together by
// scripts/terminfo.sh; they must still describe the same terminal.
func TestEmbeddedEntryMatchesCommittedSource(t *testing.T) {
	for _, tool := range []string{"tic", "infocmp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " unavailable")
		}
	}
	embedded, compiled := t.TempDir(), t.TempDir()
	if err := Install(embedded); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("tic", "-x", "-o", compiled, "xterm-ghostty.terminfo").CombinedOutput(); err != nil {
		t.Fatalf("tic: %v\n%s", err, out)
	}
	describe := func(db string) string {
		out, err := exec.Command("infocmp", "-x", "-1", "-A", db, Term).CombinedOutput()
		if err != nil {
			t.Fatalf("infocmp -A %s: %v\n%s", db, err, out)
		}
		var keep []string
		for _, l := range strings.Split(string(out), "\n") {
			if !strings.HasPrefix(l, "#") { // the comment names the file read
				keep = append(keep, l)
			}
		}
		return strings.Join(keep, "\n")
	}
	if a, b := describe(embedded), describe(compiled); a != b {
		t.Fatalf("embedded entry differs from xterm-ghostty.terminfo; run scripts/terminfo.sh\n--- embedded\n%s\n--- source\n%s", a, b)
	}
}
