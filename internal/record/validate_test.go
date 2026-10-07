package record

import (
	"os"
	"strings"
	"testing"

	"github.com/NimbleMarkets/sigmamax/internal/tape"
)

func parse(t *testing.T, source string) *tape.Tape {
	t.Helper()
	p, err := tape.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestValidateSummarizesWithoutRecording(t *testing.T) {
	dir := t.TempDir()
	p := parse(t, "Output "+dir+"/a.gif\nOutput "+dir+"/a.txt\nSet Columns 40\nSet Rows 10\nType \"hi\"\nEnter\nSleep 1s\n")
	s, err := Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Commands != 3 || s.Cols != 40 || s.Rows != 10 || s.Width < 40 || s.Height < 10 || len(s.Outputs) != 2 {
		t.Fatalf("%+v", s)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("validate wrote files: %v", entries)
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // nothing installed
	p := parse(t, "Output a.mp4\nRequire no-such-program\nSet FontFamily \"No Such Font Family 123\"\nType \"x\"\n")
	_, err := Validate(p)
	if err == nil {
		t.Fatal("expected problems")
	}
	for _, want := range []string{"no-such-program", "requires ffmpeg", "No Such Font Family 123"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
}

func TestValidateHonorsEnvPathForRequire(t *testing.T) {
	// Require looks in the recording's PATH, which a tape may change with Env.
	p := parse(t, "Output a.gif\nEnv PATH /bin:/usr/bin\nRequire sh\nType \"x\"\n")
	if _, err := Validate(p); err != nil {
		t.Fatal(err)
	}
}
