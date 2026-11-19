// Package config loads the declarative environment definition: the app
// catalogue (config/apps.yaml) and the profiles (config/profiles/*.yaml).
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Install methods an app can declare.
const (
	MethodNative  = "native"  // distro package manager (apt, pacman)
	MethodAUR     = "aur"     // Arch User Repository through yay/paru
	MethodDebURL  = "deb-url" // download a .deb and install it with apt
	MethodFlatpak = "flatpak" // flatpak from flathub
	MethodScript  = "script"  // arbitrary shell, guarded by Check
	MethodWinget  = "winget"  // Windows Package Manager; the native method on Windows
	MethodScoop   = "scoop"   // scoop (Windows)
	MethodSteam   = "steam"   // a Steam app, by AppID (Windows)
	MethodNinite  = "ninite"  // one Ninite installer for a list of its apps (Windows)
	// MethodPolicies applies config/policies.yaml: group policies, written to
	// the registry (Windows).
	MethodPolicies = "policies"
	// MethodPlaynite restores the Playnite snapshot of config/playnite.yaml
	// (Windows).
	MethodPlaynite = "playnite"
	// MethodRetroArch restores the RetroArch and games snapshot of
	// config/retroarch.yaml (Windows).
	MethodRetroArch = "retroarch"
	// MethodWindhawk imports windows/windhawk/backup.json into Windhawk 2
	// (Windows).
	MethodWindhawk = "windhawk"
)

var validMethods = map[string]bool{
	MethodNative: true, MethodAUR: true, MethodDebURL: true,
	MethodFlatpak: true, MethodScript: true,
	MethodWinget: true, MethodScoop: true, MethodSteam: true, MethodNinite: true, MethodPolicies: true, MethodPlaynite: true, MethodRetroArch: true, MethodWindhawk: true,
}

// Repo is a third-party apt repository an app needs before it can be
// installed.
type Repo struct {
	// Key is the URL of the signing key (armored or binary).
	Key string `yaml:"key"`
	// Source is what follows the options in a sources.list line: the URL,
	// the suite and the components. {codename} and {arch} are expanded.
	Source string `yaml:"source"`
}

// Wildcard is the map key used as a fallback when no distro key matches.
const Wildcard = "*"

// App is one installable thing. Every map is keyed by a distro ID
// ("ubuntu", "arch", "cachyos"...), an ID_LIKE family ("debian"), or "*" as a
// fallback. Lookup order is the exact ID, then each ID_LIKE entry, then "*".
type App struct {
	Description string `yaml:"description"`
	// Names is the package name per distro: a flatpak or winget ID, a Steam
	// AppID, or several native packages separated by spaces.
	Names map[string]string `yaml:"names"`
	// Method defaults to "native" when absent.
	Method map[string]string `yaml:"method"`
	// Source is the download URL for deb-url, or the shell script for script.
	// {arch} expands to amd64/arm64 and {x64} to x64/arm64.
	Source map[string]string `yaml:"source"`
	// Check is a shell command that exits 0 when a script app is installed.
	Check map[string]string `yaml:"check"`
	// Post commands run after the app is present. They run on every apply, so
	// they must be idempotent.
	Post map[string][]string `yaml:"post"`
	// Unavailable lists distro keys where the app does not exist.
	Unavailable []string `yaml:"unavailable"`
	// Repo is the apt repository to register before installing (Debian
	// family only).
	Repo map[string]Repo `yaml:"repo"`
}

// File is a config file to deploy.
type File struct {
	Src  string `yaml:"src"`  // relative to the repo root
	Dest string `yaml:"dest"` // ~ is expanded to the target user's home
	Mode string `yaml:"mode"` // link (default) or copy
	// Only restricts the entry to the listed distro keys (empty = all).
	Only []string `yaml:"only"`
	// App ties the file to an app: it is deployed only when that app is
	// part of the plan.
	App string `yaml:"app"`
}

// Option is one alternative of a choice step.
type Option struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Apps        []string `yaml:"apps"`
	Files       []File   `yaml:"files"`
	// Dirs are directories to create (~ is the home).
	Dirs []string `yaml:"dirs"`
	// VSCodeExtensions is a path to an extensions.json export.
	VSCodeExtensions string `yaml:"vscode_extensions"`
}

// Input is a value the user types in a step (a name, an e-mail). It is never
// stored in the repo: front-ends keep it locally and pass it with --set, and
// scripts read it as $ENVSETUP_<ID>, upper-cased with dots as underscores.
type Input struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
	// From is a shell command whose output pre-fills the field.
	From string `yaml:"from"`
}

// Step is one stage of the setup. It can list apps the user toggles
// (Apps start selected, Optional start unselected), files, and a
// single-choice list of Options.
type Step struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Apps        []string `yaml:"apps"`
	Optional    []string `yaml:"optional"`
	Files       []File   `yaml:"files"`
	// Dirs are directories to create (~ is the home).
	Dirs []string `yaml:"dirs"`
	// ChoiceTitle labels the Options group in the UI.
	ChoiceTitle string   `yaml:"choice_title"`
	Options     []Option `yaml:"options"`
	// Default is the option used when none is chosen. "none" is always a
	// valid choice and skips the options.
	Default string `yaml:"default"`
	// Inputs are values the step's scripts need from the user.
	Inputs []Input `yaml:"inputs"`
}

// Option returns the option with the given id.
func (s Step) Option(id string) (Option, bool) {
	for _, o := range s.Options {
		if o.ID == id {
			return o, true
		}
	}
	return Option{}, false
}

// Dock describes the application dock.
type Dock struct {
	// Position is bottom (default), left, right or top.
	Position string `yaml:"position"`
	AutoHide bool   `yaml:"autohide"`
	// Pinned is the ordered list of desktop-entry ids (without .desktop).
	// The apply step writes it to the real desktop configuration.
	Pinned []string `yaml:"pinned"`
}

// Bar describes the top (or bottom) panel: widget names by area.
type Bar struct {
	Position string   `yaml:"position"`
	Start    []string `yaml:"start"`
	Center   []string `yaml:"center"`
	End      []string `yaml:"end"`
}

// Desktop is what the desktop looks like: used by the preview, and by the
// apply step that writes the dock order to the system.
type Desktop struct {
	// Kind is niri-noctalia or gnome.
	Kind string `yaml:"kind"`
	// App is the app that provides the desktop; the dock is only applied
	// when it is part of the plan.
	App string `yaml:"app"`
	// Wallpaper is a repo path, a ~ path or an https URL.
	Wallpaper string `yaml:"wallpaper"`
	IconTheme string `yaml:"icon_theme"`
	Colors    struct {
		Accent string `yaml:"accent"`
		Panel  string `yaml:"panel"`
		Text   string `yaml:"text"`
	} `yaml:"colors"`
	Dock Dock `yaml:"dock"`
	Bar  Bar  `yaml:"bar"`
}

// Media describes the Windows installation drive of a profile: what goes on
// a Ventoy USB stick so the system installs itself.
type Media struct {
	// Unattend is the autounattend.xml answer file, relative to the repo root.
	Unattend string `yaml:"unattend"`
	// Ventoy is the ventoy.json that ties the answer file to the image.
	Ventoy string `yaml:"ventoy"`
	// Ninite is the catalogue app (method ninite) whose installer is put on
	// the drive, for the answer file's first-logon script to run.
	Ninite string `yaml:"ninite"`
}

// Profile is a complete environment for one kind of machine.
type Profile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	// Environment is "dev" or "gaming".
	Environment string `yaml:"environment"`
	// Match lists the distro keys (ID or ID_LIKE, or "windows") this
	// profile is meant for; it is how the profile is picked automatically.
	Match   []string `yaml:"match"`
	Desktop Desktop  `yaml:"desktop"`
	// Media is the unattended Windows installation; empty on Linux profiles.
	Media Media  `yaml:"media"`
	Steps []Step `yaml:"steps"`
	// Files are extra managed files, always deployed. `envsetup add`
	// appends here.
	Files []File `yaml:"files"`
	// Apps are extra apps outside any step, always selected by default.
	Apps []string `yaml:"apps"`
}

// MatchProfile picks the profile for a host: candidates are tried most
// specific first, so "cachyos" wins over the generic "arch".
func (c *Config) MatchProfile(candidates []string) (string, bool) {
	for _, cand := range candidates {
		for _, id := range c.ProfileNames() {
			for _, m := range c.Profiles[id].Match {
				if m == cand {
					return id, true
				}
			}
		}
	}
	return "", false
}

// Config is the full parsed definition.
type Config struct {
	Apps     map[string]App
	Profiles map[string]Profile
}

// Load reads <root>/config/apps.yaml and <root>/config/profiles/*.yaml.
func Load(root string) (*Config, error) {
	cfg := &Config{Apps: map[string]App{}, Profiles: map[string]Profile{}}

	var catalogue struct {
		Apps map[string]App `yaml:"apps"`
	}
	if err := readYAML(filepath.Join(root, "config", "apps.yaml"), &catalogue); err != nil {
		return nil, err
	}
	cfg.Apps = catalogue.Apps

	matches, err := filepath.Glob(filepath.Join(root, "config", "profiles", "*.yaml"))
	if err != nil {
		return nil, err
	}
	for _, m := range matches {
		var p Profile
		if err := readYAML(m, &p); err != nil {
			return nil, err
		}
		id := filepath.Base(m[:len(m)-len(filepath.Ext(m))])
		if p.Name == "" {
			p.Name = id
		}
		cfg.Profiles[id] = p
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func readYAML(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true) // typos in keys are errors, not silent no-ops
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Validate checks cross references and method names.
func (c *Config) Validate() error {
	for id, a := range c.Apps {
		for key, m := range a.Method {
			if !validMethods[m] {
				return fmt.Errorf("app %q: unknown method %q for %q", id, m, key)
			}
		}
		for key, r := range a.Repo {
			if !strings.HasPrefix(r.Key, "https://") || len(strings.Fields(r.Source)) < 2 {
				return fmt.Errorf("app %q: repo for %q needs an https key and a source of \"url suite [components]\"", id, key)
			}
		}
	}
	for pid, p := range c.Profiles {
		switch p.Desktop.Kind {
		case "", "niri-noctalia", "gnome":
		default:
			return fmt.Errorf("profile %q: unknown desktop kind %q", pid, p.Desktop.Kind)
		}
		switch p.Desktop.Dock.Position {
		case "", "bottom", "top", "left", "right":
		default:
			return fmt.Errorf("profile %q: bad dock position %q", pid, p.Desktop.Dock.Position)
		}
		if p.Desktop.App != "" {
			if _, ok := c.Apps[p.Desktop.App]; !ok {
				return fmt.Errorf("profile %q: desktop app %q is unknown", pid, p.Desktop.App)
			}
		}
		for _, id := range p.Apps {
			if _, ok := c.Apps[id]; !ok {
				return fmt.Errorf("profile %q: unknown app %q", pid, id)
			}
		}
		for _, path := range []string{p.Media.Unattend, p.Media.Ventoy} {
			if clean := filepath.ToSlash(filepath.Clean(path)); path != "" && (filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, "../")) {
				return fmt.Errorf("profile %q: media path %q must be inside the repo", pid, path)
			}
		}
		if id := p.Media.Ninite; id != "" {
			if _, ok := c.Apps[id]; !ok {
				return fmt.Errorf("profile %q: media ninite app %q is unknown", pid, id)
			}
		}
		files := append([]File{}, p.Files...)
		seen := map[string]bool{}
		for _, st := range p.Steps {
			if st.ID == "" || seen[st.ID] {
				return fmt.Errorf("profile %q: step ids must be set and unique (%q)", pid, st.ID)
			}
			seen[st.ID] = true
			for _, in := range st.Inputs {
				if !inputID.MatchString(in.ID) {
					return fmt.Errorf("profile %q: step %q: bad input id %q", pid, st.ID, in.ID)
				}
			}
			ids := append(append([]string{}, st.Apps...), st.Optional...)
			files = append(files, st.Files...)
			opts := map[string]bool{}
			for _, o := range st.Options {
				if o.ID == "" || o.ID == "none" || opts[o.ID] {
					return fmt.Errorf("profile %q: step %q: bad or duplicate option id %q", pid, st.ID, o.ID)
				}
				opts[o.ID] = true
				ids = append(ids, o.Apps...)
				files = append(files, o.Files...)
			}
			if st.Default != "" && st.Default != "none" && !opts[st.Default] {
				return fmt.Errorf("profile %q: step %q: default %q is not an option", pid, st.ID, st.Default)
			}
			for _, id := range ids {
				if _, ok := c.Apps[id]; !ok {
					return fmt.Errorf("profile %q: step %q: unknown app %q", pid, st.ID, id)
				}
			}
		}
		for _, f := range files {
			if f.Src == "" || f.Dest == "" {
				return fmt.Errorf("profile %q: file entry needs src and dest", pid)
			}
			if f.Mode != "" && f.Mode != "link" && f.Mode != "copy" {
				return fmt.Errorf("profile %q: file %q has invalid mode %q", pid, f.Src, f.Mode)
			}
		}
	}
	return nil
}

// inputID is what an input id (and a --set key) may look like.
var inputID = regexp.MustCompile(`^[a-z][a-z0-9]*([._][a-z0-9]+)*$`)

// ValidVar reports whether key can be used with --set.
func ValidVar(key string) bool { return inputID.MatchString(key) }

// ProfileNames returns the profile IDs sorted.
func (c *Config) ProfileNames() []string {
	out := make([]string, 0, len(c.Profiles))
	for id := range c.Profiles {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Pick resolves a per-distro map for the given candidate keys (most specific
// first), falling back to the wildcard entry.
func Pick[T any](m map[string]T, candidates []string) (T, bool) {
	for _, k := range candidates {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	v, ok := m[Wildcard]
	return v, ok
}
