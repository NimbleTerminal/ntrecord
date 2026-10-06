package render

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/sfnt"
)

func fontFaces(family string) ([4]*sfnt.Font, error) {
	var faces [4]*sfnt.Font
	if family == "" || strings.EqualFold(family, "Go Mono") || strings.EqualFold(family, "monospace") {
		for i, b := range [][]byte{gomono.TTF, gomonobold.TTF, gomonoitalic.TTF, gomonobolditalic.TTF} {
			f, e := sfnt.Parse(b)
			if e != nil {
				return faces, e
			}
			faces[i] = f
		}
		return faces, nil
	}
	if strings.ContainsAny(family, "/\\") || strings.HasSuffix(strings.ToLower(family), ".ttf") || strings.HasSuffix(strings.ToLower(family), ".otf") {
		b, e := os.ReadFile(family)
		if e != nil {
			return faces, e
		}
		f, e := sfnt.Parse(b)
		if e != nil {
			return faces, e
		}
		for i := range faces {
			faces[i] = f
		}
		return faces, nil
	}
	home, _ := os.UserHomeDir()
	roots := []string{"/Library/Fonts", "/System/Library/Fonts", filepath.Join(home, "Library/Fonts"), "/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".fonts"), filepath.Join(home, ".local/share/fonts")}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		roots = append(roots, filepath.Join(xdg, "fonts"))
	}
	normalize := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, " ", "")) }
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
				return nil
			}
			stat, e := d.Info()
			if e != nil || stat.Size() > 32*1024*1024 {
				return nil
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return nil
			}
			collection, e := sfnt.ParseCollection(b)
			if e != nil {
				return nil
			}
			for n := 0; n < collection.NumFonts(); n++ {
				f, e := collection.Font(n)
				if e != nil {
					continue
				}
				fn, _ := f.Name(nil, sfnt.NameIDFamily)
				full, _ := f.Name(nil, sfnt.NameIDFull)
				if normalize(fn) != normalize(family) && normalize(full) != normalize(family) {
					continue
				}
				style, _ := f.Name(nil, sfnt.NameIDSubfamily)
				style = strings.ToLower(style)
				i := 0
				if strings.Contains(style, "bold") {
					i++
				}
				if strings.Contains(style, "italic") || strings.Contains(style, "oblique") {
					i += 2
				}
				if faces[i] == nil {
					faces[i] = f
				}
			}
			return nil
		})
	}
	if faces[0] == nil {
		for _, f := range faces {
			if f != nil {
				faces[0] = f
				break
			}
		}
	}
	if faces[0] == nil {
		return faces, fmt.Errorf("font family %q not found; install it, use a .ttf/.otf path, or Set FontFamily \"Go Mono\"", family)
	}
	for i := range faces {
		if faces[i] == nil {
			faces[i] = faces[0]
		}
	}
	return faces, nil
}
