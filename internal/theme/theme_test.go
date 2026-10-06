package theme

import (
	"encoding/json"
	"testing"
)

func TestAllBundledThemes(t *testing.T) {
	var themes []Theme
	if err := json.Unmarshal(data, &themes); err != nil {
		t.Fatal(err)
	}
	if len(themes) < 300 {
		t.Fatal("missing theme catalog")
	}
	for _, theme := range themes {
		if _, err := Parse(theme["name"]); err != nil {
			t.Errorf("%s: %v", theme["name"], err)
		}
	}
}
