package retroarch

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// playlist is the part of a .lpl file that is read.
type playlist struct {
	DefaultCore string `json:"default_core_path"`
	Items       []struct {
		Path     string `json:"path"`
		CorePath string `json:"core_path"`
	} `json:"items"`
}

func readPlaylist(path string) (playlist, bool) {
	var p playlist
	raw, err := os.ReadFile(path)
	if err != nil {
		return p, false
	}
	return p, json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}), &p) == nil
}

// playlistFiles lists the .lpl files of the playlists folder, the built-in
// ones (history, favorites) included.
func playlistFiles(dir string) []string {
	var out []string
	filepath.WalkDir(filepath.Join(dir, "playlists"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".lpl") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// gameFile is the file of a playlist path: a game inside an archive is
// written "archive.zip#game.sfc".
func gameFile(p string) string {
	if i := strings.Index(p, "#"); i > 0 {
		return p[:i]
	}
	return p
}

// gameIndex finds a game by its file name in the games folder.
type gameIndex map[string][]string

func indexGames(dir string) gameIndex {
	idx := gameIndex{}
	if dir == "" {
		return idx
	}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			k := strings.ToLower(d.Name())
			idx[k] = append(idx[k], p)
		}
		return nil
	})
	return idx
}

// find is where the game of a playlist path is now: the file with the same
// name, when there is exactly one; "" otherwise. The archive member, if any,
// is kept.
func (g gameIndex) find(p string) string {
	file := gameFile(p)
	// the playlists are written on Windows: the separator is a backslash
	name := file[strings.LastIndexAny(file, `\/`)+1:]
	hits := g[strings.ToLower(name)]
	if len(hits) != 1 {
		return ""
	}
	return hits[0] + p[len(file):]
}

// jsonString is s as RetroArch writes it in a playlist: only backslashes and
// quotes are escaped.
func jsonString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// PlaylistFix is what FixPlaylists changed in one playlist.
type PlaylistFix struct {
	Name  string `json:"name"`
	Games int    `json:"games"`
	Cores int    `json:"cores"`
	// Missing are the games still not found.
	Missing []string `json:"missing"`
}

// FixPlaylists points the playlists at the games and cores where they are
// now: a game whose path does not exist is looked up by file name in the
// games folder, and a core by file name in the cores folder. Only paths that
// do not exist are touched, and only when the file is found exactly once; the
// rest of each file is kept byte for byte. It returns the playlists changed
// or still missing something.
func FixPlaylists(c Config) ([]PlaylistFix, error) {
	idx := indexGames(c.Games)
	coreDir := filepath.Join(c.Path, "cores")
	out := []PlaylistFix{}
	for _, file := range playlistFiles(c.Path) {
		pl, ok := readPlaylist(file)
		if !ok {
			continue
		}
		rel, _ := filepath.Rel(filepath.Join(c.Path, "playlists"), file)
		fix := PlaylistFix{Name: strings.TrimSuffix(filepath.ToSlash(rel), ".lpl"), Missing: []string{}}
		repl := map[string]string{}
		core := func(p string) {
			if p == "" || strings.EqualFold(p, "DETECT") || exists(p) {
				return
			}
			if _, done := repl[p]; done {
				return
			}
			now := filepath.Join(coreDir, p[strings.LastIndexAny(p, `\/`)+1:])
			if exists(now) {
				repl[p] = now
				fix.Cores++
			}
		}
		core(pl.DefaultCore)
		for _, it := range pl.Items {
			core(it.CorePath)
			if it.Path == "" || exists(gameFile(it.Path)) {
				continue
			}
			if _, done := repl[it.Path]; done {
				continue
			}
			if now := idx.find(it.Path); now != "" {
				repl[it.Path] = now
				fix.Games++
			} else {
				fix.Missing = append(fix.Missing, it.Path)
			}
		}
		if len(repl) > 0 {
			raw, err := os.ReadFile(file)
			if err != nil {
				return out, err
			}
			for from, to := range repl {
				raw = bytes.ReplaceAll(raw, []byte(jsonString(from)), []byte(jsonString(to)))
			}
			if err := os.WriteFile(file, raw, 0o644); err != nil {
				return out, err
			}
		}
		if len(repl) > 0 || len(fix.Missing) > 0 {
			out = append(out, fix)
		}
	}
	return out, nil
}
