package render

import (
	"math"
	"testing"
)

func TestConfigRejectsUnsafeGeometry(t *testing.T) {
	for name, change := range map[string]func(*Config){
		"negative padding": func(c *Config) { c.Padding = -1 },
		"overflow margin":  func(c *Config) { c.Margin = math.MaxInt },
		"negative height":  func(c *Config) { c.Height = -1 },
		"NaN spacing":      func(c *Config) { c.LetterSpacing = math.NaN() },
		"infinite height":  func(c *Config) { c.LineHeight = math.Inf(1) },
		"invalid bar":      func(c *Config) { c.WindowBar = "bogus" },
		"invalid fill":     func(c *Config) { c.MarginFill = "not-a-color" },
	} {
		t.Run(name, func(t *testing.T) {
			c := DefaultConfig()
			change(&c)
			r, err := New(c)
			if r != nil {
				r.Close()
			}
			if err == nil {
				t.Fatal("invalid public config accepted")
			}
		})
	}
}
