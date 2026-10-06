package tape

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	p, err := Parse(strings.NewReader("# comment\nOutput 'my demo.gif'\nSet Cols 42\nSet Rows 12\nSet FontSize 17\nHide\nType `printf 'hi'` # comment\nEnter\nShow\nSleep 250ms\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Config.Output != "my demo.gif" || p.Config.Cols != 42 || len(p.Actions) != 5 || p.Actions[1].Text != "printf 'hi'" || p.Actions[4].Duration != 250*time.Millisecond {
		t.Fatalf("unexpected tape: %+v", p)
	}
}

func TestRejectInvalid(t *testing.T) {
	for _, s := range []string{"Typo hi", "Set Theme dark", "Sleep -1s", "Type \"unclosed", "Enter hi", "Set Rows 0", "Set FontSize NaN", "Show\nSet Rows 3", "Output x.png", "Type \"x\"oops"} {
		t.Run(s, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(s)); err == nil {
				t.Fatal("accepted invalid tape")
			}
		})
	}
}

func TestOutputFormats(t *testing.T) {
	for _, path := range []string{"out.gif", "out.mp4", "out.webm", "out.MP4"} {
		p, err := Parse(strings.NewReader("Output " + path + "\nSleep 1s"))
		if err != nil {
			t.Fatal(err)
		}
		if p.Config.Output != path {
			t.Fatal(p.Config.Output)
		}
	}
}
