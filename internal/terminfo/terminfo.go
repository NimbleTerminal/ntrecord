// Package terminfo carries the compiled entry behind sigmamax's declared
// TERM, xterm-ghostty. Programs decide whether a terminal draws images by its
// name, not by probing, so a recorded program that sees xterm-256color assumes
// no Kitty graphics. Declaring Ghostty's name without Ghostty's entry would break
// every terminfo reader on a host that never installed it, so the entry travels
// inside the binary and Install puts it where ncurses looks.
//
// The entry is Ghostty's own, from the revision libghostty-vt is built from, so
// it describes what the embedded engine implements. Regenerate both files after
// a pin bump with scripts/terminfo.sh (task terminfo); never edit them by hand.
package terminfo

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// Term is the TERM value sigmamax declares.
const Term = "xterm-ghostty"

// entry is Term compiled by tic -x from xterm-ghostty.terminfo, kept beside it
// for review. The compiled format is the portable one every ncurses reads.
//
//go:embed xterm-ghostty
var entry []byte

// layouts are where ncurses looks: a directory named for the first letter
// (Linux) or its hex code (macOS). "ghostty" is an alias in the entry's names.
var layouts = []string{"x/xterm-ghostty", "78/xterm-ghostty", "g/ghostty", "67/ghostty"}

// Install writes the entry under dir, which a child process then finds through
// TERMINFO. Setting TERMINFO does not hide other terminals: ncurses searches it
// first and falls back to its usual list.
func Install(dir string) error {
	for _, rel := range layouts {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("terminfo: %w", err)
		}
		if err := os.WriteFile(path, entry, 0o644); err != nil {
			return fmt.Errorf("terminfo: %w", err)
		}
	}
	return nil
}
