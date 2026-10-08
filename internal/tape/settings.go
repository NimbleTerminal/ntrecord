package tape

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"nimbleterminal.dev/ntrecord/internal/theme"
)

func (p *parser) setting(key, value string) error {
	c := &p.tape.Config
	num := func(min, max float64) (float64, error) {
		v, e := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64)
		if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
			return 0, fmt.Errorf("%s must be between %g and %g", key, min, max)
		}
		return v, nil
	}
	integer := func(min, max int) (int, error) {
		v, e := num(float64(min), float64(max))
		if e != nil {
			return 0, e
		}
		if v != math.Trunc(v) {
			return 0, fmt.Errorf("%s must be an integer", key)
		}
		return int(v), nil
	}
	var err error
	switch key {
	case "Shell":
		if value == "" {
			return fmt.Errorf("Shell cannot be empty")
		}
		c.Shell = value
	case "FontFamily":
		if value == "" {
			return fmt.Errorf("FontFamily cannot be empty")
		}
		c.FontFamily = value
	case "FontSize":
		c.FontSize, err = num(6, 96)
	case "Cols", "Columns", "Rows", "Width", "Height":
		if key == "Cols" {
			key = "Columns"
		}
		opposite := map[string]string{"Columns": "Width", "Width": "Columns", "Rows": "Height", "Height": "Rows"}[key]
		if p.dimensions[opposite] {
			return fmt.Errorf("Set %s conflicts with Set %s", key, opposite)
		}
		p.dimensions[key] = true
		var n int
		n, err = integer(1, 8192)
		switch key {
		case "Columns":
			c.Cols = n
		case "Rows":
			c.Rows = n
		case "Width":
			c.Width = n
		case "Height":
			c.Height = n
		}
	case "TypingSpeed":
		p.speed, err = duration(value)
		if !p.started {
			c.TypingSpeed = p.speed
		}
	case "WaitTimeout":
		c.WaitTimeout, err = duration(value)
		if err == nil && c.WaitTimeout <= 0 {
			err = fmt.Errorf("WaitTimeout must be positive")
		}
	case "WaitPattern":
		_, err = regexp.Compile(value)
		c.WaitPattern = value
	case "Framerate":
		c.Framerate, err = num(1, 120)
	case "PlaybackSpeed":
		c.PlaybackSpeed, err = num(0.01, 100)
	case "LetterSpacing":
		c.LetterSpacing, err = num(-20, 100)
	case "LineHeight":
		c.LineHeight, err = num(0.5, 5)
	case "Padding":
		c.Padding, err = integer(0, 2048)
	case "Margin":
		c.Margin, err = integer(0, 2048)
	case "BorderRadius":
		c.BorderRadius, err = integer(0, 2048)
	case "WindowBarSize":
		c.WindowBarSize, err = integer(0, 512)
	case "WindowBar":
		switch strings.ToLower(value) {
		case "none", "", "colorful", "colorfulright", "rings", "ringsright":
			c.WindowBar = strings.ToLower(value)
		default:
			err = fmt.Errorf("unknown WindowBar %q", value)
		}
	case "MarginFill":
		_, err = theme.Color(value)
		c.MarginFill = value
	case "Theme":
		_, err = theme.Parse(value)
		c.Theme = value
	case "CursorBlink":
		c.CursorBlink, err = strconv.ParseBool(value)
	case "LoopOffset":
		if strings.HasSuffix(value, "%") {
			v, e := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
			if e != nil || math.IsNaN(v) || v < 0 || v >= 100 {
				err = fmt.Errorf("LoopOffset percentage must be in [0,100)")
			}
		} else {
			v, e := strconv.ParseFloat(value, 64)
			if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1e9 {
				err = fmt.Errorf("LoopOffset must be a nonnegative frame index or percentage")
			}
		}
		c.LoopOffset = value
	default:
		err = fmt.Errorf("unknown setting %q", key)
	}
	return err
}
