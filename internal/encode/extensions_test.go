package encode

import (
	"strings"
	"testing"
)

// Format, its error message, and the CLI help all derive from Extensions, so
// an accepted extension can never be missing from the documentation of -o.
func TestFormatAcceptsAndReportsEveryExtension(t *testing.T) {
	if len(Extensions) == 0 {
		t.Fatal("no extensions")
	}
	for _, ext := range Extensions {
		if _, err := Format("out" + ext); err != nil {
			t.Errorf("Format rejects listed extension %s: %v", ext, err)
		}
	}
	_, err := Format("out.bogus")
	if err == nil {
		t.Fatal("bogus extension accepted")
	}
	for _, ext := range Extensions {
		if !strings.Contains(err.Error(), ext) {
			t.Errorf("error %q does not mention %s", err, ext)
		}
	}
	if !strings.Contains(err.Error(), "/") || !strings.Contains(OutputHelp(), "frames/") {
		t.Error("frame directories not described")
	}
	for _, ext := range Extensions {
		if !strings.Contains(OutputHelp(), ext) {
			t.Errorf("OutputHelp omits %s", ext)
		}
	}
}
