package playnite

// Without the store, Playnite can still be put back from the internet and
// git: the program comes from its GitHub releases, and the add-ons (themes
// and extensions) listed in windows/playnite/addons.yaml come from the
// Playnite add-on database, at the version that was installed when possible.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"gopkg.in/yaml.v3"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// Releases is where the portable Playnite is downloaded from, and AddonAPI
// the add-on database Playnite itself uses (variables for the tests).
var (
	Releases = "https://github.com/JosefNemec/Playnite/releases/download"
	AddonAPI = "https://api.playnite.link/api/addons"
)

// ProgramURL is the portable Playnite of a release: a 7z with the program
// files at its root.
func ProgramURL(version string) string {
	return fmt.Sprintf("%s/%s/%s.7z", Releases, version, version)
}

// ParseVersion turns the file version of Playnite.dll ("10.64.0.0") into the
// release name ("10.64").
func ParseVersion(fileVersion string) string {
	parts := strings.Split(strings.TrimSpace(fileVersion), ".")
	if len(parts) < 2 {
		return ""
	}
	for _, p := range parts[:2] {
		if _, err := strconv.Atoi(p); err != nil {
			return ""
		}
	}
	return parts[0] + "." + parts[1]
}

// Addon is a theme or an extension of the Playnite folder.
type Addon struct {
	ID      string `yaml:"id" json:"id"`
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
	// Kind is extension, desktop (theme) or fullscreen (theme).
	Kind string `yaml:"kind" json:"kind"`
	// Folder is where it is installed, relative to the Playnite folder.
	Folder string `yaml:"folder" json:"folder"`
	// Builtin add-ons come with Playnite itself.
	Builtin bool `yaml:"builtin,omitempty" json:"builtin"`
}

// AddonList is windows/playnite/addons.yaml.
type AddonList struct {
	// Playnite is the release the list was made with.
	Playnite string  `yaml:"playnite" json:"playnite"`
	Addons   []Addon `yaml:"addons" json:"addons"`
}

// AddonsFile is where the list lives in the repo.
func AddonsFile(root string) string { return filepath.Join(GitDir(root), "addons.yaml") }

const addonsHeader = `# The themes and extensions of Playnite, written by playnite-save. Without the
# store (the HD), a restore downloads Playnite and installs these from the
# Playnite add-on database (api.playnite.link), at this version when it is
# still offered. builtin ones come with Playnite.

`

// ScanAddons lists the themes and extensions installed in dir.
func ScanAddons(dir string) []Addon {
	out := []Addon{}
	add := func(kind, rel, manifest string) {
		m, err := readYAML(filepath.Join(dir, filepath.FromSlash(rel), manifest))
		if err != nil {
			return
		}
		a := Addon{ID: m.ID, Name: m.Name, Version: m.Version, Kind: kind, Folder: rel}
		if a.ID == "" {
			a.ID = filepath.Base(rel)
		}
		if a.Name == "" {
			a.Name = a.ID
		}
		base := filepath.Base(rel)
		a.Builtin = strings.HasSuffix(base, "_Builtin") || strings.EqualFold(base, "Default")
		out = append(out, a)
	}
	exts, _ := os.ReadDir(filepath.Join(dir, "Extensions"))
	for _, d := range exts {
		if d.IsDir() {
			add("extension", "Extensions/"+d.Name(), "extension.yaml")
		}
	}
	for _, mode := range []string{"Desktop", "Fullscreen"} {
		themes, _ := os.ReadDir(filepath.Join(dir, "Themes", mode))
		for _, d := range themes {
			if d.IsDir() {
				add(strings.ToLower(mode), "Themes/"+mode+"/"+d.Name(), "theme.yaml")
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Folder < out[j].Folder })
	return out
}

// WriteAddons saves the list with its header; unchanged content is not
// written again.
func WriteAddons(file string, l AddonList) error {
	var b bytes.Buffer
	b.WriteString(addonsHeader)
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(l); err != nil {
		return err
	}
	if old, err := os.ReadFile(file); err == nil && bytes.Equal(old, b.Bytes()) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, b.Bytes(), 0o644)
}

// ReadAddons reads the list; an empty one when the file is missing.
func ReadAddons(file string) (AddonList, error) {
	var l AddonList
	raw, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, err
	}
	return l, yaml.Unmarshal(raw, &l)
}

var client = &http.Client{Timeout: 2 * time.Minute}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "envsetup")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// Catalog maps the id of every add-on of the database (lower case) to its
// installer manifest.
func Catalog(ctx context.Context) (map[string]string, error) {
	raw, err := get(ctx, AddonAPI)
	if err != nil {
		return nil, fmt.Errorf("the Playnite add-on database: %w", err)
	}
	var res struct {
		Data []struct {
			AddonID  string `json:"addonId"`
			Manifest string `json:"installerManifestUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("the Playnite add-on database: %w", err)
	}
	out := map[string]string{}
	for _, a := range res.Data {
		out[strings.ToLower(a.AddonID)] = a.Manifest
	}
	return out, nil
}

// pickPackage chooses from an installer manifest the package of version, or
// the newest one.
func pickPackage(manifest []byte, version string) (url, got string, err error) {
	type pkg struct {
		Version    string `yaml:"Version"`
		PackageURL string `yaml:"PackageUrl"`
	}
	var m struct{ Packages []pkg }
	// read as a node: some manifests repeat the Packages key, which a struct
	// refuses; the packages of every occurrence are kept
	var doc yaml.Node
	if err := yaml.Unmarshal(decodeText(manifest), &doc); err != nil {
		return "", "", err
	}
	if len(doc.Content) == 1 && doc.Content[0].Kind == yaml.MappingNode {
		kv := doc.Content[0].Content
		for i := 0; i+1 < len(kv); i += 2 {
			if kv[i].Value != "Packages" {
				continue
			}
			var list []pkg
			if err := kv[i+1].Decode(&list); err != nil {
				return "", "", err
			}
			m.Packages = append(m.Packages, list...)
		}
	}
	if len(m.Packages) == 0 {
		return "", "", errors.New("the manifest lists no package")
	}
	best := -1
	for i, p := range m.Packages {
		if p.Version == version && p.PackageURL != "" {
			return p.PackageURL, p.Version, nil
		}
		if p.PackageURL != "" && (best < 0 || newer(p.Version, m.Packages[best].Version)) {
			best = i
		}
	}
	if best < 0 {
		return "", "", errors.New("the manifest lists no package")
	}
	return m.Packages[best].PackageURL, m.Packages[best].Version, nil
}

// newer compares dotted versions ("3.10" > "3.9").
func newer(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// InstallAddon downloads an add-on from its installer manifest and extracts
// it to its folder in dir. It returns the version installed.
func InstallAddon(ctx context.Context, a Addon, manifestURL, dir string, out io.Writer) (string, error) {
	manifest, err := get(ctx, manifestURL)
	if err != nil {
		return "", err
	}
	url, version, err := pickPackage(manifest, a.Version)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "envsetup-addon-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	file := filepath.Join(tmp, "addon.zip")
	if err := snapshot.Download(ctx, url, file, io.Discard); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, filepath.FromSlash(a.Folder))
	if !snapshot.Inside(dest, dir) || snapshot.Clean(dest) == snapshot.Clean(dir) {
		return "", fmt.Errorf("the folder %q leaves the Playnite folder", a.Folder)
	}
	if _, _, err := snapshot.Unzip(file, dest); err != nil {
		return "", err
	}
	return version, nil
}

// decodeText turns a manifest written in UTF-16 (some add-ons do) into UTF-8.
func decodeText(raw []byte) []byte {
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE {
		u := make([]uint16, 0, len(raw)/2)
		for i := 2; i+1 < len(raw); i += 2 {
			u = append(u, uint16(raw[i])|uint16(raw[i+1])<<8)
		}
		return []byte(string(utf16.Decode(u)))
	}
	return bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
}
