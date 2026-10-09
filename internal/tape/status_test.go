package tape

import (
	"strings"
	"testing"
	"time"
)

func TestStatusWaitParsing(t *testing.T) {
	p, err := Parse(strings.NewReader("Wait+Status@2s blocked app=agent id=job/test kind=question\nWait+Status done id=\nWait+Status idle\n"))
	if err != nil {
		t.Fatal(err)
	}
	a := p.Actions[0]
	if a.Command != "Wait" || a.Scope != "Status" || a.Duration != 2*time.Second || !a.Status.Matches("blocked", "agent", "job/test", "question") {
		t.Fatalf("%+v", a)
	}
	if a.Status.Matches("blocked", "other", "job/test", "question") || a.Status.Matches("blocked", "agent", "job", "question") || a.Status.Matches("blocked", "agent", "job/test", "auth") {
		t.Fatal("filters ignored")
	}
	if !p.Actions[1].Status.Matches("done", "anything", "", "") || p.Actions[1].Status.Matches("done", "anything", "child", "") {
		t.Fatal("root ID filter")
	}
	if !p.Actions[2].Status.Matches("idle", "anything", "child", "") {
		t.Fatal("omitted ID should match any record")
	}
	for _, source := range []string{"Wait+Status", "Wait+Status clear", "Wait+Status@0s done", "Wait+Status done kind=auth", "Wait+Status blocked kind=oops", "Wait+Status done app=", "Wait+Status done id=/bad", "Wait+Status done id=a id=b", "Wait+Status done nope=x", "Wait+Status done id=" + strings.Repeat("x/", 8) + "x"} {
		if _, err := Parse(strings.NewReader(source)); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}
