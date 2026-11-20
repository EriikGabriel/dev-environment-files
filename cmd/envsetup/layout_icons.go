package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/icons"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/layout"
)

// exeNames are the programs behind the built-in apps, whose shortcuts point
// at a shell folder or an app id: the icons step finds their icon by them.
var exeNames = map[string]string{"explorer": "explorer", "edge": "msedge"}

// layoutIcons gives each app of the preview the icon the icons step gives
// its shortcut: the override of config/icons.yaml, else the pack's by the
// shortcut's name or program. An app with neither (drawn on a plate from the
// program's own icon) is left out; the preview draws it from the last scan.
// The default browser is the one this machine would pin.
func layoutIcons(root string, items []layout.Item) map[string]string {
	out := map[string]string{}
	cfg, _, err := icons.Load(icons.Path(root))
	if err != nil || !cfg.Enabled {
		return out
	}
	pack, err := icons.LoadPack(icons.Resolve(root, cfg.Pack))
	if err != nil || pack == nil {
		return out
	}
	browser := ""
	for _, it := range items {
		names := append([]string{}, it.Match...)
		if it.Browser {
			if browser == "" {
				browser = machineBrowser(pack)
			}
			names = []string{browser}
		}
		names = append(names, it.Name)
		if exe, ok := exeNames[it.ID]; ok {
			names = append(names, exe)
		}
		if file := iconFile(cfg, root, pack, names); file != "" {
			if im, err := icons.Image(icons.Choice{From: icons.FromPack, File: file}, "", cfg.Style); err == nil {
				out[it.ID] = icons.DataURL(im, icons.PreviewSide)
			}
		}
	}
	return out
}

func iconFile(cfg icons.Config, root string, pack *icons.Pack, names []string) string {
	for _, n := range names {
		for name, file := range cfg.Overrides {
			if icons.Key(name) == icons.Key(n) {
				return icons.Resolve(root, file)
			}
		}
	}
	for _, n := range names {
		if f, ok := pack.Lookup(n); ok {
			return f
		}
	}
	return ""
}

// machineBrowser is the browser the taskbar step pins: the default one (who
// opens https links), else the first of the list with a Start menu
// shortcut, else the first the pack has an icon for.
func machineBrowser(pack *icons.Pack) string {
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\https\UserChoice`, "/v", "ProgId").Output(); err == nil {
			prog := strings.ToLower(string(out))
			for prefix, name := range map[string]string{"bravehtml": "Brave", "chromehtml": "Google Chrome", "firefoxurl": "Firefox", "operagx": "Opera GX", "opera gx": "Opera GX", "operastable": "Opera", "vivaldihtm": "Vivaldi"} {
				if strings.Contains(prog, "progid    reg_sz    "+prefix) {
					return name
				}
			}
		}
		links := startMenuLinks()
		for _, n := range layout.BrowserNames {
			if links[strings.ToLower(n)] {
				return n
			}
		}
	}
	for _, n := range layout.BrowserNames {
		if _, ok := pack.Lookup(n); ok {
			return n
		}
	}
	return layout.BrowserNames[0]
}

// startMenuLinks are the names of the Start menu shortcuts, lower case.
func startMenuLinks() map[string]bool {
	out := map[string]bool{}
	for _, dir := range []string{os.Getenv("ProgramData"), os.Getenv("APPDATA")} {
		if dir == "" {
			continue
		}
		_ = filepath.WalkDir(filepath.Join(dir, "Microsoft", "Windows", "Start Menu", "Programs"), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".lnk") {
				out[strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))] = true
			}
			return nil
		})
	}
	return out
}
