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
