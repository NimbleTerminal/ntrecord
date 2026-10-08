package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	ft "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/harfbuzz"
	"golang.org/x/image/font/sfnt"
)

// These are pinned, redistributable fonts; see fonts/README.md and licenses.
//
//go:embed fonts/NotoSansSymbols2-Regular.ttf
var symbolsFont []byte

//go:embed fonts/NotoColorEmoji.ttf
var emojiFont []byte

type fallbackFont struct {
	shaper  *harfbuzz.Font
	face    *ft.Face
	outline *face
	name    string
	emoji   bool
}

func loadFallback(data []byte, index int, size float64, name string) *fallbackFont {
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || index >= len(loaders) {
		return nil
	}
	parsed, err := ft.NewFont(loaders[index])
	if err != nil {
		return nil
	}
	f := &fallbackFont{face: ft.NewFace(parsed), name: name}
	f.face.SetPpem(uint16(size+0.5), uint16(size+0.5))
	if collection, err := sfnt.ParseCollection(data); err == nil {
		if outline, err := collection.Font(index); err == nil {
			f.outline = newFace(outline, size)
		}
	}
	return f
}
func ignorable(r rune) bool {
	return r == 0x200c || r == 0x200d || r >= 0xfe00 && r <= 0xfe0f || r >= 0xe0020 && r <= 0xe007f || r >= 0xe0100 && r <= 0xe01ef
}
func covers(c ft.Cmap, text []rune) bool {
	for _, r := range text {
		if ignorable(r) || unicode.IsSpace(r) {
			continue
		}
		if gid, ok := c.Lookup(r); !ok || gid == 0 {
			return false
		}
	}
	return true
}

// Store compact coverage only; parse a full face when a missing cluster needs
// it. Limits prevent a large system font directory from growing without bound.
type fontCandidate struct {
	path   string
	index  int
	cmap   ft.Cmap
	aspect ft.Aspect
}
type fallbackCatalog struct {
	scanned bool
	entries []fontCandidate
	loaded  map[string]*fallbackFont
	bytes   int64
}

func fontRoots() []string {
	if paths, ok := os.LookupEnv("NTRECORD_FONT_PATH"); ok {
		if paths == "" {
			return nil
		}
		return filepath.SplitList(paths)
	}
	home, _ := os.UserHomeDir()
	roots := []string{"/Library/Fonts", "/System/Library/Fonts", filepath.Join(home, "Library/Fonts"), "/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".fonts"), filepath.Join(home, ".local/share/fonts")}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		roots = append(roots, filepath.Join(xdg, "fonts"))
	}
	return roots
}
func (c *fallbackCatalog) scan() {
	c.scanned = true
	c.loaded = make(map[string]*fallbackFont)
	var paths []string
	for _, root := range fontRoots() {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if len(paths) >= 1024 {
				return fs.SkipAll
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".ttf" || ext == ".otf" || ext == ".ttc" {
				paths = append(paths, path)
			}
			return nil
		})
	}
	sort.Strings(paths)
	var coverageBytes int
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		info, err := file.Stat()
		if err != nil || info.Size() > 64<<20 {
			file.Close()
			continue
		}
		loaders, err := ot.NewLoaders(file)
		if err != nil {
			file.Close()
			continue
		}
		for i, ld := range loaders {
			if len(c.entries) >= 2048 || coverageBytes >= 32<<20 {
				break
			}
			raw, err := ld.RawTable(ot.MustNewTag("cmap"))
			if err != nil {
				continue
			}
			if len(raw) > (32<<20)-coverageBytes {
				continue
			}
			coverageBytes += len(raw)
			cm, _, err := tables.ParseCmap(raw)
			if err != nil {
				continue
			}
			cmap, _, err := ft.ProcessCmap(cm, tables.FPNone)
			if err != nil {
				continue
			}
			metadata, _ := ft.Describe(ld, nil)
			c.entries = append(c.entries, fontCandidate{path, i, cmap, metadata.Aspect})
		}
		file.Close()
	}
}
func (c *fallbackCatalog) resolve(text []rune, size float64, bold, italic bool) *fallbackFont {
	if !c.scanned {
		c.scan()
	}
	type match struct {
		index int
		score float64
	}
	var matches []match
	for i, f := range c.entries {
		if !covers(f.cmap, text) {
			continue
		}
		weight := float64(400)
		if bold {
			weight = 700
		}
		score := float64(f.aspect.Weight) - weight
		if score < 0 {
			score = -score
		}
		if (f.aspect.Style == ft.StyleItalic) != italic {
			score += 1000
		}
		matches = append(matches, match{i, score})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score < matches[j].score })
	for _, m := range matches {
		candidate := c.entries[m.index]
		key := candidate.path + string(rune(candidate.index))
		if f, ok := c.loaded[key]; ok {
			if f != nil {
				return f
			}
			continue
		}
		// Bound full font parsing, including failed candidates. Source bytes are
		// not a total heap estimate: decoded tables and masks also use memory.
		if len(c.loaded) >= 16 {
			return nil
		}
		c.loaded[key] = nil
		data, err := readFont(candidate.path, min(64<<20, (128<<20)-c.bytes))
		if err != nil {
			continue
		}
		c.bytes += int64(len(data))
		f := loadFallback(data, candidate.index, size, candidate.path)
		if f == nil || f.outline == nil {
			continue
		}
		c.loaded[key] = f
		return f
	}
	return nil
}

func readFont(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("font exceeds %d byte limit or is not a regular file: %s", limit, path)
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("font exceeds %d byte limit: %s", limit, path)
	}
	return data, err
}
