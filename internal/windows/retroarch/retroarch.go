// Package retroarch keeps a portable RetroArch and the folder of games next
// to it, split by where each part can live:
//
//   - git (windows/retroarch): the personal part, a few megabytes: settings,
//     core options, playlists, saves, states, thumbnails. Passwords and tokens
//     are blanked and kept in the store (see snapshot.Export).
//   - download: the program, its assets and the cores, from the official
//     libretro buildbot, at the version recorded in config/retroarch.yaml.
//   - the store (an external or second drive): what is neither, the games and
//     the BIOS (the system folder), which are copyrighted and too large for
//     GitHub. copy_program also keeps a full copy of the program there.
//
// A portable RetroArch writes its paths relative to its own folder (":\cores"
// in retroarch.cfg). The playlists keep full paths to the games and cores;
// after a restore, or when the folders move, FixPlaylists points them at the
// files again.
package retroarch

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// Config is config/retroarch.yaml.
type Config struct {
	// Path is the RetroArch folder on this machine.
	Path string `yaml:"path" json:"path"`
	// Games is the folder of the games (one subfolder per system); empty
	// leaves the games out of the snapshot.
	Games string `yaml:"games" json:"games"`
	// Store holds the snapshot: Store\RetroArch, Store\Games and the
	// manifest Store\envsetup-retroarch.json.
	Store string `yaml:"store" json:"store"`
	// Exclude are left out of the copy of Path: a name with *, ? or a dot is
	// a file pattern (anywhere), anything else a folder relative to Path.
	Exclude []string `yaml:"exclude" json:"exclude"`
	// StartMenu registers the portable RetroArch as an app of the user
	// (Start menu shortcut and an entry in Installed apps) after a restore.
	StartMenu bool `yaml:"start_menu" json:"start_menu"`
	// Version is the RetroArch release a restore downloads; Cores the cores
	// (file names without _libretro.dll). Both are written by a save.
	Version string   `yaml:"version" json:"version"`
	Cores   []string `yaml:"cores" json:"cores"`
	// Bundles are the libretro asset packs a restore downloads with the
	// program (see BundleDirs).
	Bundles []string `yaml:"bundles" json:"bundles"`
	// Git are the files and folders (relative to Path) kept in the repo, in
	// windows/retroarch.
	Git []string `yaml:"git" json:"git"`
	// CopyProgram also keeps the whole program in the store, so a restore
	// does not download it.
	CopyProgram bool `yaml:"copy_program" json:"copy_program"`
	// CopyGames copies the games folder to the store on each save. Off when
	// the games already live on the drive that is kept (D:\Games\Retro).
	CopyGames bool `yaml:"copy_games" json:"copy_games"`
}

// Default is what a missing file means.
func Default() Config {
	return Config{
		Path:      `C:\Emuladores\RetroArch`,
		Games:     `D:\Games\Retro`,
		Exclude:   []string{"logs", "downloads", "*.log"},
		StartMenu: true,
		Cores:     []string{},
		Bundles:   append([]string(nil), DefaultBundles...),
		Git:       append([]string(nil), DefaultGit...),
	}
}

// DefaultGit is the personal part of a RetroArch folder.
var DefaultGit = []string{"retroarch.cfg", "retroarch-core-options.cfg", "config", "remaps", "playlists", "saves", "states", "thumbnails"}

// DefaultBundles are the asset packs the online updater of RetroArch offers.
var DefaultBundles = []string{"assets", "autoconfig", "cheats", "database-rdb", "info", "overlays", "shaders_slang", "shaders_glsl"}

// Path is where the config lives in the repo.
func Path(root string) string { return filepath.Join(root, "config", "retroarch.yaml") }

const header = `# The portable RetroArch and its games, split by where each part can live:
#   git       the personal part (settings, saves, playlists...) in windows/retroarch,
#             with passwords and tokens left out (they stay in the store)
#   download  the program, its assets and the cores, from buildbot.libretro.com
#   store     the games and the BIOS (system folder): copyrighted and too
#             large for GitHub, they never go to git
# Edited in the app's RetroArch tab or with ` + "`envsetup set-retroarch`" + `;
# ` + "`retroarch-save`" + ` updates git and the store, ` + "`retroarch-restore`" + ` puts everything back.
#
# path          the RetroArch folder on this machine (the one with retroarch.exe)
# games         the folder of the games, one subfolder per system (on the drive
#               that is not formatted: D:\Games\Retro); empty leaves them out
# store         folder for the rest (second drive, external drive, NAS): it gets
#               <store>\RetroArch (BIOS and what is not downloaded nor in git),
#               <store>\Games, a manifest and the secrets left out of git
# exclude       never copied: names with * or a dot are file patterns, the
#               rest are folders relative to path
# start_menu    after a restore, add RetroArch to the Start menu and to
#               Settings > Apps (windows/register-portable-app.ps1)
# version       the RetroArch release a restore downloads (written by a save)
# cores         the cores a restore downloads (written by a save)
# bundles       the asset packs downloaded with the program
# git           files and folders of path kept in the repo
# copy_program  also keep the whole program in the store (no download)
# copy_games    also copy the games to the store on each save (when they live
#               on the drive that is formatted, or to carry them elsewhere)

`

// Load reads the config; exists is false when the file is missing.
func Load(path string) (Config, bool, error) {
	cfg := Default()
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, err
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, true, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Exclude == nil {
		cfg.Exclude = []string{}
	}
	if cfg.Cores == nil {
		cfg.Cores = []string{}
	}
	return cfg, true, cfg.Validate()
}

// Validate refuses a config that would copy a folder into itself.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Path) == "" {
		return errors.New("retroarch: path is empty")
	}
	for _, x := range append(append([]string{}, c.Exclude...), c.Git...) {
		if strings.Contains(x, "..") {
			return fmt.Errorf("retroarch: %q leaves the folder", x)
		}
	}
	for _, b := range c.Bundles {
		if _, ok := BundleDirs[b]; !ok {
			return fmt.Errorf("retroarch: unknown bundle %q", b)
		}
	}
	if snapshot.Overlap(c.Path, c.Games) {
		return errors.New("retroarch: the games folder and the RetroArch folder must be separate (one is inside the other)")
	}
	if c.Store == "" {
		return nil
	}
	if snapshot.Overlap(c.Store, c.Path) {
		return errors.New("retroarch: the store and the RetroArch folder overlap")
	}
	if snapshot.Overlap(c.Store, c.Games) {
		return errors.New("retroarch: the store and the games folder overlap")
	}
	return nil
}

// Save writes the config with its explanatory header.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString(header)
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// The layout of a store.
const (
	SnapshotDir  = "RetroArch"
	GamesDir     = "Games"
	ManifestFile = "envsetup-retroarch.json"
	// Exe is the program that tells a RetroArch folder apart.
	Exe = "retroarch.exe"
	// Program names the secrets file of the store and the git folder.
	Program = "retroarch"
)

// GitDir is where the personal part lives in the repo.
func GitDir(root string) string { return filepath.Join(root, "windows", "retroarch") }

// ProgramDirs are the folders the program download and the bundles fill:
// a store without copy_program leaves them out.
var ProgramDirs = []string{"assets", "autoconfig", "cheats", "cores", "database", "filters", "info", "overlays", "platforms", "shaders"}

// StoreExclude is what the copy of Path in the store leaves out: the
// exclusions, what git carries whole and, without copy_program, the program
// (its folders and every file right in Path).
func (c Config) StoreExclude() []string {
	out := append([]string{}, c.Exclude...)
	out = append(out, snapshot.TopLevel(c.Git)...)
	if !c.CopyProgram {
		out = append(out, ProgramDirs...)
		out = append(out, rootFiles(c.Path)...)
	}
	return out
}

// RestoreExclude is what a copy from the store back to Path leaves alone:
// the exclusions and what git carries whole, which a mirror must not delete.
func (c Config) RestoreExclude() []string {
	return append(append([]string{}, c.Exclude...), snapshot.TopLevel(c.Git)...)
}

// rootFiles names the files right in dir (the program, without folders).
func rootFiles(dir string) []string {
	var out []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.Contains(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out
}

// PruneStore deletes from the copy in the store what StoreExclude leaves
// out now (robocopy never deletes excluded items): the program after
// copy_program was turned off, and what moved to git. It returns what went.
func PruneStore(c Config) ([]string, error) {
	snap := c.Snapshot()
	if c.Store == "" {
		return nil, nil
	}
	var gone []string
	names := append(append([]string{}, snapshot.TopLevel(c.Git)...), c.Exclude...)
	if !c.CopyProgram {
		names = append(names, ProgramDirs...)
		names = append(names, rootFiles(snap)...)
	}
	for _, n := range names {
		if strings.ContainsAny(n, "*?") {
			continue
		}
		p := filepath.Join(snap, filepath.FromSlash(n))
		if !snapshot.Inside(p, snap) || snapshot.Clean(p) == snapshot.Clean(snap) {
			continue
		}
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return gone, err
		}
		gone = append(gone, n)
	}
	return gone, nil
}

// Snapshot is where the copy of the RetroArch folder lives in the store.
func (c Config) Snapshot() string { return filepath.Join(c.Store, SnapshotDir) }

// GamesSnapshot is where the copy of the games lives in the store.
func (c Config) GamesSnapshot() string { return filepath.Join(c.Store, GamesDir) }

// Manifest describes a snapshot; it is written next to it.
type Manifest struct {
	SavedAt  time.Time `json:"saved_at"`
	Source   string    `json:"source"`
	Games    string    `json:"games"`
	Computer string    `json:"computer"`
	// Bytes and Files are the copy of the RetroArch folder in the store;
	// GameBytes and GameCount the games.
	Bytes     int64    `json:"bytes"`
	Files     int      `json:"files"`
	GameBytes int64    `json:"game_bytes"`
	GameCount int      `json:"game_count"`
	Cores     []string `json:"cores"`
	Systems   []string `json:"systems"`
	// Version is the RetroArch release; Program tells whether the store has
	// the whole program (copy_program).
	Version string `json:"version"`
	Program bool   `json:"program"`
	// Secrets is how many values were left out of git and kept in the store.
	Secrets int `json:"secrets"`
}

// ReadManifest reads the manifest of the store; nil when there is none.
func ReadManifest(store string) (*Manifest, error) {
	var m Manifest
	found, err := snapshot.ReadJSON(filepath.Join(store, ManifestFile), &m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ManifestFile, err)
	}
	if !found {
		return nil, nil
	}
	return &m, nil
}

// WriteManifest saves m in the store.
func WriteManifest(store string, m Manifest) error {
	return snapshot.WriteJSON(filepath.Join(store, ManifestFile), m)
}

// Core is a libretro core in the cores folder.
type Core struct {
	File   string `json:"file"`
	Name   string `json:"name"`
	System string `json:"system"`
}

// System is a subfolder of the games folder.
type System struct {
	Name  string `json:"name"`
	Games int    `json:"games"`
	Bytes int64  `json:"bytes"`
}

// Playlist is a .lpl file: the games RetroArch shows for one system.
type Playlist struct {
	Name  string `json:"name"`
	Items int    `json:"items"`
	// Missing are the items whose game is not where the playlist says.
	Missing int `json:"missing"`
	// Fixable are the missing ones FixPlaylists can find again.
	Fixable int `json:"fixable"`
}

// Setting is one line of retroarch.cfg the app shows.
type Setting struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// Install is what is in a RetroArch folder and the games folder.
type Install struct {
	Found     bool       `json:"found"`
	Cores     []Core     `json:"cores"`
	Systems   []System   `json:"systems"`
	Playlists []Playlist `json:"playlists"`
	Settings  []Setting  `json:"settings"`
	// Saves, States and Bios count the files in those folders.
	Saves  int `json:"saves"`
	States int `json:"states"`
	Bios   int `json:"bios"`
	// Folders, Bytes and Files are the copy of the RetroArch folder (when
	// measured); GameBytes and GameCount the games (count of games, not files).
	Folders   []snapshot.Folder `json:"folders"`
	Bytes     int64             `json:"bytes"`
	Files     int               `json:"files"`
	GameBytes int64             `json:"game_bytes"`
	GameCount int               `json:"game_count"`
}

// shown are the settings of retroarch.cfg the app lists, in order.
var shown = []struct{ key, label string }{
	{"video_driver", "Driver de vídeo"},
	{"video_fullscreen", "Tela cheia"},
	{"video_vsync", "VSync"},
	{"video_shader_enable", "Shaders"},
	{"video_smooth", "Filtro bilinear"},
	{"menu_driver", "Menu"},
	{"input_joypad_driver", "Driver de controle"},
	{"audio_driver", "Driver de áudio"},
	{"savestate_auto_save", "Salvar estado ao sair"},
	{"savestate_auto_load", "Carregar estado ao abrir"},
	{"rewind_enable", "Rebobinar"},
	{"cheevos_enable", "RetroAchievements"},
	{"user_language", "Idioma"},
}

// ReadCfg reads a retroarch.cfg (key = "value" lines) into a map.
func ReadCfg(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	return out, sc.Err()
}

// Read describes the RetroArch folder and the games of c. sizes walks every
// file of both folders.
func Read(c Config, sizes bool) Install {
	in := Install{Cores: []Core{}, Systems: []System{}, Playlists: []Playlist{}, Settings: []Setting{}, Folders: []snapshot.Folder{}}
	if c.Games != "" {
		in.Systems = systems(c.Games)
		for _, s := range in.Systems {
			in.GameBytes += s.Bytes
			in.GameCount += s.Games
		}
	}
	if _, err := os.Stat(filepath.Join(c.Path, Exe)); err != nil {
		return in
	}
	in.Found = true
	in.Cores = cores(c.Path)
	if cfg, err := ReadCfg(filepath.Join(c.Path, "retroarch.cfg")); err == nil {
		for _, s := range shown {
			if v, ok := cfg[s.key]; ok {
				in.Settings = append(in.Settings, Setting{Key: s.key, Label: s.label, Value: v})
			}
		}
	}
	in.Saves = countFiles(filepath.Join(c.Path, "saves"))
	in.States = countFiles(filepath.Join(c.Path, "states"))
	in.Bios = countFiles(filepath.Join(c.Path, "system"))
	idx := indexGames(c.Games)
	for _, p := range playlistFiles(c.Path) {
		pl, ok := readPlaylist(p)
		if !ok {
			continue
		}
		rel, _ := filepath.Rel(filepath.Join(c.Path, "playlists"), p)
		l := Playlist{Name: strings.TrimSuffix(filepath.ToSlash(rel), ".lpl"), Items: len(pl.Items)}
		for _, it := range pl.Items {
			if it.Path == "" || exists(gameFile(it.Path)) {
				continue
			}
			l.Missing++
			if idx.find(it.Path) != "" {
				l.Fixable++
			}
		}
		in.Playlists = append(in.Playlists, l)
	}
	if sizes {
		in.Folders, in.Bytes, in.Files = snapshot.Measure(c.Path, c.StoreExclude())
	}
	return in
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func countFiles(dir string) int {
	n := 0
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// cores lists the cores folder, named from the .info files RetroArch keeps
// in its info folder.
func cores(dir string) []Core {
	out := []Core{}
	files, _ := filepath.Glob(filepath.Join(dir, "cores", "*_libretro.dll"))
	for _, f := range files {
		base := filepath.Base(f)
		c := Core{File: base, Name: strings.TrimSuffix(base, "_libretro.dll")}
		if info, err := ReadCfg(filepath.Join(dir, "info", strings.TrimSuffix(base, ".dll")+".info")); err == nil {
			if v := info["display_name"]; v != "" {
				c.Name = v
			}
			c.System = info["systemname"]
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// systems sizes each subfolder of the games folder.
func systems(dir string) []System {
	out := []System{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s := System{Name: e.Name()}
		filepath.WalkDir(filepath.Join(dir, e.Name()), func(_ string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			// a disc in .bin + .cue is one game: count what a player opens
			switch strings.ToLower(filepath.Ext(d.Name())) {
			case ".bin", ".img", ".sub", ".sav", ".srm", ".txt", ".jpg", ".png":
			default:
				s.Games++
			}
			if info, err := d.Info(); err == nil {
				s.Bytes += info.Size()
			}
			return nil
		})
		out = append(out, s)
	}
	return out
}

// RobocopyArgs are the arguments to copy src to dst (see snapshot.RobocopyArgs).
func RobocopyArgs(src, dst string, exclude []string, mirror bool) []string {
	return snapshot.RobocopyArgs(src, dst, exclude, mirror)
}
