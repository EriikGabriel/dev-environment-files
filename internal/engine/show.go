package engine

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxTextBytes  = 256 << 10
	maxImageBytes = 5 << 20
)

// Entry is one item of a directory listing.
type Entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

// FileView is the content of a repo file or directory, for display.
type FileView struct {
	Path string `json:"path"` // relative to the repo root
	// Kind is text, dir, image or binary.
	Kind      string  `json:"kind"`
	Size      int64   `json:"size"`
	Content   string  `json:"content,omitempty"`  // text
	Truncated bool    `json:"truncated"`          // text cut at the size limit
	DataURL   string  `json:"data_url,omitempty"` // image
	Entries   []Entry `json:"entries"`            // dir
}

var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml",
}

// View reads a file or directory that lives inside the repo. Paths that
// resolve outside the root (.., absolute paths, symlinks) are refused.
func View(root, rel string) (FileView, error) {
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return FileView{}, err
	}
	target, err := filepath.EvalSymlinks(filepath.Join(rootReal, filepath.FromSlash(rel)))
	if err != nil {
		return FileView{}, err
	}
	if target != rootReal && !strings.HasPrefix(target, rootReal+string(filepath.Separator)) {
		return FileView{}, fmt.Errorf("%s is outside the repository", rel)
	}
	relClean, _ := filepath.Rel(rootReal, target)
	v := FileView{Path: filepath.ToSlash(relClean), Entries: []Entry{}}

	fi, err := os.Stat(target)
	if err != nil {
		return FileView{}, err
	}
	v.Size = fi.Size()

	if fi.IsDir() {
		v.Kind = "dir"
		list, err := os.ReadDir(target)
		if err != nil {
			return FileView{}, err
		}
		for _, e := range list {
			if e.Name() == ".git" {
				continue
			}
			en := Entry{Name: e.Name(), Dir: e.IsDir()}
			if info, err := e.Info(); err == nil && !e.IsDir() {
				en.Size = info.Size()
			}
			v.Entries = append(v.Entries, en)
		}
		sort.Slice(v.Entries, func(i, j int) bool {
			if v.Entries[i].Dir != v.Entries[j].Dir {
				return v.Entries[i].Dir
			}
			return v.Entries[i].Name < v.Entries[j].Name
		})
		return v, nil
	}

	if mime, ok := imageTypes[strings.ToLower(filepath.Ext(target))]; ok {
		if fi.Size() > maxImageBytes {
			v.Kind = "binary"
			return v, nil
		}
		b, err := os.ReadFile(target)
		if err != nil {
			return FileView{}, err
		}
		v.Kind = "image"
		v.DataURL = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
		return v, nil
	}

	f, err := os.Open(target)
	if err != nil {
		return FileView{}, err
	}
	defer f.Close()
	buf := make([]byte, maxTextBytes+1)
	n, _ := f.Read(buf)
	if n > maxTextBytes {
		n, v.Truncated = maxTextBytes, true
	}
	buf = buf[:n]
	if strings.IndexByte(string(buf), 0) >= 0 {
		v.Kind = "binary"
		return v, nil
	}
	// a cut in the middle of a multi-byte rune is not a reason to call it binary
	for len(buf) > 0 && !utf8.Valid(buf) && v.Truncated && n-len(buf) < 4 {
		buf = buf[:len(buf)-1]
	}
	if !utf8.Valid(buf) {
		v.Kind = "binary"
		return v, nil
	}
	v.Kind, v.Content = "text", string(buf)
	return v, nil
}
