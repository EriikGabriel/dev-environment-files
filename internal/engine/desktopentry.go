package engine

import (
	"bufio"
	"encoding/base64"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// desktopEntry is the part of a .desktop file the preview needs.
type desktopEntry struct {
	ID        string // file name without .desktop
	Name      string
	Icon      string
	WMClass   string
	NoDisplay bool
}

// dataDirs are the XDG application/icon roots, user first.
func dataDirs(home string) []string {
	dirs := []string{filepath.Join(home, ".local", "share"),
		filepath.Join(home, ".local", "share", "flatpak", "exports", "share"),
		"/var/lib/flatpak/exports/share"}
	xdg := os.Getenv("XDG_DATA_DIRS")
	if xdg == "" {
		xdg = "/usr/local/share:/usr/share"
	}
	return append(dirs, strings.Split(xdg, ":")...)
}

func readDesktop(path string) (desktopEntry, bool) {
	f, err := os.Open(path)
	if err != nil {
		return desktopEntry{}, false
	}
	defer f.Close()
	e := desktopEntry{ID: strings.TrimSuffix(filepath.Base(path), ".desktop")}
	inMain := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inMain = line == "[Desktop Entry]"
			continue
		}
		if !inMain {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "Name":
			if e.Name == "" {
				e.Name = v
			}
		case "Icon":
			e.Icon = v
		case "StartupWMClass":
			e.WMClass = v
		case "NoDisplay":
			e.NoDisplay = strings.EqualFold(v, "true")
		}
	}
	return e, e.Name != ""
}

// findDesktop looks an entry up by id, case-insensitively, then by the
// window class the entry declares (how some docks name their pinned apps).
func findDesktop(home, id string) (desktopEntry, bool) {
	id = strings.TrimSuffix(id, ".desktop")
	var all []string
	for _, d := range dataDirs(home) {
		p := filepath.Join(d, "applications", id+".desktop")
		if e, ok := readDesktop(p); ok {
			return e, true
		}
		matches, _ := filepath.Glob(filepath.Join(d, "applications", "*.desktop"))
		all = append(all, matches...)
	}
	for _, p := range all {
		base := strings.TrimSuffix(filepath.Base(p), ".desktop")
		if strings.EqualFold(base, id) {
			if e, ok := readDesktop(p); ok {
				return e, true
			}
		}
	}
	for _, p := range all {
		if e, ok := readDesktop(p); ok && e.WMClass != "" && strings.EqualFold(e.WMClass, id) {
			return e, true
		}
	}
	return desktopEntry{}, false
}

// listDesktop returns every visible application entry, one per id.
func listDesktop(home string) []desktopEntry {
	seen := map[string]bool{}
	var out []desktopEntry
	for _, d := range dataDirs(home) {
		matches, _ := filepath.Glob(filepath.Join(d, "applications", "*.desktop"))
		for _, p := range matches {
			e, ok := readDesktop(p)
			if !ok || e.NoDisplay || seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

var iconDirs = []string{
	"apps/scalable", "scalable/apps", "apps/512", "apps/256", "apps/128", "apps/96", "apps/64", "apps/48",
	"512x512/apps", "256x256/apps", "128x128/apps", "96x96/apps", "64x64/apps", "48x48/apps",
	"apps/32", "32x32/apps", "apps/22", "apps/16",
}

// themeChain is the theme followed by what it inherits from, ending at hicolor.
func themeChain(home, theme string) []string {
	chain, seen := []string{}, map[string]bool{}
	var walk func(t string, depth int)
	walk = func(t string, depth int) {
		if t == "" || seen[t] || depth > 4 {
			return
		}
		seen[t] = true
		chain = append(chain, t)
		for _, root := range append([]string{filepath.Join(home, ".icons")}, iconRoots(home)...) {
			b, err := os.ReadFile(filepath.Join(root, t, "index.theme"))
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(b), "\n") {
				if rest, ok := strings.CutPrefix(line, "Inherits="); ok {
					for _, p := range strings.Split(strings.TrimSpace(rest), ",") {
						walk(strings.TrimSpace(p), depth+1)
					}
				}
			}
			break
		}
	}
	walk(theme, 0)
	if !seen["hicolor"] {
		chain = append(chain, "hicolor")
	}
	return chain
}

func iconRoots(home string) []string {
	var out []string
	for _, d := range dataDirs(home) {
		out = append(out, filepath.Join(d, "icons"))
	}
	return out
}

// findIcon returns the path of the best file for an icon name.
func findIcon(home, theme, name string) string {
	if name == "" {
		return ""
	}
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name
		}
		return ""
	}
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".png"), ".svg")
	roots := append([]string{filepath.Join(home, ".icons")}, iconRoots(home)...)
	for _, t := range themeChain(home, theme) {
		for _, sub := range iconDirs {
			for _, ext := range []string{".svg", ".png"} {
				for _, r := range roots {
					p := filepath.Join(r, t, sub, name+ext)
					if _, err := os.Stat(p); err == nil {
						return p
					}
				}
			}
		}
	}
	for _, ext := range []string{".png", ".svg"} {
		p := filepath.Join("/usr/share/pixmaps", name+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

const maxIconBytes = 400 << 10

// dataURL reads an image file as a data: URL; "" when it is too big or unknown.
func dataURL(path string, limit int64) string {
	mime, ok := map[string]string{".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg",
		".jpeg": "image/jpeg", ".webp": "image/webp"}[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return ""
	}
	fi, err := os.Stat(path) // follows symlinks, as icon themes use them a lot
	if err != nil || fi.Size() > limit {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
}
