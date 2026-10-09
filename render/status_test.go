package render

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func report(body string) string { return "\x1b]7501;" + body + "\x1b\\" }

func TestProgramStatusProtocol(t *testing.T) {
	r := newRenderer(t)
	before, err := r.Frame()
	if err != nil {
		t.Fatal(err)
	}
	pixels := append([]byte(nil), before.Pix...)
	// Exercise fragmented sequences and a real capability query.
	stream := report("?") + report("state=blocked:kind=question:progress=42:app=agent:id=job:title=VGl0bGU=:msg=SGVsbG8=")
	for _, b := range []byte(stream) {
		write(t, r, string([]byte{b}))
	}
	if reply := string(r.TakeReplies()); reply != report("?") {
		t.Fatalf("reply %q", reply)
	}
	want := []ProgramStatus{{ID: "job", State: "blocked", Kind: "question", Progress: 42, App: "agent", Title: "Title", Message: "Hello"}}
	if got := r.ProgramStatuses(); !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	frame, err := r.Frame()
	if err != nil || !reflect.DeepEqual(pixels, frame.Pix) {
		t.Fatal("status changed pixels", err)
	}
	snapshot := r.ProgramStatuses()
	snapshot[0].State = "error"
	write(t, r, report("state=done:id=job"))
	want = []ProgramStatus{{ID: "job", State: "done", Progress: -1}}
	if got := r.ProgramStatuses(); !reflect.DeepEqual(got, want) {
		t.Fatalf("replacement retained fields: %+v", got)
	}
	for _, bad := range []string{"state=unknown:id=job", "state=error:id=job:msg=A", "state=error:id=job:msg=Gwo=", "state=error:id=/", "state=error:id=job:msg=" + strings.Repeat("A", 2800)} {
		write(t, r, report(bad))
		if got := r.ProgramStatuses(); !reflect.DeepEqual(got, want) {
			t.Fatalf("invalid report applied: %q: %+v", bad, got)
		}
	}
}

func TestProgramStatusLifecycle(t *testing.T) {
	r := newRenderer(t)
	write(t, r, report("state=idle:app=root")+report("state=working:id=build:app=compiler")+report("state=blocked:id=build/test")+report("state=done:id=build2"))
	got := r.ProgramStatuses()
	if got[2].App != "compiler" || got[3].App != "root" {
		t.Fatalf("inheritance: %+v", got)
	}
	write(t, r, report("state=clear:id=build"))
	if got := r.ProgramStatuses(); len(got) != 2 || got[1].ID != "build2" {
		t.Fatalf("subtree clear: %+v", got)
	}
	for _, boundary := range []string{"prompt", "exit"} {
		write(t, r, report("state=working:id=w")+report("state=blocked:id=b")+report("state=error:id=e"))
		if boundary == "prompt" {
			write(t, r, "\x1b]133;A\x1b\\")
		} else {
			r.ProgramExited()
		}
		if got := r.ProgramStatuses(); len(got) != 3 || got[2].State != "error" {
			t.Fatalf("%s: %+v", boundary, got)
		}
	}
	write(t, r, "\x1b[?1049h\x1b[?1049l\x1b[!p")
	if len(r.ProgramStatuses()) != 3 {
		t.Fatal("screen switch or soft reset cleared status")
	}
	write(t, r, "\x1bc")
	if len(r.ProgramStatuses()) != 0 {
		t.Fatal("RIS retained status")
	}
	write(t, r, report("state=done:id=x")+report("state=clear"))
	if len(r.ProgramStatuses()) != 0 {
		t.Fatal("root clear retained status")
	}
}

func TestProgramStatusBoundedEviction(t *testing.T) {
	r := newRenderer(t)
	for i := 0; i < 256; i++ {
		write(t, r, report(fmt.Sprintf("state=working:id=j%03d", i)))
	}
	write(t, r, report("state=done:id=j000")+report("state=idle:id=new"))
	got := r.ProgramStatuses()
	if len(got) != 256 {
		t.Fatal(len(got))
	}
	for _, s := range got {
		if s.ID == "j001" {
			t.Fatal("least recently updated record survived")
		}
	}
	if got[0].ID != "j000" || got[0].State != "done" {
		t.Fatal("updated record evicted")
	}
}
