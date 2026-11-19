package engine

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/playnite"
)

func TestPlayniteStepRestoresTheSnapshot(t *testing.T) {
	root, dir, store := t.TempDir(), filepath.Join(t.TempDir(), "Playnite"), t.TempDir()
	must(t, playnite.Save(playnite.Path(root), playnite.Config{Path: dir, Store: store, StartMenu: true, Exclude: []string{"cache"}}))

	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Home: t.TempDir(), Runner: fr, StateDir: t.TempDir()}
	p, err := BuildPlan(realConfig(t), "gaming-windows", env, Options{Apps: []string{"playnite"}, NoFiles: true})
	must(t, err)
	if len(p.Steps) != 1 {
		t.Fatalf("want the playnite step, got %d (skipped %+v)", len(p.Steps), p.Skipped)
	}
	if l := p.Describe(context.Background()); !l[0].Pending || !strings.Contains(l[0].Detail, "Playnite back from the store") {
		t.Fatalf("without the folder it is pending: %+v", l)
	}

	// no snapshot in the store yet: a clear failure, nothing copied
	if res := p.Execute(context.Background(), io.Discard, func(Event) {}); res.Failed != 1 {
		t.Fatalf("restoring without a snapshot must fail: %+v", res)
	}
	if strings.Contains(ranLines(fr), "robocopy") {
		t.Fatalf("nothing should be copied without a snapshot:\n%s", ranLines(fr))
	}

	must(t, os.MkdirAll(filepath.Join(store, playnite.SnapshotDir), 0o755))
	must(t, os.WriteFile(filepath.Join(store, playnite.SnapshotDir, playnite.Exe), []byte("MZ"), 0o644))
	must(t, playnite.WriteManifest(store, playnite.Manifest{Source: dir}))
	if res := p.Execute(context.Background(), io.Discard, func(Event) {}); res.Failed != 0 {
		t.Fatalf("restore failed: %+v", res)
	}
	ran := ranLines(fr)
	if !strings.Contains(ran, "& robocopy '"+filepath.Join(store, playnite.SnapshotDir)+"' '"+dir+"' '/E'") {
		t.Errorf("the snapshot must be copied to the folder without purging:\n%s", ran)
	}
	if !strings.Contains(ran, "register-portable-app.ps1") || !strings.Contains(ran, "-Name 'Playnite'") {
		t.Errorf("Playnite must be registered in the Start menu:\n%s", ran)
	}
}

func TestPlayniteSaveSplitsGitAndStore(t *testing.T) {
	root, dir, store := t.TempDir(), filepath.Join(t.TempDir(), "Playnite"), t.TempDir()
	cfg := playnite.Default()
	cfg.Path, cfg.Store = dir, store
	must(t, playnite.Save(playnite.Path(root), cfg))
	for rel, body := range map[string]string{
		playnite.Exe:                     "MZ",
		"config.json":                    `{"Theme": "Default"}`,
		"ExtensionsData/abc/config.json": `{"SgdbApiKey": "k3y", "Width": 3}`,
		"ExtensionsData/abc/games.json":  `{}`,
		"library/games.db":               "db",
		"cache/images/x.png":             "png",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, []byte(body), 0o644))
	}
	// a cache copied by an older save
	must(t, os.MkdirAll(filepath.Join(store, playnite.SnapshotDir, "cache"), 0o755))

	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Runner: fr}
	m, err := env.SavePlaynite(context.Background(), io.Discard)
	must(t, err)
	git := playnite.GitDir(root)
	raw, err := os.ReadFile(filepath.Join(git, "ExtensionsData", "abc", "config.json"))
	must(t, err)
	if string(raw) != `{"SgdbApiKey": "", "Width": 3}` {
		t.Errorf("git settings: %s", raw)
	}
	for _, rel := range []string{"library", "ExtensionsData/abc/games.json", playnite.Exe} {
		if _, err := os.Stat(filepath.Join(git, filepath.FromSlash(rel))); err == nil {
			t.Errorf("%s went to git", rel)
		}
	}
	if m.Secrets != 1 {
		t.Errorf("manifest: %+v", m)
	}
	if _, err := os.Stat(filepath.Join(store, playnite.SnapshotDir, "cache")); err == nil {
		t.Error("the old cache is still in the store")
	}
	if !strings.Contains(ranLines(fr), "'"+filepath.Join(dir, "browsercache")+"'") {
		t.Errorf("the caches are not left out:\n%s", ranLines(fr))
	}
}
