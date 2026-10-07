// Collect dependency license files for binary release archives. The native
// cache also includes build-time dependencies; their notices are retained too.
package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 5 {
		return fmt.Errorf("usage: release-licenses OUTPUT GHOSTTY_SOURCE ZIG_PACKAGE_CACHE ZIG")
	}
	dest, source, cache := os.Args[1], os.Args[2], os.Args[3]
	copyFile := func(name, path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return save(dest, name, b)
	}
	if err := copyFile("ghostty.txt", filepath.Join(source, "LICENSE")); err != nil {
		return err
	}
	goLicense := filepath.Join(runtime.GOROOT(), "LICENSE")
	if _, err := os.Stat(goLicense); os.IsNotExist(err) {
		// Homebrew keeps the license beside libexec rather than in GOROOT.
		goLicense = filepath.Join(runtime.GOROOT(), "..", "LICENSE")
	}
	if err := copyFile("go.txt", goLicense); err != nil {
		return err
	}
	zig, err := filepath.EvalSymlinks(os.Args[4])
	if err != nil {
		return err
	}
	for _, p := range []string{"LICENSE", "lib/libc/musl/COPYRIGHT"} {
		if err := copyFile("zig-"+strings.ReplaceAll(p, "/", "-")+".txt", filepath.Join(filepath.Dir(zig), p)); err != nil {
			return err
		}
	}
	cmd := exec.Command("go", "list", "-mod=readonly", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}} {{.Dir}}{{end}}{{end}}", ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	modules, err := cmd.Output()
	if err != nil {
		return err
	}
	scan := bufio.NewScanner(strings.NewReader(string(modules)))
	seen := make(map[string]bool)
	for scan.Scan() {
		mod, dir, ok := strings.Cut(scan.Text(), " ")
		if !ok || seen[mod] {
			continue
		}
		seen[mod] = true
		if dir == "" {
			return fmt.Errorf("module %s is not downloaded; run go mod download", mod)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		found := false
		for _, entry := range entries {
			if !entry.IsDir() && isLicense(entry.Name()) {
				found = true
				if err := copyFile(strings.ReplaceAll(mod, "/", "_")+"-"+entry.Name()+".txt", filepath.Join(dir, entry.Name())); err != nil {
					return err
				}
			}
		}
		if !found {
			return fmt.Errorf("no license found in %s", mod)
		}
	}
	if err := scan.Err(); err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(cache, "*.tar.gz"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("native package cache is empty")
	}
	for _, path := range paths {
		if err := copyArchiveLicenses(dest, path); err != nil {
			return err
		}
	}
	return nil
}
func isLicense(name string) bool {
	name = strings.ToUpper(name)
	return strings.HasPrefix(name, "LICENSE") || strings.HasPrefix(name, "COPYING") || name == "COPYRIGHT" || strings.HasPrefix(name, "NOTICE")
}
func save(dest, name string, b []byte) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dest, name), b, 0644)
}
func copyArchiveLicenses(dest, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg || !isLicense(filepath.Base(h.Name)) {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		// Flatten cache-relative names; never extract arbitrary archive paths.
		name := "native-" + strings.ReplaceAll(h.Name, "/", "_") + ".txt"
		if err := save(dest, name, b); err != nil {
			return err
		}
	}
}
