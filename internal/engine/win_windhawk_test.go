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
