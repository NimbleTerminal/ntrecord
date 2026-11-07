package tape

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

// FuzzParse feeds arbitrary VHS tape scripts to the lexer and parser with
// Source isolated to an in-memory map, checking that every accepted Tape
// satisfies configuration and action invariants.
func FuzzParse(f *testing.F) {
	f.Add("Output demo.gif\nRequire sh\nEnv FOO bar\nSet Shell /bin/sh\nSet FontSize 18\nSet Cols 80\nSet Rows 24\nSet Framerate 30\nSet PlaybackSpeed 1.5\nSet LetterSpacing 1\nSet LineHeight 1.2\nSet Padding 10\nSet Margin 10\nSet MarginFill #112233\nSet BorderRadius 8\nSet WindowBar colorful\nSet WindowBarSize 30\nSet Theme Dracula\nSet CursorBlink true\nSet LoopOffset 25%\nSet TypingSpeed 20ms\nSet WaitTimeout 5s\nSet WaitPattern />$/\nHide\nType@50ms \"hello\" 'world' `literal`\nCopy \"clip\"\nPaste\nEnter 2\nCtrl+C\nCtrl+@\nAlt+Enter\nShift+Tab\nWait+Line /prompt/\nWait+Screen@2s /ready/\nWait+Status@3s blocked app=claude id=main/sub kind=permission\nScreenshot shot.png\nShow\nSleep 500ms\n")
	f.Add("Source sub.tape\nType 'after source'\nEnter\n")
	f.Add("Set Theme {\"background\":\"#181a20\",\"foreground\":\"#e0e4eb\",\"purple\":\"#c678dd\",\"cursorColor\":\"#ffffff\"}\nSleep 100ms\n")
	f.Add("Source cycle.tape\n")
	f.Add("Wait+Status idle app=my_app id=\n")

	sources := map[string]string{
		"sub.tape":   "Set TypingSpeed 10ms\nType 'from source'\nEnter\n",
		"cycle.tape": "Source cycle.tape\n",
	}
	open := func(name string) (io.ReadCloser, error) {
		if content, ok := sources[name]; ok {
			return io.NopCloser(strings.NewReader(content)), nil
		}
		return nil, os.ErrNotExist
	}

	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 32*1024 {
			return
		}
		_, _ = ParseMode(src)
		parsed, err := parse(strings.NewReader(src), "<fuzz>", open)
		if err != nil {
			return
		}
		if parsed == nil {
			t.Fatal("nil Tape without error")
		}
		c := parsed.Config
		if c.FontSize < 6 || c.FontSize > 96 {
			t.Fatalf("FontSize out of bounds: %v", c.FontSize)
		}
		if c.Cols < 1 || c.Cols > 8192 || c.Rows < 1 || c.Rows > 8192 {
			t.Fatalf("grid out of bounds: %dx%d", c.Cols, c.Rows)
		}
		if c.Width != 0 && (c.Width < 1 || c.Width > 8192) || c.Height != 0 && (c.Height < 1 || c.Height > 8192) {
			t.Fatalf("pixel dimensions out of bounds: %dx%d", c.Width, c.Height)
		}
		if c.Framerate < 1 || c.Framerate > 120 {
			t.Fatalf("Framerate out of bounds: %v", c.Framerate)
		}
		if c.PlaybackSpeed < 0.01 || c.PlaybackSpeed > 100 {
			t.Fatalf("PlaybackSpeed out of bounds: %v", c.PlaybackSpeed)
		}
		if c.LineHeight < 0.5 || c.LineHeight > 5 {
			t.Fatalf("LineHeight out of bounds: %v", c.LineHeight)
		}
		if c.WaitTimeout <= 0 {
			t.Fatalf("nonpositive WaitTimeout: %v", c.WaitTimeout)
		}
		if len(c.Destinations()) == 0 {
			t.Fatal("empty Destinations")
		}
		for k, v := range c.Env {
			if k == "" || strings.ContainsRune(v, 0) {
				t.Fatalf("invalid env %q=%q", k, v)
			}
		}
		for i, a := range parsed.Actions {
			if a.Command == "" {
				t.Fatalf("action %d has empty Command", i)
			}
			if a.Line < 1 || a.Column < 1 {
				t.Fatalf("action %d has invalid position %d:%d", i, a.Line, a.Column)
			}
			if a.Count < 1 || a.Count > 1000000 {
				t.Fatalf("action %d has out-of-range Count %d", i, a.Count)
			}
			if a.Duration < 0 || a.Interval < 0 {
				t.Fatalf("action %d has negative timing: duration=%v interval=%v", i, a.Duration, a.Interval)
			}
			if a.Command == "Wait" {
				switch a.Scope {
				case "Line", "Screen":
					if _, err := regexp.Compile(a.Text); err != nil {
						t.Fatalf("action %d has invalid Wait regex %q: %v", i, a.Text, err)
					}
				case "Status":
					parts := strings.Fields(a.Status.String())
					if len(parts) == 0 {
						t.Fatalf("action %d has empty StatusCondition", i)
					}
					cond, err := parseStatusCondition(parts[0], parts[1:])
					if err != nil || cond != a.Status {
						t.Fatalf("StatusCondition round-trip: %+v -> %q -> %+v (%v)", a.Status, a.Status.String(), cond, err)
					}
					if !a.Status.Matches(a.Status.State, a.Status.App, a.Status.ID, a.Status.Kind) {
						t.Fatalf("StatusCondition %+v does not match its own fields", a.Status)
					}
				default:
					t.Fatalf("action %d has unknown Wait scope %q", i, a.Scope)
				}
			}
			if a.Error(fmt.Errorf("boom")) == nil {
				t.Fatalf("action %d Error returned nil", i)
			}
		}
	})
}
