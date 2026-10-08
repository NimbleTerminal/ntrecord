package render

import (
	"strings"
	"testing"

	"nimbleterminal.dev/ntrecord/internal/tape"
)

func TestXtversionReportsConfiguredVersion(t *testing.T) {
	for version, want := range map[string]string{"v1.2.3": "ntrecord 1.2.3", "": "ntrecord dev"} {
		c := tape.DefaultConfig()
		c.Version = version
		r, err := New(c)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Write([]byte("\x1b[>q")); err != nil {
			t.Fatal(err)
		}
		reply := string(r.TakeReplies())
		r.Close()
		if !strings.Contains(reply, want) {
			t.Errorf("version %q: reply %q lacks %q", version, reply, want)
		}
	}
}
