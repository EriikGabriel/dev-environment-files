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
	admin := AdminScript(c)
	for _, want := range []string{"ConfigureStartPins", `{"pinnedList":[]}`, "exit 0"} {
		if !strings.Contains(admin, want) {
			t.Errorf("admin script misses %s", want)
		}
	}
	fin := FinishScript(c)
	for _, want := range []string{
		"'HideIcons' -Value 1", "start*.bin",
		"PinId 'Explorador de Arquivos' 'Microsoft.Windows.Explorer' @('File Explorer', 'Explorador de Arquivos')",
		"PinLink 'Brave' @('Brave')",
		"none of the chosen apps was found",
		"Stop-Process -Name 'explorer'",
	} {
		if !strings.Contains(fin, want) {
			t.Errorf("finish script misses %s", want)
		}
	}
	if strings.Index(fin, "PinId 'Explorador") > strings.Index(fin, "PinLink 'Brave'") {
		t.Error("the order of the taskbar must be kept")
	}
	// the Start menu is reset before the pin step restarts the shell, once
	if strings.Index(fin, "start*.bin") > strings.Index(fin, "WritePins") {
		t.Error("the Start menu state must be cleared before Explorer restarts")
	}

	// nothing to pin, nothing hidden: the Start policy goes, the icons come back
	off := Config{}
	if a := AdminScript(off); strings.Contains(a, "New-ItemProperty") || strings.Contains(a, "Set-ItemProperty") || !strings.Contains(a, "Remove-ItemProperty -LiteralPath $st -Name 'ConfigureStartPins'") {
		t.Errorf("an empty layout must undo the policies:\n%s", a)
	}
	if f := FinishScript(off); !strings.Contains(f, "'HideIcons' -Value 0") || strings.Contains(f, "Taskband") || strings.Contains(f, "User Pinned") {
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
	fin := FinishScript(c)
	for _, want := range []string{
		"PinBrowser 'Navegador padrão' @('Dia', 'Brave', 'Opera GX', 'Google Chrome', 'Firefox', 'Vivaldi', 'Opera')",
		`UrlAssociations\https\UserChoice`,
		"PinLink 'RetroArch' @('RetroArch')",
		`[regex]::Escape($n) + '($|[^\p{L}\p{N}])'`, `TheBrowserCompany\.Dia_`,
		"already pinned, left out", "a store app with no Start menu shortcut",
	} {
		if !strings.Contains(fin, want) {
			t.Errorf("finish script misses %s", want)
		}
	}
	order := []string{"PinBrowser 'Navegador", "PinLink 'Playnite'", "PinLink 'RetroArch'", "PinLink 'Steam'"}
	for i := 1; i < len(order); i++ {
		if strings.Index(fin, order[i-1]) > strings.Index(fin, order[i]) {
			t.Errorf("%s must come before %s", order[i-1], order[i])
		}
	}
	if !strings.Contains(fin, "StuckRects3") || !strings.Contains(fin, "$v[8] = 3") {
		t.Errorf("auto-hide on must set byte 8 to 3:\n%s", fin)
	}
	if f := FinishScript(Config{}); !strings.Contains(f, "$v[8] = 2") {
		t.Errorf("auto-hide off must set byte 8 to 2:\n%s", f)
	}
}

// The pins are the user's own, written the way Explorer keeps them, so no
// policy reapplies (and drops) them at sign-in. Windows 11 read the layout
// file of the "Start layout" policy at every sign-in and dropped most pins,
// leaving copies like "Brave (2).lnk"; what earlier versions set up for it
// (the policy, the file, the sign-in guard) is taken out.
func TestPinsAreTheUsersOwn(t *testing.T) {
	c := Config{Taskbar: []string{"explorer", "browser", "playnite", "retroarch", "hydra"}}
	fin := FinishScript(c)
	for _, want := range []string{
		"PinLink 'Hydra Launcher' @('Hydra Launcher', 'Hydra')",
		// the pins replace what is there, duplicates included
		`User Pinned\TaskBar`, "Get-ChildItem -LiteralPath $dir -Filter *.lnk -Force | Remove-Item -Force",
		// Explorer's own format: the User Pinned root, the three blocks of the item
		"1f3427c8-5c10-4210-aa03-2ee45287d668", "Block 0x1d $p.Id $back", "Block 0x1e 'UserPinned' $back", "0x2b, 0x00, 0xef, 0xbe",
		"-Name 'Favorites'", "-Name 'FavoritesResolve'", "$fav.Add(0xff)",
		// every pin gets an app id, or Explorer drops it and the ones after it
		"Get-StartApps", "System.AppUserModel.ID", "{7C5A40EF-A0FB-4BFC-874A-C0F2E0B9FA8E}",
		// the localized name of a pin (File Explorer) goes with it
		"[LocalizedFileNames]",
		// checked against the taskbar's own list after the restart
		"0DD79AE2-D156-45D4-9EEB-3B549769E940", "not on the taskbar after Explorer restarted",
	} {
		if !strings.Contains(fin, want) {
			t.Errorf("finish script misses %q", want)
		}
	}
	for _, s := range []string{fin, FinishScript(Config{})} {
		for _, want := range []string{"-Name '" + legacyRunValue + "'", "'" + legacyGuard + "'", "'" + legacyXML + "'", "taskbar-guard.txt"} {
			if !strings.Contains(s, want) {
				t.Errorf("finish must take out what earlier versions left (%s)", want)
			}
		}
	}
	for _, s := range []string{fin, AdminScript(c)} {
		if strings.Contains(s, "LayoutModificationTemplate") || strings.Contains(s, "New-ItemProperty -LiteralPath $ex -Name 'StartLayoutFile'") {
			t.Error("the taskbar must not go through the Start layout policy any more")
		}
	}
	// the old policy goes only when it is the one envsetup set
	if a := AdminScript(c); !strings.Contains(a, `-like ('*\' + 'envsetup\TaskbarLayout.xml')`) || !strings.Contains(a, "Remove-ItemProperty -LiteralPath $ex -Name 'StartLayoutFile'") {
		t.Errorf("admin must remove the old Start layout policy:\n%s", a)
	}
	if strings.Contains(fin, "`") {
		t.Error("the script must not need PowerShell's escape character")
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
