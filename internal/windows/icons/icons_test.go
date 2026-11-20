package icons

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/testutil"
)

func disc() *image.NRGBA {
	im := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			if dx, dy := x-128, y-128; dx*dx+dy*dy < 100*100 {
				im.SetNRGBA(x, y, color.NRGBA{R: 88, G: 101, B: 242, A: 255})
			}
		}
	}
	return im
}

func writePNG(t *testing.T, path string, im image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderDrawsAPlateAroundTheIcon(t *testing.T) {
	for _, style := range []string{StyleLight, StyleDark} {
		out := Render(disc(), style)
		if b := out.Bounds(); b.Dx() != Canvas || b.Dy() != Canvas {
			t.Fatalf("%s: size %v", style, b)
		}
		if a := out.NRGBAAt(0, 0).A; a != 0 {
			t.Errorf("%s: the corner must stay transparent, alpha %d", style, a)
		}
		plate := out.NRGBAAt(40, 128)
		if plate.A != 255 {
			t.Errorf("%s: the plate must be opaque next to the icon, alpha %d", style, plate.A)
		}
		if got := out.NRGBAAt(128, 128); got.B < 200 || got.R > 140 {
			t.Errorf("%s: the original icon must show in the middle: %+v", style, got)
		}
	}
	light, dark := Render(disc(), StyleLight).NRGBAAt(40, 128), Render(disc(), StyleDark).NRGBAAt(40, 128)
	if int(light.R) < int(dark.R)+80 {
		t.Errorf("the light plate must be lighter than the dark one: %+v %+v", light, dark)
	}
}

func TestICOHoldsEverySizeAndReadsBack(t *testing.T) {
	data, err := ICO(Render(disc(), StyleLight))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 6 || data[2] != 1 || int(data[4]) != len(icoSizes) {
		t.Fatalf("not an .ico of %d images: % x", len(icoSizes), data[:6])
	}
	im, ok := FromICO(data)
	if !ok || im.Bounds().Dx() != 256 {
		t.Fatalf("the largest image must be the 256 px one: %v %v", ok, im)
	}
	if _, ok := FromICO([]byte("not an icon")); ok {
		t.Error("garbage is not an icon")
	}
	if u := DataURL(im, 64); !strings.HasPrefix(u, "data:image/png;base64,") {
		t.Errorf("preview must be a PNG data URL: %.30s", u)
	}
}

func TestPackLookupAndDecide(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"discord.png", "epic-games.png", "Steam.ico", "steam.png", "this-pc.png", "recycle-bin.png", "recycle-bin-full.png", "notes.txt"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	p, err := LoadPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Count() != 6 {
		t.Errorf("the pack has 6 icons (notes.txt is not one), got %d", p.Count())
	}
	if f, ok := p.Lookup("Steam"); !ok || filepath.Ext(f) != ".ico" {
		t.Errorf("an .ico wins over a .png: %s", f)
	}
	if f, ok := p.Lookup("Epic Games Launcher"); !ok || !strings.HasSuffix(f, "epic-games.png") {
		t.Errorf("a longer name finds the shorter file: %s %v", f, ok)
	}
	if _, ok := p.Lookup("Git"); ok {
		t.Error("a name shorter than 4 letters must not match loosely")
	}
	if f, ok := p.Lookup("Some Window Title", "discord"); !ok || !strings.HasSuffix(f, "discord.png") {
		t.Errorf("the executable's name is tried next: %s %v", f, ok)
	}

	cfg := Default()
	cfg.Skip = []string{"Steam"}
	cfg.Overrides = map[string]string{"Brave": "mine/brave.png"}
	root := t.TempDir()
	cases := []struct {
		s    Shortcut
		from string
	}{
		{Shortcut{Name: "Steam", Target: `C:\steam.exe`}, FromSkip},
		{Shortcut{Name: "Brave", Target: `C:\brave.exe`}, FromOverride},
		{Shortcut{Name: "Discord", Target: `C:\Update.exe`}, FromPack},
		{Shortcut{Name: "Windhawk", Target: `C:\windhawk.exe`}, FromStyle},
	}
	for _, c := range cases {
		if got := Decide(cfg, root, p, c.s); got.From != c.from {
			t.Errorf("%s: want %s, got %+v", c.s.Name, c.from, got)
		}
	}
	if got := Decide(cfg, root, p, cases[1].s); got.File != filepath.Join(root, "mine", "brave.png") {
		t.Errorf("a relative override starts at the repo: %s", got.File)
	}

	sys := SystemIcons(p)
	names := map[string]int{}
	for _, s := range sys {
		names[s.Item]++
	}
	if names["this-pc"] != 1 || names["recycle-bin"] != 3 || names["network"] != 0 {
		t.Errorf("system icons follow the pack: %v", names)
	}
	if SystemIcons(nil) != nil {
		t.Error("no pack, no system icons")
	}
	if _, err := LoadPack(filepath.Join(dir, "missing")); err == nil {
		t.Error("a missing pack folder must be reported")
	}
}

func TestWriteICOsAndPlan(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "a.png")
	writePNG(t, src, disc())
	pack := t.TempDir()
	writePNG(t, filepath.Join(pack, "steam.png"), disc())
	p, _ := LoadPack(pack)
	list := []Shortcut{
		{ID: "aaaa", Name: "Windhawk", Path: `C:\x\Windhawk.lnk`, Target: `C:\w.exe`, Src: src},
		{ID: "bbbb", Name: "Steam", Path: `C:\x\Steam.lnk`, Target: `C:\s.exe`, Src: src},
		{ID: "cccc", Name: "Zed", Path: `C:\x\Zed.lnk`, Target: `C:\z.exe`, Src: src},
	}
	cfg := Default()
	cfg.Skip = []string{"Zed"}
	entries := Plan(cfg, work, p, list)
	if len(entries) != 3 || entries[0].From != FromStyle || entries[1].From != FromPack || entries[2].From != FromSkip {
		t.Fatalf("decisions: %+v", entries)
	}
	if entries[0].After == "" || entries[0].Before == "" || entries[2].After != "" {
		t.Error("a drawn icon has a before and an after, a skipped one only a before")
	}
	if entries[0].After == entries[0].Before {
		t.Error("the drawn icon must differ from the original")
	}

	dir := filepath.Join(work, "ico")
	job, problems, err := WriteICOs(dir, cfg, work, p, list, filepath.Join(work, "manifest.json"))
	if err != nil || len(problems) != 0 {
		t.Fatalf("%v %v", err, problems)
	}
	if len(job.Entries) != 2 {
		t.Fatalf("the skipped shortcut gets no icon: %+v", job.Entries)
	}
	for _, e := range job.Entries {
		if data, err := os.ReadFile(e.Ico); err != nil || len(data) < 6 || data[2] != 1 {
			t.Errorf("%s must be an .ico: %v", e.Ico, err)
		}
	}
}

func TestScriptsAreSafe(t *testing.T) {
	scan := ScanScript(`C:\w'x`, `C:\m.json`, `C:\ico`, []string{ScopeDesktop, ScopeTaskbar})
	for _, want := range []string{"SHCreateItemFromParsingName", `'C:\w''x'`, "AddRoot 'desktop'", "AddRoot 'taskbar'",
		"GetExtension($t) -ne '.exe'", "StartsWith($icoDir", "scan.json"} {
		if !strings.Contains(scan, want) {
			t.Errorf("scan script misses %s", want)
		}
	}
	if strings.Contains(scan, "AddRoot 'start-menu'") {
		t.Error("a scope that is off must not be scanned")
	}
	apply := ApplyScript(`C:\job.json`)
	for _, want := range []string{"Test-Path -LiteralPath $e.path", "$orig = $old[$key].orig", "$l.Save()", "ie4uinit", "exit 0"} {
		if !strings.Contains(apply, want) {
			t.Errorf("apply script misses %s", want)
		}
	}
	restore := RestoreScript(`C:\m.json`, `C:\i'co`)
	for _, want := range []string{"$icoDir = 'C:\\i''co'", "$l.IconLocation = $orig", "Test-Path -LiteralPath $e.path", "Remove-Item -LiteralPath $manifest",
		"Remove-Item -LiteralPath $icoDir", `-replace ',-?\d+$'`, "iconcache*.db", "Quick Launch", "imageres.dll,-55"} {
		if !strings.Contains(restore, want) {
			t.Errorf("restore script misses %s", want)
		}
	}
}

func TestConfigRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "icons.yaml")
	if cfg, exists, err := Load(path); err != nil || exists || cfg.Style != StyleLight || len(cfg.Scopes) != 3 {
		t.Fatalf("a missing file is the default: %+v %v %v", cfg, exists, err)
	}
	want := Default()
	want.Enabled, want.Style, want.Pack = true, StyleDark, "icons/mine"
	want.Scopes = []string{ScopeDesktop}
	want.Skip = []string{"Steam"}
	want.Overrides = map[string]string{"Brave": "x.png"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Load(path)
	if err != nil || !exists || !got.Enabled || got.Style != StyleDark || got.Pack != "icons/mine" || len(got.Scopes) != 1 || got.Overrides["Brave"] != "x.png" {
		t.Fatalf("round trip: %+v %v %v", got, exists, err)
	}
	bad := want
	bad.Style = "neon"
	if Save(path, bad) == nil {
		t.Error("an unknown style must be refused")
	}
	bad = want
	bad.Scopes = []string{"everything"}
	if Save(path, bad) == nil {
		t.Error("an unknown scope must be refused")
	}
}

func TestPackAliasesAndWindowsNames(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"git.png", "terminal.png", "steam.png", "Steam++.png", "text-editor.png", "visual-studio-code.png", "wine.png", "spotify-client.png"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	os.WriteFile(filepath.Join(dir, "aliases.json"), []byte(`{"code": "text-editor", "windows": "wine", "spotify": "spotify-client", "steam": "wine", "ghost": "missing"}`), 0o644)
	p, err := LoadPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Count() != 7 {
		t.Errorf("Steam++ and steam are one icon: %d", p.Count())
	}
	for _, c := range []struct {
		names []string
		want  string
	}{
		{[]string{"Steam", "steam"}, "steam.png"},                      // the shorter file name, and a file before an alias
		{[]string{"Spotify"}, "spotify-client.png"},                    // another name of an icon
		{[]string{"Git Bash", "git-bash"}, "git.png"},                  // a Windows name
		{[]string{"Code", "Code"}, "visual-studio-code.png"},           // a Windows name before an alias
		{[]string{"Windows PowerShell", "powershell"}, "terminal.png"}, // not "windows"
		{[]string{"Windows Thing", "thing"}, ""},
		{[]string{"ghost"}, ""},
	} {
		f, ok := p.Lookup(c.names...)
		if ok != (c.want != "") || (ok && filepath.Base(f) != c.want) {
			t.Errorf("Lookup(%q) = %q %v, want %q", c.names, filepath.Base(f), ok, c.want)
		}
	}
	if got := p.Search("code"); len(got) != 2 || got[0].Name != "text-editor" {
		t.Errorf("Search finds names and other names: %+v", got)
	}
	if got := p.Search(""); len(got) != 7 {
		t.Errorf("an empty query lists every icon: %d", len(got))
	}
	os.WriteFile(filepath.Join(dir, "aliases.json"), []byte(`{`), 0o644)
	if _, err := LoadPack(dir); err == nil {
		t.Error("a broken aliases.json must be reported")
	}
}

func TestBuiltinPackIsInTheRepo(t *testing.T) {
	p, err := LoadPack(filepath.Join(testutil.RepoRoot(t), filepath.FromSlash(BuiltinPack)))
	if err != nil {
		t.Fatal(err)
	}
	if p.Count() < 1000 || len(SystemIcons(p)) < 5 {
		t.Errorf("the pack looks incomplete: %d icons, %d system values", p.Count(), len(SystemIcons(p)))
	}
	for _, n := range []string{"Discord", "Google Chrome", "Visual Studio Code", "Steam", "Spotify"} {
		if _, ok := p.Lookup(n); !ok {
			t.Errorf("no icon for %s", n)
		}
	}
}

func TestIconFilesAreNamedAfterTheirContent(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "a.png")
	writePNG(t, src, disc())
	list := []Shortcut{{ID: "aaaa", Name: "App", Path: `C:\x\App.lnk`, Target: `C:\a.exe`, Src: src}}
	cfg := Default()
	light, _, err := WriteICOs(filepath.Join(work, "ico"), cfg, work, nil, list, "m.json")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Style = StyleDark
	dark, _, err := WriteICOs(filepath.Join(work, "ico"), cfg, work, nil, list, "m.json")
	if err != nil {
		t.Fatal(err)
	}
	if light.Entries[0].Ico == dark.Entries[0].Ico {
		t.Error("an icon that changed must get another path: Windows caches icons by path")
	}
	scan := ScanScript(`C:\w`, `C:\m.json`, `C:\ico`, []string{ScopeTaskbar})
	apply := ApplyScript(`C:\job.json`)
	for _, want := range []string{`( \(\d+\))?$`, `-replace ',-?\d+$'`, "$byIco["} {
		if !strings.Contains(scan, want) {
			t.Errorf("scan script misses %q", want)
		}
	}
	for _, want := range []string{`-replace ',-?\d+$'`, "$byIco[", "Remove-Item -Force"} {
		if !strings.Contains(apply, want) {
			t.Errorf("apply script misses %q", want)
		}
	}
}

// The shortcut names Windows gives these apps find the macOS icons picked
// for them (windows/icons/macosicons, copied into the pack).
func TestPackHasTheHandPickedIcons(t *testing.T) {
	p, err := LoadPack(filepath.Join(testutil.RepoRoot(t), filepath.FromSlash(BuiltinPack)))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"Hydra": "hydra.png", "Epic Games Launcher": "epic-games.png", "SignalRgb": "signalrgb.png", "Proton VPN": "protonvpn-gui.png", "Playnite": "playnite.png", "Dia": "dia.png"} {
		if f, ok := p.Lookup(name); !ok || filepath.Base(f) != want {
			t.Errorf("%s: got %q, want %s", name, f, want)
		}
	}
}

// A running Steam shows the icon of the shortcut Windows keeps in
// ImplicitAppShortcuts (its windows carry the app id Valve.Steam.Client), so
// the taskbar scope goes through that folder too.
func TestTaskbarScopeCoversImplicitAppShortcuts(t *testing.T) {
	scan := ScanScript(`C:\w`, `C:\m.json`, `C:\ico`, []string{ScopeTaskbar})
	if !strings.Contains(scan, `User Pinned\ImplicitAppShortcuts') $true`) {
		t.Errorf("the taskbar scope must scan ImplicitAppShortcuts, recursively:\n%s", scan)
	}
}
