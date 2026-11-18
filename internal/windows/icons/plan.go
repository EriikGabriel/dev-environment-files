package icons

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Shortcut is a .lnk found by the scan script.
type Shortcut struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Scope  string `json:"scope"`
	Path   string `json:"path"`
	Target string `json:"target"`
	// Themed is set when the shortcut already points at an icon made here;
	// Src is then the icon it had before.
	Themed bool `json:"themed"`
	// Src is a PNG of the icon the shortcut shows (the original one).
	Src string `json:"src"`
}

// Where a shortcut's new icon comes from.
const (
	FromStyle    = "style"    // drawn from the original icon
	FromPack     = "pack"     // a file of the icon pack, untouched
	FromOverride = "override" // a file chosen for this shortcut
	FromSkip     = "skip"     // left as it is
)

// Choice is the decision for one shortcut.
type Choice struct {
	From string `json:"from"`
	// File is the pack or override file; empty for style and skip.
	File string `json:"file,omitempty"`
}

// BuiltinPack is the pack the repo ships (relative to its root): the app
// icons of the MacTahoe theme, made by cmd/iconpack.
const BuiltinPack = "windows/icons/mactahoe"

// Pack is a folder of icons named after the apps. It may hold an
// aliases.json with other names its icons answer to ({"code-oss": "vscode"}).
type Pack struct {
	Dir     string
	files   map[string]string // Key(name) -> path
	aliases map[string]string // Key(other name) -> path
	names   map[string]string // path -> name of the file, without extension
}

var packExt = map[string]bool{".ico": true, ".png": true}

// LoadPack lists a pack folder. An empty dir gives no pack; a folder that
// cannot be read is an error. A .ico wins over a .png of the same name, and
// of two names with the same key ("steam", "Steam++") the shorter one.
func LoadPack(dir string) (*Pack, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	p := &Pack{Dir: dir, files: map[string]string{}, aliases: map[string]string{}, names: map[string]string{}}
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || !packExt[ext] {
			continue
		}
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		k := Key(name)
		if k == "" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if cur, ok := p.files[k]; ok {
			curName := p.names[cur]
			curICO := strings.EqualFold(filepath.Ext(cur), ".ico")
			switch {
			case len(curName) < len(name), len(curName) == len(name) && curICO:
				continue
			case len(curName) == len(name) && ext != ".ico":
				continue
			}
			delete(p.names, cur)
		}
		p.files[k] = path
		p.names[path] = name
	}
	raw, err := os.ReadFile(filepath.Join(dir, "aliases.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return nil, err
	}
	var aliases map[string]string
	if err := json.Unmarshal(raw, &aliases); err != nil {
		return nil, fmt.Errorf("aliases.json: %w", err)
	}
	for alias, target := range aliases {
		k := Key(alias)
		f, ok := p.files[Key(target)]
		if _, taken := p.files[k]; !ok || taken || k == "" {
			continue
		}
		p.aliases[k] = f
	}
	return p, nil
}

// Count is how many icons the pack has.
func (p *Pack) Count() int {
	if p == nil {
		return 0
	}
	return len(p.files)
}

// PackIcon is one icon of a pack.
type PackIcon struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// Search lists the icons whose name, or one of their other names, contains
// the query (all of them when it is empty), in the order of the names.
func (p *Pack) Search(query string) []PackIcon {
	if p == nil {
		return nil
	}
	q := Key(query)
	hit := map[string]bool{}
	for k, f := range p.files {
		if strings.Contains(k, q) {
			hit[f] = true
		}
	}
	if q != "" {
		for k, f := range p.aliases {
			if strings.Contains(k, q) {
				hit[f] = true
			}
		}
	}
	out := make([]PackIcon, 0, len(hit))
	for f := range hit {
		out = append(out, PackIcon{Name: p.names[f], File: f})
	}
	sort.Slice(out, func(i, j int) bool {
		// names that start with the query come first
		a, b := strings.HasPrefix(Key(out[i].Name), q), strings.HasPrefix(Key(out[j].Name), q)
		if a != b {
			return a
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// windowsNames are the icons, by the names icon themes use, of programs whose
// Windows shortcut or executable is called something else.
var windowsNames = map[string][]string{
	"cmd":               {"terminal"},
	"commandprompt":     {"terminal"},
	"powershell":        {"powershell", "terminal"},
	"pwsh":              {"powershell", "terminal"},
	"powershellise":     {"powershell", "terminal"},
	"windowsterminal":   {"terminal"},
	"wt":                {"terminal"},
	"gitbash":           {"git"},
	"gitcmd":            {"git"},
	"gitgui":            {"git"},
	"msedge":            {"microsoft-edge"},
	"code":              {"visual-studio-code", "vscode"},
	"taskmgr":           {"utilities-system-monitor", "system-monitor"},
	"perfmon":           {"utilities-system-monitor", "system-monitor"},
	"charmap":           {"accessories-character-map"},
	"cleanmgr":          {"accessories-system-cleaner"},
	"dfrgui":            {"gnome-disks", "disk-utility"},
	"msinfo32":          {"hwinfo", "hardinfo"},
	"explorer":          {"file-manager", "system-file-manager"},
	"control":           {"preferences-system"},
	"msconfig":          {"preferences-system"},
	"systemsettings":    {"preferences-system"},
	"osk":               {"preferences-desktop-keyboard"},
	"notepad":           {"text-editor", "accessories-text-editor"},
	"calc":              {"calculator", "accessories-calculator"},
	"mspaint":           {"paint", "kolourpaint"},
	"winword":           {"ms-word"},
	"excel":             {"ms-excel"},
	"powerpnt":          {"ms-powerpoint"},
	"outlook":           {"ms-outlook"},
	"onenote":           {"ms-onenote"},
	"node":              {"nodejs"},
	"vlc":               {"vlc"},
	"qbittorrent":       {"qbittorrent"},
	"epicgameslauncher": {"epic-games", "epicgames"},
}

// vague are first words that say who made a program, not which one it is:
// "Windows PowerShell" is not the icon called "windows".
var vague = map[string]bool{"windows": true, "microsoft": true, "google": true, "apple": true, "adobe": true,
	"system": true, "application": true, "install": true, "developer": true, "global": true, "visual": true, "open": true}

func (p *Pack) exact(k string) (string, bool) {
	if f, ok := p.files[k]; ok {
		return f, true
	}
	for _, n := range windowsNames[k] {
		if f, ok := p.files[Key(n)]; ok {
			return f, true
		}
	}
	f, ok := p.aliases[k]
	return f, ok
}

// Lookup finds the icon for the first name that has one. Every name is tried
// whole first: as a file, as a Windows name of a program the pack calls
// something else, and as another name of an icon. Then the first words of
// each name are tried, the most first ("Epic Games Launcher" finds
// epic-games), at least four letters and not just the maker's name.
func (p *Pack) Lookup(names ...string) (string, bool) {
	if p == nil {
		return "", false
	}
	for _, name := range names {
		if k := Key(name); k != "" {
			if f, ok := p.exact(k); ok {
				return f, true
			}
		}
	}
	for _, name := range names {
		words := strings.Fields(name)
		for n := len(words) - 1; n >= 1; n-- {
			k := Key(strings.Join(words[:n], ""))
			if len(k) < 4 || (n == 1 && vague[k]) {
				continue
			}
			if f, ok := p.exact(k); ok {
				return f, true
			}
		}
	}
	return "", false
}

// Resolve makes a path from the config absolute: relative ones start at the
// repo root.
func Resolve(root, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func exeName(target string) string {
	base := filepath.Base(strings.ReplaceAll(target, `\`, "/"))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Decide chooses where the icon of s comes from.
func Decide(cfg Config, root string, p *Pack, s Shortcut) Choice {
	k := Key(s.Name)
	for _, skip := range cfg.Skip {
		if Key(skip) == k {
			return Choice{From: FromSkip}
		}
	}
	for name, file := range cfg.Overrides {
		if Key(name) == k {
			return Choice{From: FromOverride, File: Resolve(root, file)}
		}
	}
	if f, ok := p.Lookup(s.Name, exeName(s.Target)); ok {
		return Choice{From: FromPack, File: f}
	}
	return Choice{From: FromStyle}
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// Image returns the picture a choice stands for, for the preview: the
// drawn icon, or the pack file (an .ico that holds only bitmaps has none).
func Image(c Choice, srcPNG, style string) (image.Image, error) {
	switch c.From {
	case FromSkip:
		return decodePNG(srcPNG)
	case FromStyle:
		src, err := decodePNG(srcPNG)
		if err != nil {
			return nil, err
		}
		return Render(src, style), nil
	}
	data, err := os.ReadFile(c.File)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(c.File), ".ico") {
		if im, ok := FromICO(data); ok {
			return im, nil
		}
		return nil, fmt.Errorf("%s only has bitmap images, which the preview cannot draw", filepath.Base(c.File))
	}
	return decodePNG(c.File)
}

// ICOFile returns the bytes of the .ico to write for a choice.
func ICOFile(c Choice, srcPNG, style string) ([]byte, error) {
	switch c.From {
	case FromPack, FromOverride:
		data, err := os.ReadFile(c.File)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(filepath.Ext(c.File), ".ico") {
			return data, nil
		}
		im, err := decodePNG(c.File)
		if err != nil {
			return nil, err
		}
		return ICO(im)
	case FromStyle:
		src, err := decodePNG(srcPNG)
		if err != nil {
			return nil, err
		}
		return ICO(Render(src, style))
	}
	return nil, fmt.Errorf("nothing to write for %q", c.From)
}

// SystemIcon is one registry value that holds the icon of a system item.
type SystemIcon struct {
	Item string `json:"item"` // this-pc, recycle-bin...
	Key  string `json:"key"`
	Name string `json:"name"` // "(default)" or a named value
	File string `json:"-"`    // the pack file
}

const (
	classes  = `Registry::HKEY_CURRENT_USER\Software\Classes\CLSID\`
	explorer = `Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Explorer\CLSID\`
)

// SystemIcons lists the system items the pack has an icon for.
func SystemIcons(p *Pack) []SystemIcon {
	if p == nil {
		return nil
	}
	var out []SystemIcon
	simple := []struct{ item, clsid string }{
		{"this-pc", "{20D04FE0-3AEA-1069-A2D8-08002B30309D}"},
		{"user-files", "{59031a47-3f72-44a7-89c5-5595fe6b30ee}"},
		{"network", "{F02C1A0D-BE21-4350-88B0-7367FC96EF3C}"},
	}
	for _, s := range simple {
		if f, ok := p.files[Key(s.item)]; ok {
			out = append(out, SystemIcon{Item: s.item, Key: classes + s.clsid + `\DefaultIcon`, Name: "(default)", File: f})
		}
	}
	if empty, ok := p.files[Key("recycle-bin")]; ok {
		full := empty
		if f, ok := p.files[Key("recycle-bin-full")]; ok {
			full = f
		}
		key := explorer + "{645FF040-5081-101B-9F08-00AA002F954E}" + `\DefaultIcon`
		out = append(out,
			SystemIcon{Item: "recycle-bin", Key: key, Name: "(default)", File: empty},
			SystemIcon{Item: "recycle-bin", Key: key, Name: "empty", File: empty},
			SystemIcon{Item: "recycle-bin", Key: key, Name: "full", File: full})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Item < out[j].Item })
	return out
}
