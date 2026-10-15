package tape

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLanguage(t *testing.T) {
	input := `Output a.gif
Output a.mp4
Output frames/
Require sh
Env HELLO "world"
Set TypingSpeed 0.1
Set WaitTimeout 2s
Set WaitPattern /ready # [0-9]+/
Set Theme {
 "background": "#123456", "foreground": "#ffffff"
}
Type@5ms "printf '\n'" Enter 2
Set TypingSpeed 50 ms
Ctrl+Alt+Shift+R
Tab@0.2 3
Sleep 0.5
Wait+Screen@1s /https:\/\/example.com # ready/
Copy 'hello'
Paste
Screenshot /tmp/frame.png
ScrollUp 2
ScrollDown
`
	p, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Config.Outputs) != 3 || p.Config.Env["HELLO"] != "world" {
		t.Fatalf("bad config: %+v", p.Config)
	}
	a := p.Actions
	if a[0].Text != `printf '\n'` || a[0].Interval != 5*time.Millisecond || a[1].Count != 2 || a[2].Interval != 50*time.Millisecond || a[3].Interval != 200*time.Millisecond || a[4].Duration != 500*time.Millisecond || a[5].Text != `https://example.com # ready` {
		t.Fatalf("bad actions: %+v", a)
	}
}

func TestSourceDiagnosticsAndCycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "child.tape")
	if err := os.WriteFile(path, []byte("Output ignored.gif\nSet TypingSpeed 1ms\nType child\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := Parse(strings.NewReader("Output parent.gif\nSource '" + path + "'\nEnter"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Config.Outputs) != 1 || p.Config.Output != "parent.gif" || p.Actions[0].Source != path {
		t.Fatalf("bad source result %+v", p)
	}
	if err := os.WriteFile(path, []byte("Source '"+path+"'"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("missing cycle error: %v", err)
	}
	if err := os.WriteFile(path, []byte("Sleep 1\nUnknown"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path); err == nil || !strings.Contains(err.Error(), path+":2:1") {
		t.Fatalf("missing source location: %v", err)
	}
}

func TestRejectInvalidVHS(t *testing.T) {
	for _, input := range []string{
		"Output@1 x.gif", "Set@1 Rows 20", "Wait@0 /x/", "Wait /[/", "Wait /unclosed", "Set Unknown 1", "Set Theme {broken}",
		"Set Framerate NaN", "Set PlaybackSpeed 0", "Set LineHeight 0", "Set Theme missing-theme",
		"Set Theme {\"foreground\":\"bad\"}", "Set Width 400\nSet Columns 40", "Set Rows 20\nSet Height 500",
		"Set MarginFill bad", "Set WindowBar mystery", "Set LoopOffset 100%", "Set LoopOffset -2",
		"Set CursorBlink maybe", "Type x\nEnv PATH /bin", "Ctrl+Banana", "Enter -1", "Output a.gif\nOutput a.gif",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(input)); err == nil {
				t.Fatal("accepted invalid tape")
			}
		})
	}
}

func TestSpacedModifiersAndTiming(t *testing.T) {
	p, err := Parse(strings.NewReader("Type @ 10 ms \"text\"\nCtrl + Shift + R\nWait + Screen @ 2 s /ready/"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Actions[0].Interval != 10*time.Millisecond || p.Actions[1].Command != "Ctrl+Shift+R" || p.Actions[2].Duration != 2*time.Second {
		t.Fatalf("bad actions: %+v", p.Actions)
	}
}

// VHS tokenizes "Type@.2'text'" as Type, @, .2, 'text': a quoted string needs
// no space after the timing that precedes it (the jqp example relies on this).
func TestQuoteMayFollowTimingDirectly(t *testing.T) {
	for _, tc := range []struct {
		input    string
		text     string
		interval time.Duration
	}{
		{`Type@.2'[ .products[] ]'`, `[ .products[] ]`, 200 * time.Millisecond},
		{`Type@50ms"hi there"`, `hi there`, 50 * time.Millisecond},
		{"Type@1s`back`", `back`, time.Second},
		{`Type@.2 'spaced'`, `spaced`, 200 * time.Millisecond},
	} {
		p, err := Parse(strings.NewReader(tc.input + "\nEnter"))
		if err != nil {
			t.Errorf("%s: %v", tc.input, err)
			continue
		}
		if a := p.Actions[0]; a.Command != "Type" || a.Text != tc.text || a.Interval != tc.interval {
			t.Errorf("%s: %+v", tc.input, a)
		}
	}
	// A quote inside an ordinary word is not a timing boundary.
	if _, err := Parse(strings.NewReader(`Wait+Screen /a'b/`)); err != nil {
		t.Errorf("regex with a quote: %v", err)
	}
}

// Wait arms a regular expression only as a command, and WaitPattern only as
// the setting: elsewhere they are words, and a / after them is not a regex.
func TestWaitWordIsNotARegexOutsideCommands(t *testing.T) {
	ts, err := lex("Type WaitPattern /usr/bin/env\nSet FontFamily Wait /usr/x.ttf\nSet WaitPattern /a b/\nWait /c d/\nWait+Screen@1s /e/")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tok := range ts {
		got = append(got, string(tok.kind)+":"+tok.text)
	}
	want := "w:Type w:WaitPattern w:/usr/bin/env w:Set w:FontFamily w:Wait w:/usr/x.ttf w:Set w:WaitPattern r:a b w:Wait r:c d w:Wait+Screen@1s r:e"
	if strings.Join(got, " ") != want {
		t.Fatalf("tokens:\n%s\nwant:\n%s", strings.Join(got, " "), want)
	}
	p, err := Parse(strings.NewReader("Type WaitPattern /usr/bin/env"))
	if err != nil || p.Actions[0].Text != "WaitPattern /usr/bin/env" {
		t.Fatalf("Type: %v %+v", err, p)
	}
}

// Ctrl+@ is a key, and like any key it takes a timing.
func TestCtrlAtTakesTiming(t *testing.T) {
	for _, source := range []string{"Ctrl+@100ms", "Ctrl+@@100ms"} {
		p, err := Parse(strings.NewReader(source))
		if err != nil {
			t.Errorf("%s: %v", source, err)
			continue
		}
		if a := p.Actions[0]; a.Command != "Ctrl+@" || a.Interval != 100*time.Millisecond {
			t.Errorf("%s: %+v", source, a)
		}
	}
	p, err := Parse(strings.NewReader("Ctrl+@"))
	if err != nil || p.Actions[0].Command != "Ctrl+@" {
		t.Fatalf("Ctrl+@: %v %+v", err, p)
	}
}
