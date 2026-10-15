package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"nimbleterminal.dev/ntrecord/skills"
)

const skillUsage = `Usage: ntrecord skill [show]
       ntrecord skill install [FOLDER]

show (the default) prints the skill that teaches an agent to use ntrecord.
install writes it as ntrecord/SKILL.md under the shared ~/.agents/skills folder
and the skills folder of each agent found here (Claude Code, Kimi Code), or
under FOLDER alone. The paths written go to stdout, one to a line.`

// skillCommand is the skill verb: show the skill, or install it.
func skillCommand(args []string, stdout, stderr io.Writer) error {
	// Help is help wherever it comes, and the verb has no other option, so an
	// option is never taken for a folder.
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, skillUsage)
			return nil
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("skill: unknown option %s (see ntrecord skill --help)", a)
		}
	}
	sub := "show"
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	switch {
	case sub == "show" && len(args) == 0:
		_, err := fmt.Fprint(stdout, skills.NTRecord)
		return err
	case sub == "install" && len(args) == 0:
		return installSkill("auto", stdout, stderr)
	case sub == "install" && len(args) == 1:
		return installSkill(args[0], stdout, stderr)
	case sub == "show" || sub == "install":
		return fmt.Errorf("skill %s: too many arguments (see ntrecord skill --help)", sub)
	}
	return fmt.Errorf("skill: unknown command %q: show or install (see ntrecord skill --help)", sub)
}

// skillHome is a skills folder an agent reads, detected by the presence of
// detect: the agent's home. The shared folder has no detect: agents that read
// it, Codex among them, do not make it, so it is always written.
type skillHome struct {
	agent, detect, root string
}

// skillHomes are the user-level skills folders ntrecord knows, in the order
// they are reported.
func skillHomes() []skillHome {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	kimi := os.Getenv("KIMI_CODE_HOME")
	if kimi == "" {
		kimi = filepath.Join(home, ".kimi-code")
	}
	shared := filepath.Join(home, ".agents", "skills")
	return []skillHome{
		{"Claude Code", filepath.Join(home, ".claude"), filepath.Join(home, ".claude", "skills")},
		{"Kimi Code", kimi, filepath.Join(kimi, "skills")},
		{"shared agents folder (Codex and others)", "", shared},
	}
}

// installSkill writes the embedded skill as ntrecord/SKILL.md under the shared
// agents folder and the folder of each agent found on this machine, or under
// the one named. The paths written go to stdout, one to a line; who each is
// for goes to stderr. A skill already there is updated: it is meant to match
// the binary.
func installSkill(where string, stdout, stderr io.Writer) error {
	var homes []skillHome
	if where == "auto" {
		for _, h := range skillHomes() {
			if h.detect == "" {
				homes = append(homes, h)
			} else if _, err := os.Stat(h.detect); err == nil {
				homes = append(homes, h)
			}
		}
		if len(homes) == 0 {
			return errors.New("no home folder to install under; name one: ntrecord skill install $HOME/.agents/skills")
		}
	} else {
		homes = []skillHome{{root: expandHome(where)}}
	}
	for _, h := range homes {
		dir := filepath.Join(h.root, "ntrecord")
		if err := clearDanglingLink(dir, stderr); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		path := filepath.Join(dir, "SKILL.md")
		verb := "installed"
		if _, err := os.Stat(path); err == nil {
			verb = "updated"
		}
		if err := writeFileAtomic(path, []byte(skills.NTRecord), 0o644); err != nil {
			return err
		}
		if h.agent != "" {
			fmt.Fprintf(stderr, "ntrecord: skill %s for %s: %s\n", verb, h.agent, path)
		} else {
			fmt.Fprintf(stderr, "ntrecord: skill %s: %s\n", verb, path)
		}
		fmt.Fprintln(stdout, path)
	}
	return nil
}

// clearDanglingLink removes dir when it is a symbolic link to nothing, such as
// one left by a moved checkout, so a real folder can take its place. A link to
// a folder that exists was made on purpose: it is neither replaced nor written
// through, and the error says where it points.
func clearDanglingLink(dir string, stderr io.Writer) error {
	info, err := os.Lstat(dir)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	target, err := os.Readlink(dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s is a link to %s; remove it to install the skill from this binary, or keep it to use that copy", dir, target)
	}
	if err := os.Remove(dir); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "ntrecord: replaced a broken link to %s: %s\n", target, dir)
	return nil
}

// writeFileAtomic writes the file whole or not at all: to a temporary file in
// the same folder, then renamed over the name. A name that is a symbolic link
// is replaced, and what it pointed at is left alone.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ntrecord-skill-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // Gone once renamed; otherwise, not left behind.
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := errors.Join(tmp.Chmod(mode), tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// expandHome resolves a leading ~/ , which the shell leaves alone after =.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
