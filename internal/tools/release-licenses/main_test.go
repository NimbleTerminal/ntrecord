package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyModuleLicensesIncludesNestedPackages(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "harfbuzz"), 0755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{"LICENSE": "BSD notice", "harfbuzz/LICENSE": "MIT notice", "harfbuzz/source.go": "not a license"} {
		if err := os.WriteFile(filepath.Join(src, path), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyModuleLicenses(dst, "example.org/typesetting", src); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"example.org_typesetting-LICENSE.txt": "BSD notice", "example.org_typesetting-harfbuzz_LICENSE.txt": "MIT notice"} {
		got, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q, %v", name, got, err)
		}
	}
	entries, err := os.ReadDir(dst)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected archive files: %v %v", entries, err)
	}
	if err := copyModuleLicenses(dst, "missing", t.TempDir()); err == nil {
		t.Fatal("accepted module without a license")
	}
}
