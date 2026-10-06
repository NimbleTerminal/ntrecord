// Package theme loads VHS-compatible named themes and inline theme objects.
package theme

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// Themes are data from Charm's VHS, revision
// 24fa2254a9806091e6ee6a980e9f3bcfe0a9ba53. See LICENSE in this directory.
//
//go:embed themes.json
var data []byte

type Theme map[string]string

func (t *Theme) UnmarshalJSON(b []byte) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(b, &values); err != nil {
		return err
	}
	*t = make(Theme)
	for k, v := range values {
		if k == "meta" {
			continue
		} // preserve upstream credits in the embedded data
		var text string
		if err := json.Unmarshal(v, &text); err != nil {
			return err
		}
		(*t)[k] = text
	}
	return nil
}

func Parse(s string) (Theme, error) {
	if s == "" {
		return Theme{}, nil
	}
	var t Theme
	if strings.HasPrefix(strings.TrimSpace(s), "{") {
		if err := json.Unmarshal([]byte(s), &t); err != nil {
			return nil, err
		}
	} else {
		var all []Theme
		if err := json.Unmarshal(data, &all); err != nil {
			return nil, err
		}
		for _, candidate := range all {
			if strings.EqualFold(candidate["name"], s) {
				t = candidate
				break
			}
		}
		if t == nil {
			return nil, fmt.Errorf("unknown theme %q; use a VHS theme name or inline JSON", s)
		}
	}
	for alias, key := range map[string]string{"purple": "magenta", "brightPurple": "brightMagenta", "cursorColor": "cursor"} {
		if v, ok := t[alias]; ok {
			if _, exists := t[key]; !exists {
				t[key] = v
			}
			delete(t, alias)
		}
	}
	for k, v := range t {
		switch k {
		case "name":
			continue
		case "background", "foreground", "cursor", "cursorAccent", "selection", "selectionBackground", "selectionForeground", "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite":
		default:
			return nil, fmt.Errorf("unknown theme color %q", k)
		}
		if v != "" {
			if _, err := Color(v); err != nil {
				return nil, fmt.Errorf("theme %s: %w", k, err)
			}
		}
	}
	return t, nil
}

func Color(s string) (color.RGBA, error) {
	if len(s) == 4 && s[0] == '#' {
		s = fmt.Sprintf("#%c%c%c%c%c%c", s[1], s[1], s[2], s[2], s[3], s[3])
	}
	if len(s) != 7 || s[0] != '#' {
		return color.RGBA{}, fmt.Errorf("expected #RRGGBB color, got %q", s)
	}
	v, e := strconv.ParseUint(s[1:], 16, 24)
	if e != nil {
		return color.RGBA{}, fmt.Errorf("invalid color %q", s)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}
