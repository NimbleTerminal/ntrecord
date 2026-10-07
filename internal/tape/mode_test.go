package tape

import "testing"

func TestParseMode(t *testing.T) {
	for in, want := range map[string]Mode{"": Realtime, "realtime": Realtime, "deterministic": Deterministic} {
		if got, err := ParseMode(in); err != nil || got != want {
			t.Errorf("ParseMode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseMode("fast"); err == nil {
		t.Error("an unknown mode was accepted")
	}
	if DefaultConfig().Mode != Realtime {
		t.Errorf("the default mode is %q; existing recordings must keep working", DefaultConfig().Mode)
	}
}
