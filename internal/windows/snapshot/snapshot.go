// Package snapshot has what the portable programs kept outside the repo
// (Playnite, RetroArch) share: a folder copied with robocopy to a "store" (an
// external drive, a NAS, a synced folder), the patterns that leave parts of
// it out, and the sizes the app shows before copying.
//
// Each program keeps its own config, manifest and checks; this package only
// knows about folders and files.
package snapshot

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Clean folds a Windows path for comparison: backslashes, no trailing
// separator, lower case.
func Clean(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "/", `\`)
	return strings.ToLower(strings.TrimRight(p, `\`))
}

// Inside tells whether p is dir or a folder under it.
func Inside(p, dir string) bool {
	return strings.HasPrefix(Clean(p)+`\`, Clean(dir)+`\`)
}

// Overlap tells whether one of the folders is inside the other (or both are
// the same): copying one onto the other would copy a folder into itself.
func Overlap(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	return Inside(a, b) || Inside(b, a)
}

// IsFilePattern tells an exclusion that names files (it has *, ? or a dot)
// from one that names a folder relative to the copied folder.
func IsFilePattern(p string) bool { return strings.ContainsAny(p, "*?.") }

// Excluded tells whether rel (relative to the copied folder, with
// backslashes) is left out by the patterns.
func Excluded(patterns []string, rel string, dir bool) bool {
	name := rel[strings.LastIndex(rel, `\`)+1:]
	for _, p := range patterns {
		if IsFilePattern(p) {
			if !dir {
				if ok, _ := filepath.Match(strings.ToLower(p), strings.ToLower(name)); ok {
					return true
				}
			}
			continue
		}
		if dir && strings.EqualFold(strings.Trim(strings.ReplaceAll(p, "/", `\`), `\`), rel) {
			return true
		}
	}
	return false
}

// Folder is a top-level entry of a copied folder with its size.
type Folder struct {
	Name     string `json:"name"`
	Bytes    int64  `json:"bytes"`
	Files    int    `json:"files"`
	Excluded bool   `json:"excluded"`
}

// LooseFiles names the files right in the copied folder in Measure.
const LooseFiles = "(arquivos do programa)"

// Measure sizes every top-level entry of dir, largest first, and the total a
// copy takes (exclusions left out). The files right in dir are one entry.
func Measure(dir string, exclude []string) ([]Folder, int64, int) {
	out := []Folder{}
	var total int64
	var files int
	loose := Folder{Name: LooseFiles}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			if info, err := e.Info(); err == nil && !Excluded(exclude, e.Name(), false) {
				loose.Bytes += info.Size()
				loose.Files++
			}
			continue
		}
		f := Folder{Name: e.Name(), Excluded: Excluded(exclude, e.Name(), true)}
		base := filepath.Join(dir, e.Name())
		filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			rel = strings.ReplaceAll(rel, "/", `\`)
			if d.IsDir() {
				if p != base && Excluded(exclude, rel, true) {
					return filepath.SkipDir
				}
				return nil
			}
			if Excluded(exclude, rel, false) {
				return nil
			}
			if info, err := d.Info(); err == nil {
				f.Bytes += info.Size()
				f.Files++
			}
			return nil
		})
		if !f.Excluded {
			total += f.Bytes
			files += f.Files
		}
		out = append(out, f)
	}
	if loose.Files > 0 {
		total += loose.Bytes
		files += loose.Files
		out = append(out, loose)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out, total, files
}

// RobocopyArgs are the arguments to copy src to dst. mirror deletes what is
// in dst and not in src; the excluded items are never deleted from dst.
func RobocopyArgs(src, dst string, exclude []string, mirror bool) []string {
	args := []string{src, dst}
	if mirror {
		args = append(args, "/MIR")
	} else {
		args = append(args, "/E")
	}
	args = append(args, "/COPY:DAT", "/DCOPY:DAT", "/R:1", "/W:1", "/MT:16", "/NP", "/NFL", "/NDL", "/NJH", "/XJ")
	var dirs, files []string
	for _, x := range exclude {
		if IsFilePattern(x) {
			files = append(files, x)
		} else {
			dirs = append(dirs, filepath.Join(src, strings.ReplaceAll(x, "/", `\`)))
		}
	}
	if len(dirs) > 0 {
		args = append(append(args, "/XD"), dirs...)
	}
	if len(files) > 0 {
		args = append(append(args, "/XF"), files...)
	}
	return args
}

// ReadJSON reads a JSON file into v; found is false when it does not exist.
func ReadJSON(path string, v any) (found bool, err error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, v)
}

// WriteJSON writes v as indented JSON.
func WriteJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// FindStore returns store, or the same path on another drive when store is
// not there and that drive has the manifest: an external drive gets another
// letter on another machine.
func FindStore(store, manifest string) string {
	if store == "" || len(store) < 3 || store[1] != ':' {
		return store
	}
	if _, err := os.Stat(store); err == nil {
		return store
	}
	for c := 'C'; c <= 'Z'; c++ {
		alt := string(c) + store[1:]
		if strings.EqualFold(alt, store) {
			continue
		}
		if _, err := os.Stat(filepath.Join(alt, manifest)); err == nil {
			return alt
		}
	}
	return store
}
