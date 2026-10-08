// Package boundaries holds the test that keeps ntrecord's package structure
// honest. Each package lists what it may import; anything else fails the build.
// The point is to keep three seams narrow: libghostty (and so cgo) is reached
// only through internal/render, external programs are started only by the
// packages that own them, and the leaf packages stay free of the rest.
package boundaries

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const module = "nimbleterminal.dev/ntrecord"

// rule is what one package may import beyond the standard library.
type rule struct {
	internal []string // packages of this module, relative to its root
	external []string // third-party import path prefixes
	exec     bool     // may import os/exec
}

// rules is the allow-list. A new package must be added here, which is the
// moment to decide what it may depend on.
var rules = map[string]rule{
	".":                  {internal: []string{"internal/doctor", "internal/encode", "internal/player", "internal/record", "internal/tape"}, external: []string{"golang.org/x/term"}},
	"internal/doctor":    {internal: []string{"internal/encode", "internal/render", "internal/tape", "internal/terminfo"}, external: []string{"github.com/creack/pty"}, exec: true},
	"internal/encode":    {internal: []string{"internal/gifencode"}, exec: true},
	"internal/gifencode": {},
	"internal/playback":  {internal: []string{"internal/asciicast", "internal/render", "internal/tape"}},
	"internal/player":    {internal: []string{"internal/playback", "internal/tape"}, external: []string{"golang.org/x/term", "golang.org/x/sys/unix"}},
	"internal/asciicast": {}, // standard-library event codec, independent of rendering
	"internal/record":    {internal: []string{"internal/asciicast", "internal/encode", "internal/render", "internal/tape", "internal/terminfo"}, external: []string{"github.com/creack/pty"}, exec: true},
	// The only package that may import libghostty, so cgo stays behind one door.
	"internal/render":    {internal: []string{"internal/tape", "internal/theme"}, external: []string{"go.mitchellh.com/libghostty", "golang.org/x/image", "github.com/go-text/typesetting", "golang.org/x/text/unicode/norm"}},
	"internal/tape":      {internal: []string{"internal/encode", "internal/theme"}},
	"internal/terminfo":  {},
	"internal/testvideo": {exec: true}, // test helper that drives ffmpeg/ffprobe
	"internal/theme":     {},
}

// exempt packages are tools and demos, not part of the recorder.
func exempt(pkg string) bool {
	return strings.HasPrefix(pkg, "internal/tools/") || strings.HasPrefix(pkg, "examples/")
}

// violations lists what pkg may not import, given everything it does.
func violations(pkg string, imports []string) []string {
	r, known := rules[pkg]
	if !known {
		return []string{pkg + ": no boundary rule; add one to rules in internal/boundaries"}
	}
	var out []string
	for _, imp := range imports {
		switch {
		case imp == "C":
			out = append(out, pkg+`: imports "C"; cgo belongs to the libghostty binding`)
		case imp == "os/exec":
			if !r.exec {
				out = append(out, pkg+": starts programs (os/exec); only packages that own an external program may")
			}
		case strings.HasPrefix(imp, module+"/"):
			if rel := strings.TrimPrefix(imp, module+"/"); !slices.Contains(r.internal, rel) {
				out = append(out, pkg+": imports "+rel)
			}
		case strings.Contains(strings.SplitN(imp, "/", 2)[0], "."): // third party
			if !slices.ContainsFunc(r.external, func(p string) bool { return imp == p || strings.HasPrefix(imp, p+"/") }) {
				out = append(out, pkg+": imports "+imp)
			}
		}
	}
	return out
}

func TestViolationsBite(t *testing.T) {
	cases := []struct {
		pkg     string
		imports []string
		want    string // substring of the one expected violation; "" for none
	}{
		{"internal/render", []string{"fmt", "go.mitchellh.com/libghostty", "go.mitchellh.com/libghostty/sys/png", module + "/internal/theme"}, ""},
		{"internal/record", []string{"go.mitchellh.com/libghostty"}, "go.mitchellh.com/libghostty"},
		{"internal/tape", []string{module + "/internal/render"}, "imports internal/render"},
		{"internal/gifencode", []string{"os/exec"}, "os/exec"},
		{"internal/theme", []string{"golang.org/x/image/draw"}, "golang.org/x/image/draw"},
		{"internal/encode", []string{"os/exec", module + "/internal/gifencode"}, ""},
		{"internal/render", []string{"C"}, `"C"`},
		{"internal/brandnew", []string{"fmt"}, "no boundary rule"},
	}
	for _, c := range cases {
		got := violations(c.pkg, c.imports)
		switch {
		case c.want == "" && len(got) != 0:
			t.Errorf("%s %v: unexpected %v", c.pkg, c.imports, got)
		case c.want != "" && (len(got) != 1 || !strings.Contains(got[0], c.want)):
			t.Errorf("%s %v: want one violation containing %q, got %v", c.pkg, c.imports, c.want, got)
		}
	}
}

func TestRepositoryRespectsBoundaries(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string][]string{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "bin" || name == "docs" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		pkg := filepath.ToSlash(rel)
		found[pkg] = append(found[pkg], "") // the package exists even without imports
		for _, spec := range file.Imports {
			imp, _ := strconv.Unquote(spec.Path.Value)
			found[pkg] = append(found[pkg], imp)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := found["internal/render"]; !ok {
		t.Fatalf("walked the wrong tree (%s): internal/render not found", root)
	}
	for pkg, imports := range found {
		if exempt(pkg) {
			continue
		}
		for _, v := range violations(pkg, imports) {
			t.Error(v)
		}
	}
	// A rule for a package that no longer exists is stale.
	for pkg := range rules {
		if _, ok := found[pkg]; !ok {
			if _, err := os.Stat(filepath.Join(root, pkg)); err != nil {
				t.Errorf("rule for missing package %s", pkg)
			}
		}
	}
}
