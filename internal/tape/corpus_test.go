package tape

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The corpus is every .tape in Charm's VHS at the commit in testdata/vhs/MANIFEST:
// 109 tapes, from one-line settings to the Bubble Tea gallery. A VHS tape should
// be a sigmamax tape, so each must parse. Where sigmamax intentionally differs,
// knownDifferences says so, with the reason; the test fails if that list and
// reality drift apart in either direction.
const corpusDir = "testdata/vhs"

// rejectedByVHSToo are tapes upstream ships to demonstrate its own parse errors:
// rejecting them is correct, so they must not parse.
var rejectedByVHSToo = map[string]string{
	"examples/errors/parser.tape": "VHS's demonstration of parse errors (unknown setting and commands)",
}

// knownDifferences maps a corpus tape that VHS accepts to why sigmamax rejects
// it. Keep it empty when it can be.
var knownDifferences = map[string]string{}

// rejected is both lists: tapes that must not parse, with why.
func rejected() map[string]string {
	out := map[string]string{}
	for _, m := range []map[string]string{rejectedByVHSToo, knownDifferences} {
		for rel, reason := range m {
			out[rel] = reason
		}
	}
	return out
}

// manifest reads MANIFEST: "<git blob sha1> <path>" per line, # for comments.
func manifest(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join(corpusDir, "MANIFEST"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sum, path, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("bad MANIFEST line %q", line)
		}
		out[path] = sum
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// gitBlobSHA is the SHA-1 git computes for a file's content.
func gitBlobSHA(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestCorpusIsUnmodifiedUpstream(t *testing.T) {
	want := manifest(t)
	if len(want) < 100 {
		t.Fatalf("MANIFEST lists %d tapes; the corpus should have all of upstream's", len(want))
	}
	have := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(corpusDir, "examples"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".tape") {
			return err
		}
		rel, _ := filepath.Rel(corpusDir, path)
		rel = filepath.ToSlash(rel)
		have[rel] = true
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch sum, listed := want[rel]; {
		case !listed:
			t.Errorf("%s is not in MANIFEST: the corpus holds upstream files only", rel)
		case gitBlobSHA(content) != sum:
			t.Errorf("%s was modified; fixtures are copied unchanged from upstream", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for rel := range want {
		if !have[rel] {
			t.Errorf("%s is in MANIFEST but missing", rel)
		}
	}
}

func TestEveryVHSTapeParses(t *testing.T) {
	var paths []string
	for rel := range manifest(t) {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for rel, reason := range rejected() {
		if reason == "" {
			t.Errorf("%s: a rejected tape needs a reason", rel)
		}
	}
	for _, rel := range paths {
		t.Run(rel, func(t *testing.T) {
			tape, err := ParseFile(filepath.Join(corpusDir, filepath.FromSlash(rel)))
			reason, expected := rejected()[rel]
			switch {
			case err != nil && !expected:
				t.Fatalf("a VHS tape does not parse: %v", err)
			case err == nil && expected:
				t.Fatalf("listed as rejected (%s) but it parses: update the lists in corpus_test.go", reason)
			case err == nil && len(tape.Actions) == 0 && !strings.Contains(rel, "settings/"):
				t.Fatalf("parsed to no actions")
			}
		})
	}
	for rel := range rejected() {
		if _, ok := manifest(t)[rel]; !ok {
			t.Errorf("%s is listed as rejected but is not in the corpus", rel)
		}
	}
}
