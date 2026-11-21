// Package layout describes and applies how the Windows desktop is laid out:
// which apps are pinned to the taskbar and in what order, whether the Start
// menu keeps its pinned apps and whether the desktop icons are hidden. The
// choice lives in config/layout.yaml.
package layout

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/install"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/ninite"
)

// Config is config/layout.yaml.
type Config struct {
	// Taskbar is the pinned apps, left to right, by Item.ID. Empty leaves the
	// taskbar as it is.
	Taskbar []string `yaml:"taskbar" json:"taskbar"`
	// ClearStart removes the apps pinned to the Start menu.
	ClearStart bool `yaml:"clear_start" json:"clear_start"`
	// HideDesktopIcons hides every icon on the desktop.
	HideDesktopIcons bool `yaml:"hide_desktop_icons" json:"hide_desktop_icons"`
	// TaskbarAutoHide turns "Automatically hide the taskbar" on (the dock of
	// the Windhawk mod hides with it) or off.
	TaskbarAutoHide bool `yaml:"taskbar_autohide" json:"taskbar_autohide"`
	// SingleEntries keeps one launcher entry per app: a desktop shortcut that
	// repeats a Start menu one (same name, same program), and a second Start
	// menu shortcut of the same app, are moved to a backup folder. Launchers
	// such as Raycast list every shortcut, so an app showed up twice, once of
	// them with the icon the icons step did not reach. On unless the file says
	// otherwise.
	SingleEntries bool `yaml:"single_entries" json:"single_entries"`
}

// Path is where the layout lives in the repo.
func Path(root string) string { return filepath.Join(root, "config", "layout.yaml") }

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

// Load reads the layout. A missing file is not an error: exists tells.
func Load(path string) (cfg Config, exists bool, err error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{Taskbar: []string{}, SingleEntries: true}, false, nil
	}
	if err != nil {
		return cfg, false, err
	}
	cfg.SingleEntries = true // a file from before the option has it on
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, true, fmt.Errorf("%s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, true, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Taskbar == nil {
		cfg.Taskbar = []string{}
	}
	return cfg, true, nil
}

// Validate refuses ids that could not come from the app and duplicates.
func (c Config) Validate() error {
	seen := map[string]bool{}
	for _, id := range c.Taskbar {
		if !idRe.MatchString(id) {
			return fmt.Errorf("%q is not a valid app id", id)
		}
		if seen[id] {
			return fmt.Errorf("%q is pinned twice", id)
		}
		seen[id] = true
	}
	return nil
}

const header = `# How the Windows desktop is laid out, edited in the app's Preview tab (Windows) or with
# ` + "`envsetup set-layout`" + `; ` + "`envsetup apply-layout`" + ` writes it to the machine.
#
# taskbar            apps pinned to the taskbar, left to right. Empty leaves the
#                    taskbar as it is. Ids are the ones the Preview tab shows.
# clear_start        remove the apps pinned to the Start menu
# hide_desktop_icons hide every icon on the desktop
# taskbar_autohide   hide the taskbar automatically (the Windhawk dock hides with it)
# single_entries     one launcher entry per app: a desktop shortcut that repeats a Start
#                    menu one (same name and program) is moved to a backup folder
`

// Save writes the layout, replacing the file.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.Taskbar == nil {
		c.Taskbar = []string{}
	}
	body, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append([]byte(header+"\n"), body...), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Where an item comes from.
const (
	SourceWindows  = "windows"  // part of Windows
	SourceNinite   = "ninite"   // the Ninite list
	SourceUnattend = "unattend" // a first-logon script of the answer file
	SourceCatalog  = "catalog"  // an app of the profile that is ticked in Configurar
)

// Item is an app that can be put on the taskbar.
type Item struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
	// Pinnable is false for what has no window to pin (Java, Node, runtimes).
	Pinnable bool `json:"pinnable"`
	// Desktop is set when the installer usually leaves a shortcut on the desktop.
	Desktop bool   `json:"desktop"`
	Glyph   string `json:"glyph"`
	Color   string `json:"color"`

	// Match are the names of the Start menu shortcut; AUMID is the app id of
	// a built-in app, whose shortcut is also found by it in another language.
	// The apply script resolves them on the machine.
	Match []string `json:"-"`
	AUMID string   `json:"-"`
	// Browser pins the default browser of the machine, or the first of Match
	// that is installed when the default is Edge or unknown.
	Browser bool `json:"-"`
}

type known struct {
	id, name string
	ninite   []string // slugs on ninite.com
	apps     []string // ids in config/apps.yaml
	github   []string // repositories the answer file installs, by name
	match    []string
	aumid    string
	glyph    string
	color    string
	desktop  bool
	browser  bool
}

// The apps the preview knows how to draw and pin. Anything else that is
// installed still shows up, in grey.
var table = []known{
	{id: "explorer", name: "Explorador de Arquivos", aumid: "Microsoft.Windows.Explorer", match: []string{"File Explorer", "Explorador de Arquivos"}, glyph: "🗂", color: "#f2b632"},
	// Whatever browser the machine uses: someone who picks Opera GX instead of
	// Dia keeps the same layout.
	{id: "browser", name: "Navegador padrão", match: BrowserNames, glyph: "🌐", color: "#3b82f6", browser: true},
	{id: "edge", name: "Microsoft Edge", aumid: "MSEdge", match: []string{"Microsoft Edge"}, ninite: []string{"edge"}, glyph: "e", color: "#0f7cc0"},
	{id: "chrome", name: "Chrome", ninite: []string{"chrome"}, apps: []string{"google-chrome"}, match: []string{"Google Chrome"}, glyph: "C", color: "#4285f4", desktop: true},
	{id: "firefox", name: "Firefox", ninite: []string{"firefox"}, match: []string{"Firefox"}, glyph: "F", color: "#ff7139", desktop: true},
	{id: "dia", name: "Dia", apps: []string{"dia"}, match: []string{"Dia"}, glyph: "D", color: "#5b5bf7"},
	{id: "brave", name: "Brave", ninite: []string{"brave"}, apps: []string{"brave"}, match: []string{"Brave"}, glyph: "B", color: "#fb542b", desktop: true},
	{id: "opera", name: "Opera", ninite: []string{"operaChromium"}, match: []string{"Opera"}, glyph: "O", color: "#ff1b2d", desktop: true},
	{id: "opera-gx", name: "Opera GX", apps: []string{"opera-gx"}, match: []string{"Opera GX"}, glyph: "GX", color: "#fa1e4e", desktop: true},
	{id: "vivaldi", name: "Vivaldi", ninite: []string{"vivaldi"}, match: []string{"Vivaldi"}, glyph: "V", color: "#ef3939", desktop: true},
	{id: "discord", name: "Discord", ninite: []string{"discord"}, apps: []string{"discord"}, match: []string{"Discord"}, glyph: "D", color: "#5865f2", desktop: true},
	{id: "steam", name: "Steam", ninite: []string{"steam"}, apps: []string{"steam"}, match: []string{"Steam"}, glyph: "S", color: "#1b2838", desktop: true},
	{id: "playnite", name: "Playnite", apps: []string{"playnite"}, match: []string{"Playnite"}, glyph: "P", color: "#7b2fbe"},
	{id: "retroarch", name: "RetroArch", apps: []string{"retroarch"}, match: []string{"RetroArch"}, glyph: "R", color: "#3d3d3d"},
	{id: "epic", name: "Epic Games Launcher", ninite: []string{"epic"}, apps: []string{"epic-games"}, match: []string{"Epic Games Launcher"}, glyph: "E", color: "#2a2a2a", desktop: true},
	{id: "spotify", name: "Spotify", ninite: []string{"spotify"}, apps: []string{"spotify"}, match: []string{"Spotify"}, glyph: "♫", color: "#1db954"},
	{id: "powertoys", name: "PowerToys", ninite: []string{"powertoys"}, apps: []string{"powertoys"}, match: []string{"PowerToys"}, glyph: "P", color: "#f2c811"},
	{id: "git", name: "Git Bash", ninite: []string{"git"}, match: []string{"Git Bash"}, glyph: "G", color: "#f05032"},
	{id: "nvidia-app", name: "NVIDIA app", apps: []string{"nvidia-app"}, match: []string{"NVIDIA app"}, glyph: "N", color: "#76b900", desktop: true},
	{id: "amd-software", name: "AMD Software", apps: []string{"amd-software"}, match: []string{"AMD Software"}, glyph: "A", color: "#ed1c24"},
	{id: "hydra", name: "Hydra Launcher", github: []string{"hydra"}, match: []string{"Hydra Launcher", "Hydra"}, glyph: "H", color: "#1c9ad6", desktop: true},
	{id: "proton-vpn", name: "Proton VPN", apps: []string{"proton-vpn"}, match: []string{"Proton VPN"}, glyph: "▽", color: "#6d4aff"},
	{id: "obs-studio", name: "OBS Studio", apps: []string{"obs-studio"}, match: []string{"OBS Studio"}, glyph: "◉", color: "#302e31", desktop: true},
	{id: "gog-galaxy", name: "GOG Galaxy", apps: []string{"gog-galaxy"}, match: []string{"GOG GALAXY"}, glyph: "G", color: "#8f3ad0"},
	{id: "ea-app", name: "EA app", apps: []string{"ea-app"}, match: []string{"EA"}, glyph: "EA", color: "#ff4747"},
	{id: "ubisoft-connect", name: "Ubisoft Connect", apps: []string{"ubisoft-connect"}, match: []string{"Ubisoft Connect"}, glyph: "U", color: "#0070ff"},
	{id: "battle-net", name: "Battle.net", apps: []string{"battle-net"}, match: []string{"Battle.net"}, glyph: "B", color: "#148eff"},
	{id: "msi-afterburner", name: "MSI Afterburner", apps: []string{"msi-afterburner"}, match: []string{"MSI Afterburner"}, glyph: "M", color: "#c8102e"},
	{id: "hwinfo", name: "HWiNFO", apps: []string{"hwinfo"}, match: []string{"HWiNFO"}, glyph: "i", color: "#2d7dd2"},
	{id: "7zip", name: "7-Zip", ninite: []string{"7zip"}, apps: []string{"7zip"}, match: []string{"7-Zip File Manager"}, glyph: "7z", color: "#1d6b3a"},
}

// windowsItems are always on offer.
var windowsItems = []string{"explorer", "browser", "edge"}

// BrowserNames are the Start menu shortcuts of the browsers, in the order the
// item "browser" falls back to when the default browser is Edge or unknown.
var BrowserNames = []string{"Dia", "Brave", "Opera GX", "Google Chrome", "Firefox", "Vivaldi", "Opera"}

// noWindow tells the Ninite apps that install nothing to open.
func noWindow(slug, category string) bool {
	switch category {
	case ".NET", "Java", "Runtimes":
		return true
	}
	for _, p := range []string{"node.js", "python", "vcredist", ".net", "adopt", "corretto", "klite", "cccp", "vsto", "edgewebview"} {
		if strings.HasPrefix(strings.ToLower(slug), p) {
			return true
		}
	}
	return false
}

func (k known) item(sources []string) Item {
	return Item{ID: k.id, Name: k.name, Sources: sources, Pinnable: true, Desktop: k.desktop,
		Glyph: k.glyph, Color: k.color, Match: k.match, AUMID: k.aumid, Browser: k.browser}
}

// Inputs are what decides which apps the machine will have.
type Inputs struct {
	Ninite   []string      // slugs of the Ninite list
	Apps     []string      // ids of the profile's apps that are ticked
	Installs []install.App // what the answer file installs
}

func set(list []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range list {
		m[s] = true
	}
	return m
}

func anyIn(m map[string]bool, list []string) bool {
	for _, s := range list {
		if m[s] {
			return true
		}
	}
	return false
}

// Candidates lists the apps the machine will have, in a stable order:
// Windows' own, the ones the preview knows, then the rest of the Ninite list
// and of the answer file.
func Candidates(in Inputs) []Item {
	onNinite, apps := set(in.Ninite), set(in.Apps)
	github := map[string]bool{}
	for _, i := range in.Installs {
		if i.Kind == "github" {
			github[i.ID] = true
		}
	}
	out := []Item{}
	covered := map[string]bool{}
	for _, k := range table {
		var src []string
		if contains(windowsItems, k.id) {
			src = append(src, SourceWindows)
		}
		if anyIn(onNinite, k.ninite) {
			src = append(src, SourceNinite)
		}
		if anyIn(github, k.github) {
			src = append(src, SourceUnattend)
		}
		if anyIn(apps, k.apps) {
			src = append(src, SourceCatalog)
		}
		for _, s := range k.ninite {
			covered[s] = true
		}
		for _, g := range k.github {
			covered["github:"+g] = true
		}
		if len(src) > 0 {
			out = append(out, k.item(src))
		}
	}
	cat := map[string]ninite.App{}
	for _, a := range ninite.Catalog {
		cat[a.Slug] = a
	}
	for _, slug := range in.Ninite {
		if covered[slug] {
			continue
		}
		out = append(out, niniteItem(slug, cat[slug]))
	}
	for _, i := range in.Installs {
		if i.Kind == "github" && !covered["github:"+i.ID] {
			out = append(out, githubItem(i.ID, i.Name))
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}

func niniteItem(slug string, a ninite.App) Item {
	name := a.Name
	if name == "" {
		name = slug
	}
	return Item{ID: "ninite:" + slug, Name: name, Sources: []string{SourceNinite}, Pinnable: !noWindow(slug, a.Category),
		Glyph: initial(name), Color: "#6b7280", Match: []string{name}}
}

func githubItem(id, name string) Item {
	return Item{ID: "github:" + id, Name: name, Sources: []string{SourceUnattend}, Pinnable: true,
		Glyph: initial(name), Color: "#6b7280", Match: []string{name}}
}

// Find turns a saved id back into an item, without knowing the machine's
// lists: the table knows its own ids, and the others carry the name.
func Find(id string) (Item, bool) {
	for _, k := range table {
		if k.id == id {
			return k.item(nil), true
		}
	}
	if slug, ok := strings.CutPrefix(id, "ninite:"); ok {
		for _, a := range ninite.Catalog {
			if a.Slug == slug {
				return niniteItem(slug, a), true
			}
		}
	}
	if name, ok := strings.CutPrefix(id, "github:"); ok && name != "" {
		return githubItem(name, strings.ToUpper(name[:1])+name[1:]), true
	}
	return Item{}, false
}
