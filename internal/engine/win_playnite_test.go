package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestPlayniteRestoreWithoutTheStore(t *testing.T) {
	root, dir := t.TempDir(), filepath.Join(t.TempDir(), "Playnite")
	cfg := playnite.Default()
	cfg.Path, cfg.Store, cfg.Version = dir, filepath.Join(t.TempDir(), "gone"), "10.64"
	must(t, playnite.Save(playnite.Path(root), cfg))
	git := playnite.GitDir(root)
	for rel, body := range map[string]string{
		"config.json":          `{"Theme": "Harmony"}`,
		"library/emulators.db": "emu",
		"library/games.db":     "never in git, but if it were it is skipped",
	} {
		p := filepath.Join(git, filepath.FromSlash(rel))
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, []byte(body), 0o644))
	}
	must(t, playnite.WriteAddons(playnite.AddonsFile(root), playnite.AddonList{Playnite: "10.64", Addons: []playnite.Addon{
		{ID: "Harmony_d49", Name: "Harmony", Version: "2.74", Kind: "desktop", Folder: "Themes/Desktop/Harmony_d49"},
		{ID: "SteamLibrary", Name: "Steam", Kind: "extension", Folder: "Extensions/SteamLibrary_Builtin", Builtin: true},
		{ID: "HandMade", Name: "Hand made", Kind: "extension", Folder: "Extensions/HandMade"},
	}}))

	var got []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path)
		switch r.URL.Path {
		case "/releases/10.64/10.64.7z":
			w.Write([]byte("7z"))
		case "/api/addons":
			fmt.Fprintf(w, `{"data":[{"addonId":"Harmony_d49","installerManifestUrl":"%s/manifest.yaml"}]}`, srv.URL)
		case "/manifest.yaml":
			fmt.Fprintf(w, "Packages:\n  - Version: 2.74\n    PackageUrl: %s/harmony.pthm\n", srv.URL)
		case "/harmony.pthm":
			w.Write(zipOf(t, map[string]string{"theme.yaml": "Id: Harmony_d49\nName: Harmony\nVersion: 2.74\n"}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	oldR, oldA := playnite.Releases, playnite.AddonAPI
	playnite.Releases, playnite.AddonAPI = srv.URL+"/releases", srv.URL+"/api/addons"
	defer func() { playnite.Releases, playnite.AddonAPI = oldR, oldA }()

	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Runner: fr}
	var log strings.Builder
	must(t, env.RestorePlaynite(context.Background(), &log, false))
	if !strings.Contains(ranLines(fr), "tar.exe -xf") {
		t.Errorf("the program was not extracted:\n%s", ranLines(fr))
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "Themes", "Desktop", "Harmony_d49", "theme.yaml")); err != nil || !strings.Contains(string(raw), "2.74") {
		t.Errorf("the theme was not installed: %v", err)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "library", "emulators.db")); string(raw) != "emu" {
		t.Error("the library databases did not come from git")
	}
	if exists(filepath.Join(dir, "library", "games.db")) {
		t.Error("games.db must not come from git")
	}
	if !strings.Contains(log.String(), "install by hand: Hand made") {
		t.Errorf("the add-on missing from the database is not reported:\n%s", log.String())
	}
	if exists(filepath.Join(dir, "Extensions", "SteamLibrary_Builtin")) {
		t.Error("a builtin add-on was downloaded")
	}
}
