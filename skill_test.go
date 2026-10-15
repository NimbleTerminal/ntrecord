package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nimbleterminal.dev/ntrecord/skills"
)

func TestSkillShow(t *testing.T) {
	out, _, err := runCLI(t, "skill", "show")
	if err != nil || out != skills.NTRecord {
		t.Fatalf("skill show: err=%v, printed the skill: %v", err, out == skills.NTRecord)
	}
	// Bare skill is help: the skill is long, so printing it takes asking.
	out, _, err = runCLI(t, "skill")
	if err != nil || !strings.Contains(out, "ntrecord skill install [FOLDER]") || strings.Contains(out, "name: ntrecord") {
		t.Fatalf("bare skill: err=%v\n%s", err, out)
	}
	if !strings.HasPrefix(skills.NTRecord, "---\nname: ntrecord\n") {
		t.Fatalf("embedded skill lacks its front matter:\n%.80s", skills.NTRecord)
	}
}

func TestSkillInstall(t *testing.T) {
	// A named skills folder gets ntrecord/SKILL.md, whole; the path is the answer.
	root := filepath.Join(t.TempDir(), "skills")
	out, _, err := runCLI(t, "skill", "install", root)
	if err != nil {
		t.Fatal(err)
	}
	written := filepath.Join(root, "ntrecord", "SKILL.md")
	if strings.TrimSpace(out) != written {
		t.Fatalf("stdout = %q, want %q", out, written)
	}
	content, err := os.ReadFile(written)
	if err != nil || string(content) != skills.NTRecord {
		t.Fatalf("%s: %v", written, err)
	}
	// Again, and it is an update, said as such.
	_, errOut, err := runCLI(t, "skill", "install", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut, "updated") {
		t.Fatalf("reinstall not called an update: %q", errOut)
	}
}

func TestSkillInstallAuto(t *testing.T) {
	// Detection looks in the home folder; give it one, with Claude Code in it.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KIMI_CODE_HOME", "")
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := installSkill("auto", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(home, ".claude", "skills", "ntrecord", "SKILL.md")
	shared := filepath.Join(home, ".agents", "skills", "ntrecord", "SKILL.md")
	if stdout.String() != claude+"\n"+shared+"\n" || !strings.Contains(stderr.String(), "Claude Code") || !strings.Contains(stderr.String(), "Codex") {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
	for _, path := range []string{claude, shared} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	// Kimi Code's home, moved by KIMI_CODE_HOME, is found as well.
	kimi := filepath.Join(home, "kimi-home")
	t.Setenv("KIMI_CODE_HOME", kimi)
	if err := os.Mkdir(kimi, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := installSkill("auto", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), filepath.Join(kimi, "skills", "ntrecord", "SKILL.md")) {
		t.Fatalf("Kimi Code's folder missed: %q", stdout.String())
	}
}

func TestSkillInstallSharedAlways(t *testing.T) {
	// The shared folder is written whether or not anything is found: the
	// agents that read it, Codex among them, do not make it.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KIMI_CODE_HOME", "")
	out, errOut, err := runCLI(t, "skill", "install")
	if err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(home, ".agents", "skills", "ntrecord", "SKILL.md")
	if strings.TrimSpace(out) != shared || !strings.Contains(errOut, "Codex") {
		t.Fatalf("stdout = %q, stderr = %q", out, errOut)
	}
	if content, err := os.ReadFile(shared); err != nil || string(content) != skills.NTRecord {
		t.Fatalf("%s: %v", shared, err)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if expandHome("~/skills") != filepath.Join(home, "skills") || expandHome("~") != home || expandHome("/tmp/x") != "/tmp/x" {
		t.Fatalf("expandHome: ~=%q ~/skills=%q", expandHome("~"), expandHome("~/skills"))
	}
}

func TestSkillVerbRejectsAndHelps(t *testing.T) {
	for _, args := range [][]string{{"skill", "uninstall"}, {"skill", "show", "x"}, {"skill", "install", "a", "b"}, {"skill", "--bogus"}, {"skill", "install", "-x"}} {
		if _, _, err := runCLI(t, args...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	// Help is help after the subcommand too, never a folder named --help.
	dir := t.TempDir()
	t.Chdir(dir)
	for _, args := range [][]string{{"skill", "--help"}, {"skill", "-h"}, {"skill", "install", "--help"}, {"skill", "install", "-h"}, {"skill", "show", "--help"}} {
		out, _, err := runCLI(t, args...)
		if err != nil || !strings.Contains(out, "ntrecord skill install [FOLDER]") {
			t.Fatalf("%v: err=%v\n%s", args, err, out)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("help wrote files: %v", entries)
	}
}

func TestSkillInstallReplacesDanglingLink(t *testing.T) {
	// A link left by a moved or deleted checkout points nowhere; it is
	// replaced by a real folder, as nothing is lost.
	root := t.TempDir()
	link := filepath.Join(root, "ntrecord")
	if err := os.Symlink(filepath.Join(root, "gone", "ntrecord"), link); err != nil {
		t.Fatal(err)
	}
	_, errOut, err := runCLI(t, "skill", "install", root)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || !info.IsDir() {
		t.Fatalf("%s is not a folder now: %v", link, err)
	}
	if !strings.Contains(errOut, "replaced a broken link") {
		t.Fatalf("stderr does not mention the link: %q", errOut)
	}
}

func TestSkillInstallRefusesLinkedFolder(t *testing.T) {
	// A link to a live folder, such as a source checkout's skills/ntrecord,
	// is someone's on purpose; it is not written through or replaced.
	root, target := t.TempDir(), t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "ntrecord")); err != nil {
		t.Fatal(err)
	}
	_, _, err := runCLI(t, "skill", "install", root)
	if err == nil || !strings.Contains(err.Error(), target) {
		t.Fatalf("err = %v, want one naming %s", err, target)
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Fatalf("wrote through the link: %v", entries)
	}
}
