package engine

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/testutil"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/windhawk"
)

func TestWindhawkImportRunsElevated(t *testing.T) {
	root := t.TempDir()
	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Runner: fr, WindhawkCLI: `C:\WH\windhawk-cli.exe`}
	if err := env.ImportWindhawk(context.Background(), io.Discard); err == nil || !strings.Contains(err.Error(), "backup.json") {
		t.Fatalf("without an export: %v", err)
	}
	if len(fr.ran) != 0 {
		t.Fatalf("nothing should run without an export: %v", fr.ran)
	}

	raw, err := os.ReadFile(windhawk.BackupPath(testutil.RepoRoot(t)))
	must(t, err)
	must(t, os.MkdirAll(filepath.Dir(windhawk.BackupPath(root)), 0o755))
	must(t, os.WriteFile(windhawk.BackupPath(root), raw, 0o644))
	must(t, env.ImportWindhawk(context.Background(), io.Discard))
	ran := ranLines(fr)
	if !strings.Contains(ran, "-Verb RunAs") || !strings.Contains(ran, "windhawk.ps1") {
		t.Errorf("the import must run elevated:\n%s", ran)
	}

	// the step is pending while the machine cannot be read
	p, err := BuildPlan(realConfig(t), "gaming-windows", env, Options{Apps: []string{"windhawk-mods"}, NoFiles: true})
	must(t, err)
	if l := p.Describe(context.Background()); len(l) != 1 || !l[0].Pending || !strings.Contains(l[0].Detail, "Windhawk mods") {
		t.Errorf("describe = %+v", l)
	}
}

// winget only has Windhawk 1.x, which has no windhawk-cli.exe (the mods are
// installed with it), and Windhawk 2 is a GitHub pre-release: the item must
// come from the script that takes the newest release of version 2 or later.
func TestWindhawkComesFromGitHubNotWinget(t *testing.T) {
	root := testutil.RepoRoot(t)
	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Runner: fr}
	p, err := BuildPlan(realConfig(t), "gaming-windows", env, Options{Apps: []string{"windhawk"}, NoFiles: true})
	must(t, err)
	p.Execute(context.Background(), io.Discard, func(Event) {})
	ran := ranLines(fr)
	if strings.Contains(ran, "winget") || strings.Contains(ran, "RamenSoftware.Windhawk") {
		t.Errorf("Windhawk must not come from winget:\n%s", ran)
	}
	if !strings.Contains(ran, `windows\install-windhawk.ps1`) {
		t.Errorf("Windhawk must be installed by its script:\n%s", ran)
	}

	raw, err := os.ReadFile(filepath.Join(root, "windows", "install-windhawk.ps1"))
	must(t, err)
	for _, want := range []string{"ramensoftware/windhawk", "windhawk_setup.exe", "-ge 2", "Get-FileHash"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("install-windhawk.ps1 lost %q", want)
		}
	}
	// the stable 1.x line must not be the only thing it can pick
	if strings.Contains(string(raw), "-not $_.prerelease") || strings.Contains(string(raw), "releases/latest") {
		t.Error("install-windhawk.ps1 must take pre-releases: /releases/latest skips them")
	}

	// and the step that needs the CLI says what to do when it is missing
	env.WindhawkCLI = ""
	t.Setenv("ProgramFiles", t.TempDir())
	t.Setenv("ProgramW6432", t.TempDir())
	if _, err := env.windhawkCLI(); err == nil || !strings.Contains(err.Error(), "install-windhawk.ps1") {
		t.Errorf("missing CLI: %v", err)
	}
}

func TestTaskbarDockUsesTheIslandMod(t *testing.T) {
	cli := `C:\WH\windhawk-cli.exe`
	fr := &fakeRunner{answers: map[string]string{
		"taskbarMode": `{"success":true,"data":{"key":"taskbarMode","value":"windowsTransparent"}}`,
	}}
	env := &Env{Info: win, Root: t.TempDir(), Runner: fr, WindhawkCLI: cli, StateDir: t.TempDir()}
	ctx := context.Background()
	if m, err := env.DockMode(ctx); err != nil || m != "windowsTransparent" {
		t.Fatalf("mode = %q %v", m, err)
	}
	must(t, env.TaskbarDock(ctx, io.Discard, false))
	if raw, _ := os.ReadFile(env.dockMemory()); string(raw) != "windowsTransparent" {
		t.Errorf("the previous mode must be kept, got %q", raw)
	}
	ran := ranLines(fr)
	if !strings.Contains(ran, "-Verb RunAs") || strings.Contains(ran, "taskbar-styler") {
		t.Errorf("one elevated script, no Taskbar Styler:\n%s", ran)
	}

	// restore puts the kept mode back and forgets it
	fr.answers["taskbarMode"] = `{"success":true,"data":{"key":"taskbarMode","value":"dock"}}`
	must(t, env.TaskbarDock(ctx, io.Discard, true))
	if _, err := os.Stat(env.dockMemory()); err == nil {
		t.Error("the kept mode must be forgotten after restoring")
	}

	// the mod missing: nothing to read, and turning the dock on installs it
	fr.answers["taskbarMode"] = `{"success":false,"error":{"code":"MOD_NOT_INSTALLED","message":"Mod not installed: mac-island-dock"}}`
	if m, err := env.DockMode(ctx); err != nil || m != "" {
		t.Errorf("missing mod: %q %v", m, err)
	}
}
