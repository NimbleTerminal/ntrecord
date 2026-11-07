package theme

import (
	"encoding/json"
	"fmt"
	"maps"
	"testing"
)

// FuzzParse tests theme and hex color parsing, verifying that accepted themes
// contain only valid colors and canonical keys and survive JSON round-tripping.
func FuzzParse(f *testing.F) {
	f.Add("")
	f.Add("Dracula")
	f.Add("Catppuccin Mocha")
	f.Add("#181a20")
	f.Add("#abc")
	f.Add(`{"background":"#181a20","foreground":"#e0e4eb","purple":"#c678dd","brightPurple":"#e5c07b","cursorColor":"#ffffff","meta":{"credit":"x"}}`)
	f.Add(`{"unknown":"#000000"}`)

	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 16*1024 {
			return
		}
		if c, err := Color(s); err == nil {
			if c.A != 255 {
				t.Fatalf("Color(%q) alpha = %d, want 255", s, c.A)
			}
			hex := fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
			if again, err := Color(hex); err != nil || again != c {
				t.Fatalf("Color round-trip %q -> %q: %v (%v)", s, hex, again, err)
			}
		}

		th, err := Parse(s)
		if err != nil {
			return
		}
		for _, alias := range []string{"purple", "brightPurple", "cursorColor", "meta"} {
			if _, ok := th[alias]; ok {
				t.Fatalf("Parse(%q) retained alias or meta key %q", s, alias)
			}
		}
		for k, v := range th {
			if k == "name" || v == "" {
				continue
			}
			if _, err := Color(v); err != nil {
				t.Fatalf("Parse(%q) kept invalid color %s=%q: %v", s, k, v, err)
			}
		}
		if len(th) == 0 {
			return
		}
		encoded, err := json.Marshal(th)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		again, err := Parse(string(encoded))
		if err != nil {
			t.Fatalf("re-parse marshaled theme %s: %v", encoded, err)
		}
		if !maps.Equal(th, again) {
			t.Fatalf("theme round-trip mismatch:\ngot:  %v\nwant: %v", again, th)
		}
	})
}
