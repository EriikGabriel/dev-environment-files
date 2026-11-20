package layout

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/install"
)

func ids(items []Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func find(items []Item, id string) *Item {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func TestCandidatesMergeTheSources(t *testing.T) {
	items := Candidates(Inputs{
		Ninite:   []string{"brave", "discord", "steam", "epic", "git", "spotify", "powertoys", "adoptjavax21", "node.js24"},
		Apps:     []string{"steam", "nvidia-app", "brave", "vcredist"},
		Installs: []install.App{{ID: "hydra", Name: "Hydra", Kind: "github"}, {ID: "vc-redist", Name: "VC", Kind: "runtime"}, {ID: "ninite", Name: "Ninite", Kind: "ninite"}},
	})
	if it := find(items, "explorer"); it == nil || it.Sources[0] != SourceWindows {
		t.Errorf("Windows' own apps are always on offer: %v", ids(items))
	}
	steam := find(items, "steam")
	if steam == nil || strings.Join(steam.Sources, ",") != "ninite,catalog" {
		t.Errorf("steam comes from Ninite and the profile, once: %+v", steam)
	}
	if find(items, "nvidia-app") == nil || find(items, "brave") == nil {
		t.Errorf("NVIDIA app and Brave must be offered: %v", ids(items))
	}
	if h := find(items, "hydra"); h == nil || h.Sources[0] != SourceUnattend {
		t.Errorf("Hydra comes from the answer file: %+v", h)
	}
	for _, id := range []string{"ninite:adoptjavax21", "ninite:node.js24"} {
		if it := find(items, id); it == nil || it.Pinnable {
			t.Errorf("%s has no window to pin: %+v", id, it)
		}
	}
	if find(items, "vcredist") != nil || find(items, "vc-redist") != nil || find(items, "ninite") != nil {
		t.Errorf("runtimes and the installer itself are not apps to pin: %v", ids(items))
	}
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.ID] {
			t.Errorf("%s listed twice", it.ID)
		}
		seen[it.ID] = true
	}
}

func TestFindKnowsSavedIDs(t *testing.T) {
	for _, id := range []string{"brave", "explorer", "ninite:notepadplusplus", "github:hydra"} {
		if _, ok := Find(id); !ok {
			t.Errorf("%s must resolve", id)
		}
	}
	if _, ok := Find("ninite:nope"); ok {
		t.Error("an id that is not in the Ninite catalogue must not resolve")
	}
}

func TestConfigRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "layout.yaml")
	if _, exists, err := Load(path); err != nil || exists {
		t.Fatalf("a missing file is an empty layout: %v %v", exists, err)
	}
	want := Config{Taskbar: []string{"explorer", "brave", "ninite:7zip"}, ClearStart: true, HideDesktopIcons: true}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Load(path)
	if err != nil || !exists || strings.Join(got.Taskbar, " ") != strings.Join(want.Taskbar, " ") || !got.ClearStart || !got.HideDesktopIcons {
		t.Fatalf("round trip: %+v %v %v", got, exists, err)
	}
	if err := Save(path, Config{Taskbar: []string{"a", "a"}}); err == nil {
		t.Error("a duplicate pin must be refused")
	}
	if err := Save(path, Config{Taskbar: []string{"x'; calc; '"}}); err == nil {
		t.Error("an id that is not plain must be refused")
	}
}

func TestScripts(t *testing.T) {
	c := Config{Taskbar: []string{"explorer", "brave", "ninite:node.js24", "unknown"}, ClearStart: true, HideDesktopIcons: true}
	if pinned := Pinned(c.Taskbar); len(pinned) != 2 {
		t.Fatalf("only explorer and brave can be pinned: %v", ids(pinned))
	}
	prep := PrepareScript(c)
	for _, want := range []string{
		"PinId 'Explorador de Arquivos' 'Microsoft.Windows.Explorer'",
		"PinLink 'Brave' @('Brave')",
		"PinListPlacement=\"Replace\"",
		"none of the chosen apps was found",
	} {
		if !strings.Contains(prep, want) {
			t.Errorf("prepare script misses %s", want)
		}
	}
	if strings.Index(prep, "PinId") > strings.Index(prep, "PinLink 'Brave'") {
		t.Error("the order of the taskbar must be kept")
	}
	admin := AdminScript(c)
	for _, want := range []string{"StartLayoutFile", "LockedStartLayout", "ConfigureStartPins", `{"pinnedList":[]}`, "exit 0"} {
		if !strings.Contains(admin, want) {
			t.Errorf("admin script misses %s", want)
		}
	}
	fin := FinishScript(c)
	for _, want := range []string{"'HideIcons' -Value 1", "Explorer\\Taskband", "start*.bin", "Stop-Process -Name 'explorer'"} {
		if !strings.Contains(fin, want) {
			t.Errorf("finish script misses %s", want)
		}
	}

	// nothing to pin, nothing hidden: the policies are removed and the icons shown
	off := Config{}
	if a := AdminScript(off); strings.Contains(a, "New-ItemProperty") || !strings.Contains(a, "Remove-ItemProperty -LiteralPath $ex -Name 'StartLayoutFile'") || !strings.Contains(a, "Remove-ItemProperty -LiteralPath $st -Name 'ConfigureStartPins'") {
		t.Errorf("an empty layout must undo the policies:\n%s", a)
	}
	if f := FinishScript(off); !strings.Contains(f, "'HideIcons' -Value 0") || strings.Contains(f, "Taskband") {
		t.Errorf("icons must come back and the taskbar must be left alone:\n%s", f)
	}
}

func TestBrowserRetroArchAndAutoHide(t *testing.T) {
	items := Candidates(Inputs{Apps: []string{"retroarch", "playnite"}})
	if it := find(items, "browser"); it == nil || !it.Pinnable || it.Sources[0] != SourceWindows {
		t.Errorf("the default browser is always on offer: %v", ids(items))
	}
	if find(items, "retroarch") == nil {
		t.Errorf("RetroArch must be offered when the profile has it: %v", ids(items))
	}

	c := Config{Taskbar: []string{"explorer", "browser", "playnite", "retroarch", "steam"}, TaskbarAutoHide: true}
	prep := PrepareScript(c)
	for _, want := range []string{
		"PinBrowser 'Navegador padrão' @('Brave', 'Opera GX', 'Google Chrome', 'Firefox', 'Vivaldi', 'Opera')",
		`UrlAssociations\https\UserChoice`,
		"PinLink 'RetroArch' @('RetroArch')",
		"already pinned, left out",
	} {
		if !strings.Contains(prep, want) {
			t.Errorf("prepare script misses %s", want)
		}
	}
	order := []string{"PinBrowser 'Navegador", "PinLink 'Playnite'", "PinLink 'RetroArch'", "PinLink 'Steam'"}
	for i := 1; i < len(order); i++ {
		if strings.Index(prep, order[i-1]) > strings.Index(prep, order[i]) {
			t.Errorf("%s must come before %s", order[i-1], order[i])
		}
	}
	if f := FinishScript(c); !strings.Contains(f, "StuckRects3") || !strings.Contains(f, "$v[8] = 3") {
		t.Errorf("auto-hide on must set byte 8 to 3:\n%s", f)
	}
	if f := FinishScript(Config{}); !strings.Contains(f, "$v[8] = 2") {
		t.Errorf("auto-hide off must set byte 8 to 2:\n%s", f)
	}
}

// Pins that Windows drops at sign-in: apps with an id of their own go by it,
// the others by a shortcut written with the variables the layout expects, and
// a guard started at sign-in puts the layout back when the taskbar comes up
// almost empty.
func TestPinsSurviveTheSignIn(t *testing.T) {
	c := Config{Taskbar: []string{"explorer", "browser", "playnite", "retroarch", "hydra"}}
	prep := PrepareScript(c)
	for _, want := range []string{
		"Get-StartApps",
		"[IO.Path]::GetFileName($p) + '|' + $aumid",
		"'%APPDATA%'", "'%ALLUSERSPROFILE%'",
		"PinLink 'Hydra Launcher' @('Hydra Launcher', 'Hydra')",
		"'" + PinsFile + "'", "'" + GuardFile + "'",
		"$guard = @'\n# envsetup: puts the taskbar pins back",
		"\n'@\n",
	} {
		if !strings.Contains(prep, want) {
			t.Errorf("prepare script misses %q", want)
		}
	}
	// on a fresh machine the apps given by app id were gone after signing in
	// again, the shortcuts written with %APPDATA% stayed: only Explorer, which
	// has no shortcut, goes by id
	if i := strings.Index(prep, "function PinXml"); i < 0 || strings.Contains(prep[i:strings.Index(prep, "function PinLink")], "DesktopApplicationID") {
		t.Error("the apps must be pinned by their shortcut")
	}
	if strings.Contains(GuardScript, "\n'@") || strings.Contains(GuardScript, "`") {
		t.Error("the guard must fit in a single-quoted here-string")
	}
	for _, want := range []string{"Favorites", "User Pinned", "[regex]::Escape($f[2])", "GetString($fav, 1, $fav.Length - 1)", "$text.IndexOf($target", "LastWriteTime = Get-Date", "Stop-Process -Name explorer", "$miss.Count * 2 -le $want.Count", "not trying again for this layout"} {
		if !strings.Contains(GuardScript, want) {
			t.Errorf("guard misses %q", want)
		}
	}
	fin := FinishScript(c)
	if !strings.Contains(fin, "-Name '"+RunValue+"'") || !strings.Contains(fin, GuardFile) || !strings.Contains(fin, "taskbar-guard.txt") {
		t.Errorf("finish must start the guard at sign-in and give it a fresh start:\n%s", fin)
	}
	if off := FinishScript(Config{}); !strings.Contains(off, "Remove-ItemProperty -LiteralPath $run -Name '"+RunValue+"'") {
		t.Errorf("no pins, no guard:\n%s", off)
	}
}

func TestRepoLayoutPinsTheLaunchersAfterTheBrowser(t *testing.T) {
	cfg, exists, err := Load(filepath.Join("..", "..", "..", "config", "layout.yaml"))
	if err != nil || !exists {
		t.Fatalf("config/layout.yaml: %v %v", exists, err)
	}
	got := strings.Join(cfg.Taskbar, ",")
	if !strings.HasSuffix(got, "browser,playnite,retroarch,hydra") || strings.Contains(got, "epic") || strings.Contains(got, "steam") {
		t.Errorf("the default dock ends with the browser, then Playnite, RetroArch and Hydra Launcher, without Epic or Steam: %s", got)
	}
	if !cfg.TaskbarAutoHide {
		t.Error("the taskbar hides by default")
	}
}
