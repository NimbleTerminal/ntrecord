package render

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	vt "go.mitchellh.com/libghostty"
)

// Key uses libghostty's keyboard encoder so application cursor mode and Kitty
// keyboard flags are read from the same terminal that receives PTY output.
func (r *Renderer) Key(spec string) ([]byte, error) {
	parts := strings.Split(spec, "+")
	n := 0
	for n < len(parts) && (parts[n] == "Ctrl" || parts[n] == "Alt" || parts[n] == "Shift") {
		n++
	}
	if n > 0 && len(parts)-n > 1 {
		var out []byte
		for _, key := range parts[n:] {
			b, e := r.Key(strings.Join(parts[:n], "+") + "+" + key)
			if e != nil {
				return nil, e
			}
			out = append(out, b...)
		}
		return out, nil
	}
	name := parts[len(parts)-1]
	var mods vt.Mods
	for _, s := range parts[:len(parts)-1] {
		switch s {
		case "Ctrl":
			mods |= vt.ModCtrl
		case "Alt":
			mods |= vt.ModAlt
		case "Shift":
			mods |= vt.ModShift
		default:
			return nil, fmt.Errorf("invalid modifier %q", s)
		}
	}
	names := map[string]string{"Up": "arrow_up", "Down": "arrow_down", "Left": "arrow_left", "Right": "arrow_right", "PageUp": "page_up", "PageDown": "page_down"}
	keyName := strings.ToLower(name)
	if n, ok := names[name]; ok {
		keyName = n
	}
	var text string
	var cp rune
	if utf8.RuneCountInString(name) == 1 {
		cp, _ = utf8.DecodeRuneInString(name)
		cp = unicode.ToLower(cp)
		switch {
		case cp >= 'a' && cp <= 'z':
			keyName = "key_" + string(cp)
		case cp >= '0' && cp <= '9':
			keyName = "digit_" + string(cp)
		default:
			punctuation := map[rune]string{'@': "digit_2", '[': "bracket_left", ']': "bracket_right", '\\': "backslash", '^': "digit_6", '_': "minus", '-': "minus", '/': "slash", '.': "period", ',': "comma", '=': "equal", ';': "semicolon", '\'': "quote", '`': "backquote", '?': "slash"}
			keyName = punctuation[cp]
		}
		text = string(cp)
		if mods&vt.ModShift != 0 {
			text = strings.ToUpper(text)
		}
	}
	if name == "Space" {
		text = " "
		cp = ' '
	}
	key, err := vt.ParseKey(keyName)
	if err != nil {
		return nil, err
	}
	return r.encodeKey(key, mods, text, cp)
}

func (r *Renderer) TypeRune(ch rune) ([]byte, error) {
	switch ch {
	case '\n', '\r':
		return r.Key("Enter")
	case '\t':
		return r.Key("Tab")
	}
	cp := unicode.ToLower(ch)
	key := vt.KeyUnidentified
	if cp >= 'a' && cp <= 'z' {
		key, _ = vt.ParseKey("key_" + string(cp))
	} else if cp >= '0' && cp <= '9' {
		key, _ = vt.ParseKey("digit_" + string(cp))
	} else if ch == ' ' {
		key = vt.KeySpace
	}
	return r.encodeKey(key, 0, string(ch), cp)
}

func (r *Renderer) encodeKey(key vt.Key, mods vt.Mods, text string, cp rune) ([]byte, error) {
	enc, err := vt.NewKeyEncoder()
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	enc.SetOptFromTerminal(r.term)
	enc.SetOptOptionAsAlt(vt.OptionAsAltTrue)
	event, err := vt.NewKeyEvent()
	if err != nil {
		return nil, err
	}
	defer event.Close()
	event.SetKey(key)
	event.SetMods(mods)
	event.SetUTF8(text)
	event.SetUnshiftedCodepoint(cp)
	event.SetAction(vt.KeyActionPress)
	down, err := enc.Encode(event)
	if err != nil {
		return nil, err
	}
	event.SetAction(vt.KeyActionRelease)
	up, err := enc.Encode(event)
	if err != nil {
		return nil, err
	}
	return append(down, up...), nil
}

func (r *Renderer) Paste(text string) ([]byte, error) {
	bracketed, err := r.term.Mode(vt.ModeBracketedPaste)
	if err != nil {
		return nil, err
	}
	return vt.PasteEncode([]byte(text), bracketed)
}

func (r *Renderer) Scroll(delta int) error {
	r.term.ScrollViewportDelta(delta)
	return r.snapshotImages()
}

// Text reads displayed cells, not the PTY byte stream: erased text and escape
// sequences cannot satisfy Wait. Line means the current cursor row.
func (r *Renderer) Text(scope string) (string, error) {
	if err := r.state.Update(r.term); err != nil {
		return "", err
	}
	cursor, err := r.state.Cursor()
	if err != nil {
		return "", err
	}
	if err = r.state.RowIterator(r.rows); err != nil {
		return "", err
	}
	var lines []string
	for y := 0; r.rows.Next(); y++ {
		b, e := r.rows.AppendText(nil, r.cells)
		if e != nil {
			return "", e
		}
		s := strings.TrimRight(string(b), " ")
		if scope == "Line" && cursor.ViewportHasValue && y == int(cursor.ViewportY) {
			return s, nil
		}
		lines = append(lines, s)
	}
	if scope == "Line" {
		return "", nil
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n"), nil
}
