// Package playnite keeps a portable Playnite installation (the folder with
// Playnite.DesktopApp.exe, its library, themes, extensions and the media
// they downloaded) as a snapshot outside the repo, and puts it back on
// another machine.
//
// A portable Playnite keeps everything in its own folder, so a copy of it is
// the whole setup: covers, trailers, theme music, the configured add-ons.
// The settings (config.json, fullscreenConfig.json and the settings of each
// extension) go into git, in windows/playnite, with API keys and tokens
// blanked (see snapshot.Export). The rest is tens of gigabytes (the library,
// its covers and the trailers, slow to download again), so it is a mirror in
// a folder chosen in config/playnite.yaml (a second drive, an external drive,
// a NAS), with a manifest and the secrets next to it; the caches Playnite
// rebuilds stay out. What the folder does not carry (Windows Media Player for
// the trailers, the Start menu entry, programs some extensions point to) is
// listed as requirements.
package playnite

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// Config is config/playnite.yaml.
type Config struct {
	// Path is the Playnite folder on this machine.
	Path string `yaml:"path" json:"path"`
	// Store is the folder that holds the snapshot: the copy goes to
	// Store\Playnite and the manifest to Store\envsetup-playnite.json.
	Store string `yaml:"store" json:"store"`
	// Exclude are left out of the snapshot: a name with * or a dot is a file
	// pattern (anywhere), anything else a folder relative to Path.
	Exclude []string `yaml:"exclude" json:"exclude"`
	// StartMenu registers the portable Playnite as an app of the user (Start
	// menu shortcut and an entry in Installed apps) after a restore.
	StartMenu bool `yaml:"start_menu" json:"start_menu"`
	// Git are the settings files (relative to Path, * matches one folder)
	// kept in the repo, in windows/playnite.
	Git []string `yaml:"git" json:"git"`
}

// Default is what a missing file means.
func Default() Config {
	return Config{
		Path:      `C:\Playnite`,
		Exclude:   append([]string(nil), DefaultExclude...),
		StartMenu: true,
		Git:       append([]string(nil), DefaultGit...),
	}
}

// DefaultExclude are what Playnite rebuilds by itself: image and browser
// caches, the JIT profiles and the logs.
var DefaultExclude = []string{"cache", "browsercache", "JITProfiles", "*.log"}

// DefaultGit are the settings of Playnite and of its extensions.
var DefaultGit = []string{"config.json", "fullscreenConfig.json", "ExtensionsData/*/config.json"}

// GitDir is where the settings live in the repo.
func GitDir(root string) string { return filepath.Join(root, "windows", "playnite") }

// StoreExclude is what the copy in the store leaves out: the exclusions and
// what git carries whole.
func (c Config) StoreExclude() []string {
	return append(append([]string{}, c.Exclude...), snapshot.TopLevel(c.Git)...)
}

// Path is where the config lives in the repo.
func Path(root string) string { return filepath.Join(root, "config", "playnite.yaml") }

const header = `# The portable Playnite (Desktop and Fullscreen), kept as a snapshot outside
# the repo: covers, trailers, theme music, themes, extensions and their
# settings are all inside its folder, so a copy of the folder is the setup.
# Edited in the app's Playnite tab or with ` + "`envsetup set-playnite`" + `;
# ` + "`playnite-save`" + ` updates git and the store, ` + "`playnite-restore`" + ` puts everything back.
# The settings go to git (windows/playnite) without API keys and tokens; the
# library, covers, trailers, themes and extensions go to the store.
#
# path        the Playnite folder on this machine
# store       folder for the snapshot (second drive, external drive, NAS): the
#             copy goes to <store>\Playnite, with a manifest and the secrets
#             left out of git next to it
# exclude     left out of the snapshot: names with * or a dot are file
#             patterns, the rest are folders relative to path
# start_menu  after a restore, add Playnite to the Start menu and to
#             Settings > Apps (windows/register-portable-app.ps1)
# git         settings files of path kept in the repo (* matches one folder)

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
	return cfg, true, cfg.Validate()
}

// Validate refuses a config that would copy somewhere dangerous.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Path) == "" {
		return errors.New("playnite: path is empty")
	}
	for _, x := range append(append([]string{}, c.Exclude...), c.Git...) {
		if strings.Contains(x, "..") {
			return fmt.Errorf("playnite: %q leaves the folder", x)
		}
	}
	if c.Store == "" {
		return nil
	}
	p, s := clean(c.Path), clean(c.Store)
	snap := s + `\` + strings.ToLower(SnapshotDir)
	switch {
	case p == s:
		return errors.New("playnite: store is the Playnite folder itself")
	case strings.HasPrefix(s+`\`, p+`\`):
		return errors.New("playnite: store is inside the Playnite folder")
	case strings.HasPrefix(p+`\`, snap+`\`):
		return errors.New("playnite: the snapshot would be the Playnite folder itself")
	}
	return nil
}

func clean(p string) string { return snapshot.Clean(p) }

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
	SnapshotDir  = "Playnite"
	ManifestFile = "envsetup-playnite.json"
	// Exe is the program that tells a Playnite folder apart.
	Exe = "Playnite.DesktopApp.exe"
	// Program names the secrets file of the store.
	Program = "playnite"
)

// Snapshot is where the copy lives in the store.
func (c Config) Snapshot() string { return filepath.Join(c.Store, SnapshotDir) }

// Manifest describes a snapshot; it is written next to it.
type Manifest struct {
	SavedAt  time.Time `json:"saved_at"`
	Source   string    `json:"source"`
	Computer string    `json:"computer"`
	Bytes    int64     `json:"bytes"`
	Files    int       `json:"files"`
	// The themes in use and the extensions, to tell snapshots apart.
	DesktopTheme    string   `json:"desktop_theme"`
	FullscreenTheme string   `json:"fullscreen_theme"`
	Extensions      []string `json:"extensions"`
	// Secrets is how many values were left out of git and kept in the store.
	Secrets int `json:"secrets"`
}

// ReadManifest reads the manifest of the store; nil when there is none.
func ReadManifest(store string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(store, ManifestFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", ManifestFile, err)
	}
	return &m, nil
}

// WriteManifest saves m in the store.
func WriteManifest(store string, m Manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(store, ManifestFile), raw, 0o644)
}

// Theme is a theme found in Themes\Desktop or Themes\Fullscreen.
type Theme struct {
	Mode    string `json:"mode"` // desktop or fullscreen
	ID      string `json:"id"`
	Name    string `json:"name"`
	Author  string `json:"author"`
	Version string `json:"version"`
	Active  bool   `json:"active"`
}

// Extension is an add-on found in Extensions.
type Extension struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Author  string `json:"author"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	// Data is the folder of its settings in ExtensionsData, when found.
	Data string `json:"data,omitempty"`
}

// Folder is a top-level entry of the Playnite folder with its size.
type Folder = snapshot.Folder

// Install is what is in a Playnite folder.
type Install struct {
	Found      bool        `json:"found"`
	Themes     []Theme     `json:"themes"`
	Extensions []Extension `json:"extensions"`
	Folders    []Folder    `json:"folders"`
	// Bytes and Files are what a snapshot would copy (exclusions left out).
	Bytes int64 `json:"bytes"`
	Files int   `json:"files"`
}

// manifestYAML is the part of theme.yaml and extension.yaml that is read.
type manifestYAML struct {
	ID      string `yaml:"Id"`
	Name    string `yaml:"Name"`
	Author  string `yaml:"Author"`
	Version string `yaml:"Version"`
	Type    string `yaml:"Type"`
	Module  string `yaml:"Module"`
}

func readYAML(path string) (manifestYAML, error) {
	var m manifestYAML
	raw, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	err = yaml.Unmarshal(raw, &m)
	return m, err
}

// readJSON reads a Playnite settings file (which may start with a BOM).
func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}), v)
}

// Read describes the Playnite folder of c. sizes walks every file (a second
// or two for a library with trailers).
func Read(c Config, sizes bool) Install {
	in := Install{Themes: []Theme{}, Extensions: []Extension{}, Folders: []Folder{}}
	if _, err := os.Stat(filepath.Join(c.Path, Exe)); err != nil {
		return in
	}
	in.Found = true

	var desk struct {
		Theme           string
		DisabledPlugins []string
	}
	var full struct{ Theme string }
	_ = readJSON(filepath.Join(c.Path, "config.json"), &desk)
	_ = readJSON(filepath.Join(c.Path, "fullscreenConfig.json"), &full)

	for _, mode := range []string{"Desktop", "Fullscreen"} {
		active := desk.Theme
		if mode == "Fullscreen" {
			active = full.Theme
		}
		if active == "" {
			active = "Default"
		}
		dirs, _ := os.ReadDir(filepath.Join(c.Path, "Themes", mode))
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			t := Theme{Mode: strings.ToLower(mode), ID: d.Name(), Name: d.Name()}
			if m, err := readYAML(filepath.Join(c.Path, "Themes", mode, d.Name(), "theme.yaml")); err == nil {
				if m.ID != "" {
					t.ID = m.ID
				}
				if m.Name != "" {
					t.Name = m.Name
				}
				t.Author, t.Version = m.Author, m.Version
			}
			t.Active = strings.EqualFold(t.ID, active) || strings.EqualFold(d.Name(), active)
			in.Themes = append(in.Themes, t)
		}
	}

	disabled := map[string]bool{}
	for _, id := range desk.DisabledPlugins {
		disabled[strings.ToLower(id)] = true
	}
	data, _ := os.ReadDir(filepath.Join(c.Path, "ExtensionsData"))
	exts, _ := os.ReadDir(filepath.Join(c.Path, "Extensions"))
	modules := map[int]string{}
	for _, d := range exts {
		if !d.IsDir() {
			continue
		}
		m, err := readYAML(filepath.Join(c.Path, "Extensions", d.Name(), "extension.yaml"))
		if err != nil {
			continue
		}
		e := Extension{ID: m.ID, Name: m.Name, Author: m.Author, Version: m.Version, Type: m.Type, Enabled: !disabled[strings.ToLower(m.ID)]}
		if e.Name == "" {
			e.Name = d.Name()
		}
		e.Data = dataFolder(m.ID, data)
		if e.Data == "" && m.Module != "" {
			modules[len(in.Extensions)] = filepath.Join(c.Path, "Extensions", d.Name(), m.Module)
		}
		in.Extensions = append(in.Extensions, e)
	}
	matchModules(in.Extensions, modules, data)
	sort.Slice(in.Extensions, func(i, j int) bool {
		return strings.ToLower(in.Extensions[i].Name) < strings.ToLower(in.Extensions[j].Name)
	})

	if sizes {
		in.Folders, in.Bytes, in.Files = snapshot.Measure(c.Path, c.StoreExclude())
	}
	return in
}

var guidRE = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// dataFolder finds the ExtensionsData folder of an extension: the folder is
// named after the plugin's GUID, which most ids end with.
func dataFolder(id string, data []os.DirEntry) string {
	g := guidRE.FindString(id)
	for _, d := range data {
		if !d.IsDir() {
			continue
		}
		if (g != "" && strings.EqualFold(d.Name(), g)) || strings.EqualFold(d.Name(), id) {
			return d.Name()
		}
	}
	return ""
}

// matchModules finds the data folder of the extensions whose id does not
// carry their GUID: a .NET plugin declares it as a string, which is kept in
// UTF-16 inside its DLL.
//
// A DLL can hold the GUIDs of other plugins too (shared helper libraries
// do), so a folder is only given to an extension whose DLL names exactly one
// folder not taken yet; taking it can make another DLL unambiguous, so this
// repeats until nothing changes. What stays ambiguous is left without a
// folder. The result does not depend on the order of the map.
func matchModules(exts []Extension, modules map[int]string, data []os.DirEntry) {
	taken := map[string]bool{}
	for _, e := range exts {
		taken[strings.ToLower(e.Data)] = true
	}
	var free []string
	for _, d := range data {
		if d.IsDir() && guidRE.MatchString(d.Name()) && len(d.Name()) == 36 && !taken[strings.ToLower(d.Name())] {
			free = append(free, d.Name())
		}
	}
	utf16 := func(s string) []byte {
		b := make([]byte, 0, len(s)*2)
		for _, c := range []byte(s) {
			b = append(b, c, 0)
		}
		return b
	}
	// the free folders each DLL names
	names := map[int][]string{}
	for i, dll := range modules {
		info, err := os.Stat(dll)
		if err != nil || info.Size() > 64<<20 {
			continue
		}
		raw, err := os.ReadFile(dll)
		if err != nil {
			continue
		}
		for _, g := range free {
			if bytes.Contains(raw, utf16(strings.ToLower(g))) || bytes.Contains(raw, utf16(strings.ToUpper(g))) {
				names[i] = append(names[i], g)
			}
		}
	}
	for changed := true; changed; {
		changed = false
		// the DLLs left with one free folder, by folder: a folder two DLLs
		// claim stays free in this pass
		claims := map[string][]int{}
		for i, gs := range names {
			var left []string
			for _, g := range gs {
				if !taken[strings.ToLower(g)] {
					left = append(left, g)
				}
			}
			if len(left) == 1 {
				claims[left[0]] = append(claims[left[0]], i)
			}
		}
		for g, by := range claims {
			if len(by) == 1 {
				exts[by[0]].Data = g
				taken[strings.ToLower(g)] = true
				delete(names, by[0])
				changed = true
			}
		}
	}
}

// RobocopyArgs are the arguments to copy src to dst (see snapshot.RobocopyArgs).
func RobocopyArgs(src, dst string, exclude []string, mirror bool) []string {
	return snapshot.RobocopyArgs(src, dst, exclude, mirror)
}

// RewritePaths replaces the folder a snapshot was taken from (from) with the
// folder it was restored to (to) in Playnite's settings files: the extensions
// keep full paths to ffmpeg, yt-dlp and images. It returns the files changed.
func RewritePaths(dir, from, to string) ([]string, error) {
	if clean(from) == clean(to) || from == "" {
		return nil, nil
	}
	// JSON escapes the backslashes; match the folder only as a whole segment.
	esc := func(s string) string { return strings.ReplaceAll(strings.TrimRight(s, `\/`), `\`, `\\`) }
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(esc(from)) + `(\\\\|")`)
	repl := strings.ReplaceAll(esc(to), "$", "$$") + "$1"
	var changed []string
	visit := func(p string) error {
		info, err := os.Stat(p)
		if err != nil || info.Size() > 50<<20 {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out := re.ReplaceAll(raw, []byte(repl))
		if bytes.Equal(out, raw) {
			return nil
		}
		if err := os.WriteFile(p, out, info.Mode().Perm()); err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		changed = append(changed, rel)
		return nil
	}
	top, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, p := range top {
		if err := visit(p); err != nil {
			return changed, err
		}
	}
	err := filepath.WalkDir(filepath.Join(dir, "ExtensionsData"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".json") {
			return visit(p)
		}
		return nil
	})
	return changed, err
}

// External is a full path an extension's settings point to outside the
// Playnite folder: a program the snapshot does not carry.
type External struct {
	Extension string `json:"extension"` // name, or the data folder when unknown
	Key       string `json:"key"`
	Path      string `json:"path"`
}

// Externals lists the paths in the extensions' settings (keys ending in
// "Path") that are outside the Playnite folder.
func Externals(c Config, exts []Extension) []External {
	names := map[string]string{}
	for _, e := range exts {
		if e.Data != "" {
			names[strings.ToLower(e.Data)] = e.Name
		}
	}
	out := []External{}
	seen := map[string]bool{}
	dirs, _ := os.ReadDir(filepath.Join(c.Path, "ExtensionsData"))
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		var v any
		if readJSON(filepath.Join(c.Path, "ExtensionsData", d.Name(), "config.json"), &v) != nil {
			continue
		}
		name := names[strings.ToLower(d.Name())]
		if name == "" {
			name = d.Name()
		}
		walkPaths(v, "", func(key, p string) {
			if strings.HasPrefix(clean(p)+`\`, clean(c.Path)+`\`) || seen[clean(p)] {
				return
			}
			seen[clean(p)] = true
			out = append(out, External{Extension: name, Key: key, Path: p})
		})
	}
	return out
}

var absRE = regexp.MustCompile(`^[A-Za-z]:\\`)

func walkPaths(v any, key string, fn func(key, path string)) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			walkPaths(x, k, fn)
		}
	case []any:
		for _, x := range t {
			walkPaths(x, key, fn)
		}
	case string:
		if strings.HasSuffix(strings.ToLower(key), "path") && absRE.MatchString(t) {
			fn(key, t)
		}
	}
}
