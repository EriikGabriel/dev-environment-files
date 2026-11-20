package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/retroarch"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

func TestRetroArchStepRestoresTheSnapshot(t *testing.T) {
	root, base, store := t.TempDir(), t.TempDir(), t.TempDir()
	cfg := retroarch.Config{Path: filepath.Join(base, "RetroArch"), Games: filepath.Join(base, "Games"), Store: store, StartMenu: true, Exclude: []string{"logs"}}
	must(t, retroarch.Save(retroarch.Path(root), cfg))

	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Home: t.TempDir(), Runner: fr, StateDir: t.TempDir()}
	p, err := BuildPlan(realConfig(t), "gaming-windows", env, Options{Apps: []string{"retroarch"}, NoFiles: true})
	must(t, err)
	if len(p.Steps) != 1 {
		t.Fatalf("want the retroarch step, got %d (skipped %+v)", len(p.Steps), p.Skipped)
	}
	if l := p.Describe(context.Background()); !l[0].Pending || !strings.Contains(l[0].Detail, "RetroArch") {
		t.Fatalf("without the folder it is pending: %+v", l)
	}

	// no snapshot yet: a clear failure, nothing copied
	if res := p.Execute(context.Background(), io.Discard, func(Event) {}); res.Failed != 1 {
		t.Fatalf("restoring without a snapshot must fail: %+v", res)
	}
	if strings.Contains(ranLines(fr), "robocopy") {
		t.Fatalf("nothing should be copied without a snapshot:\n%s", ranLines(fr))
	}

	must(t, os.MkdirAll(cfg.GamesSnapshot(), 0o755))
	must(t, os.MkdirAll(cfg.Snapshot(), 0o755))
	must(t, os.WriteFile(filepath.Join(cfg.Snapshot(), retroarch.Exe), []byte("MZ"), 0o644))
	must(t, retroarch.WriteManifest(store, retroarch.Manifest{Source: cfg.Path}))
	if res := p.Execute(context.Background(), io.Discard, func(Event) {}); res.Failed != 0 {
		t.Fatalf("restore failed: %+v", res)
	}
	ran := ranLines(fr)
	for _, want := range []string{
		"& robocopy '" + cfg.Snapshot() + "' '" + cfg.Path + "' '/E'",
		"& robocopy '" + cfg.GamesSnapshot() + "' '" + cfg.Games + "' '/E'",
		"-Name 'RetroArch'",
	} {
		if !strings.Contains(ran, want) {
			t.Errorf("missing %q in:\n%s", want, ran)
		}
	}
}

// raFixture is a RetroArch folder with a bit of everything.
func raFixture(t *testing.T) retroarch.Config {
	t.Helper()
	base := t.TempDir()
	cfg := retroarch.Default()
	cfg.Path, cfg.Games, cfg.Store = filepath.Join(base, "RetroArch"), filepath.Join(base, "Games"), filepath.Join(base, "Store")
	for rel, body := range map[string]string{
		retroarch.Exe:               "MZ",
		"SDL2.dll":                  "dll",
		"retroarch.cfg":             "video_driver = \"vulkan\"\ncheevos_password = \"hunter2\"\n",
		`saves\Snes9x\Mario.srm`:    "save",
		`system\scph5501.bin`:       "bios",
		`cores\snes9x_libretro.dll`: "core",
		`assets\xmb\icon.png`:       "png",
		`logs\retroarch.log`:        "log",
	} {
		p := filepath.Join(cfg.Path, rel)
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, []byte(body), 0o644))
	}
	must(t, os.MkdirAll(filepath.Join(cfg.Games, "SNES"), 0o755))
	must(t, os.WriteFile(filepath.Join(cfg.Games, "SNES", "Mario.sfc"), []byte("rom"), 0o644))
	return cfg
}

func TestRetroArchSaveSplitsGitAndStore(t *testing.T) {
	root := t.TempDir()
	cfg := raFixture(t)
	must(t, retroarch.Save(retroarch.Path(root), cfg))
	// a full copy saved before, whose program must go
	must(t, os.MkdirAll(filepath.Join(cfg.Snapshot(), "assets"), 0o755))
	must(t, os.WriteFile(filepath.Join(cfg.Snapshot(), retroarch.Exe), []byte("MZ"), 0o644))

	fr := &fakeRunner{answers: map[string]string{"--version": "RetroArch - Frontend for libretro\nVersion: 1.22.2 (Git 69a4f0e)\n"}}
	env := &Env{Info: win, Root: root, Runner: fr}
	m, err := env.SaveRetroArch(context.Background(), io.Discard)
	must(t, err)

	saved, _, err := retroarch.Load(retroarch.Path(root))
	must(t, err)
	if saved.Version != "1.22.2" || len(saved.Cores) != 1 || saved.Cores[0] != "snes9x" {
		t.Errorf("version and cores not recorded: %+v", saved)
	}
	git := retroarch.GitDir(root)
	raw, err := os.ReadFile(filepath.Join(git, "retroarch.cfg"))
	must(t, err)
	if strings.Contains(string(raw), "hunter2") || !strings.Contains(string(raw), "vulkan") {
		t.Errorf("git cfg: %s", raw)
	}
	if _, err := os.Stat(filepath.Join(git, "saves", "Snes9x", "Mario.srm")); err != nil {
		t.Error("the save is not in git")
	}
	for _, rel := range []string{`system`, `cores`, retroarch.Exe} {
		if _, err := os.Stat(filepath.Join(git, rel)); err == nil {
			t.Errorf("%s went to git", rel)
		}
	}
	secrets, err := snapshot.ReadSecrets(snapshot.SecretsFile(cfg.Store, retroarch.Program))
	must(t, err)
	if len(secrets) != 1 || secrets[0].Value != "hunter2" || m.Secrets != 1 {
		t.Errorf("secrets: %+v", secrets)
	}
	if m.Program || m.Version != "1.22.2" {
		t.Errorf("manifest: %+v", m)
	}
	// the copy leaves out the program, git and the logs
	ran := ranLines(fr)
	for _, want := range []string{"'" + filepath.Join(cfg.Path, "cores") + "'", "'" + filepath.Join(cfg.Path, "saves") + "'", "'SDL2.dll'", "'retroarch.cfg'"} {
		if !strings.Contains(ran, want) {
			t.Errorf("the store copy does not leave out %s:\n%s", want, ran)
		}
	}
	for _, gone := range []string{retroarch.Exe, "assets"} {
		if _, err := os.Stat(filepath.Join(cfg.Snapshot(), gone)); err == nil {
			t.Errorf("%s left in the store", gone)
		}
	}
}

func TestRetroArchSaveWithoutStoreOnlyGit(t *testing.T) {
	root := t.TempDir()
	cfg := raFixture(t)
	cfg.Store = ""
	must(t, retroarch.Save(retroarch.Path(root), cfg))
	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Runner: fr}
	if _, err := env.SaveRetroArch(context.Background(), io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(retroarch.GitDir(root), "retroarch.cfg")); err != nil {
		t.Error("git was not updated")
	}
	if strings.Contains(ranLines(fr), "robocopy") {
		t.Error("nothing should be copied without a store")
	}
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, body := range files {
		f, err := w.Create(name)
		must(t, err)
		f.Write([]byte(body))
	}
	must(t, w.Close())
	return b.Bytes()
}

func TestRetroArchRestoreDownloads(t *testing.T) {
	// the machine before: saved to git and a light store
	root := t.TempDir()
	cfg := raFixture(t)
	cfg.Bundles = []string{"info"}
	cfg.CopyGames = true
	must(t, retroarch.Save(retroarch.Path(root), cfg))
	fr := &fakeRunner{answers: map[string]string{"--version": "Version: 1.22.2\n"}}
	env := &Env{Info: win, Root: root, Runner: fr}
	_, err := env.SaveRetroArch(context.Background(), io.Discard)
	must(t, err)

	// the machine after: nothing in the RetroArch folder
	must(t, os.RemoveAll(cfg.Path))
	must(t, os.MkdirAll(cfg.GamesSnapshot(), 0o755)) // robocopy is not run in the tests
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, ".7z"):
			w.Write([]byte("7z"))
		case strings.HasSuffix(r.URL.Path, "info.zip"):
			w.Write(zipOf(t, map[string]string{"snes9x_libretro.info": "display_name = \"Snes9x\""}))
		case strings.HasSuffix(r.URL.Path, "snes9x_libretro.dll.zip"):
			w.Write(zipOf(t, map[string]string{"snes9x_libretro.dll": "core"}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	old := retroarch.Buildbot
	retroarch.Buildbot = srv.URL
	defer func() { retroarch.Buildbot = old }()

	fr.ran = nil
	must(t, env.RestoreRetroArch(context.Background(), io.Discard, false))
	want := []string{"/stable/1.22.2/windows/x86_64/RetroArch.7z", "/assets/frontend/info.zip", "/nightly/windows/x86_64/latest/snes9x_libretro.dll.zip"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("downloads = %v", got)
	}
	for rel, body := range map[string]string{
		`cores\snes9x_libretro.dll`: "core",
		`info\snes9x_libretro.info`: "display_name = \"Snes9x\"",
		"retroarch.cfg":             "video_driver = \"vulkan\"\ncheevos_password = \"hunter2\"\n",
		`saves\Snes9x\Mario.srm`:    "save",
	} {
		raw, err := os.ReadFile(filepath.Join(cfg.Path, rel))
		if err != nil || string(raw) != body {
			t.Errorf("%s = %q (%v)", rel, raw, err)
		}
	}
	ran := ranLines(fr)
	for _, want := range []string{"tar.exe -xf", "& robocopy '" + cfg.Snapshot() + "' '" + cfg.Path + "' '/E'", "& robocopy '" + cfg.GamesSnapshot() + "'"} {
		if !strings.Contains(ran, want) {
			t.Errorf("missing %q in:\n%s", want, ran)
		}
	}
}

func TestMoveRetroArchGames(t *testing.T) {
	root := t.TempDir()
	cfg := raFixture(t)
	cfg.Store = ""
	must(t, retroarch.Save(retroarch.Path(root), cfg))
	dest := filepath.Join(t.TempDir(), "Games", "Retro")
	// robocopy does not run in the tests: the copy is made here
	must(t, os.MkdirAll(filepath.Join(dest, "SNES"), 0o755))
	must(t, os.WriteFile(filepath.Join(dest, "SNES", "Mario.sfc"), []byte("rom"), 0o644))
	old := cfg.Games

	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Runner: fr}
	must(t, env.MoveRetroArchGames(context.Background(), io.Discard, dest))
	if exists(old) {
		t.Error("the old folder is still there")
	}
	saved, _, err := retroarch.Load(retroarch.Path(root))
	must(t, err)
	if saved.Games != dest {
		t.Errorf("games = %s", saved.Games)
	}
	if !strings.Contains(ranLines(fr), "robocopy '"+old+"' '"+dest+"'") {
		t.Errorf("no copy:\n%s", ranLines(fr))
	}

	// an incomplete copy keeps the old folder
	other := filepath.Join(t.TempDir(), "Other")
	must(t, os.MkdirAll(other, 0o755))
	if err := env.MoveRetroArchGames(context.Background(), io.Discard, other); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Errorf("err = %v", err)
	}
	if !exists(dest) {
		t.Error("the games were deleted after an incomplete copy")
	}
}

func TestRetroArchRestoreForSomeoneElse(t *testing.T) {
	root := t.TempDir()
	cfg := raFixture(t)
	cfg.Store = ""
	must(t, retroarch.Save(retroarch.Path(root), cfg))
	git := retroarch.GitDir(root)
	for rel, body := range map[string]string{
		"retroarch.cfg":         "video_driver = \"vulkan\"\ncheevos_username = \"Owner\"\n",
		"saves/Snes9x/Mine.srm": "owner's save",
		"playlists/SNES.lpl":    "{}",
		"config/Snes9x/x.opt":   "snes9x_region = \"auto\"\n",
	} {
		p := filepath.Join(git, filepath.FromSlash(rel))
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, []byte(body), 0o644))
	}
	must(t, os.RemoveAll(filepath.Join(cfg.Path, "saves")))
	env := &Env{Info: win, Root: root, Runner: &fakeRunner{}, Guest: true}
	must(t, env.RestoreRetroArch(context.Background(), io.Discard, false))
	raw, _ := os.ReadFile(filepath.Join(cfg.Path, "retroarch.cfg"))
	if string(raw) != "video_driver = \"vulkan\"\ncheevos_username = \"\"\n" {
		t.Errorf("cfg: %q", raw)
	}
	if exists(filepath.Join(cfg.Path, "saves", "Snes9x", "Mine.srm")) || exists(filepath.Join(cfg.Path, "playlists", "SNES.lpl")) {
		t.Error("the owner's saves or playlists were copied")
	}
	if !exists(filepath.Join(cfg.Path, "config", "Snes9x", "x.opt")) {
		t.Error("the core options were left out")
	}

	// saving makes the folder theirs
	_, err := env.SaveRetroArch(context.Background(), io.Discard)
	must(t, err)
	if owner, explicit := config.IsOwner(root); !owner || !explicit || env.Guest {
		t.Errorf("after a save: owner=%v explicit=%v guest=%v", owner, explicit, env.Guest)
	}
}

// RetroArch already in its folder, without the Start menu entry (it was copied
// there by hand, or the restore stopped before): the item is pending and only
// registers it, so Windows search finds it and the taskbar can pin it.
func TestRetroArchInPlaceOnlyGetsRegistered(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir()) // no Start menu entry yet
	root := t.TempDir()
	cfg := raFixture(t)
	cfg.StartMenu = true
	must(t, retroarch.Save(retroarch.Path(root), cfg))

	fr := &fakeRunner{}
	env := &Env{Info: win, Root: root, Home: t.TempDir(), Runner: fr, StateDir: t.TempDir()}
	if env.retroarchInstalled() {
		t.Fatal("without the Start menu entry the item is pending")
	}
	must(t, env.retroarchInstall(context.Background(), io.Discard))
	ran := ranLines(fr)
	if !strings.Contains(ran, "register-portable-app.ps1") || !strings.Contains(ran, "-Name 'RetroArch'") || strings.Contains(ran, "robocopy") {
		t.Errorf("only the registration must run:\n%s", ran)
	}

	link := retroarch.StartMenuLink()
	must(t, os.MkdirAll(filepath.Dir(link), 0o755))
	must(t, os.WriteFile(link, nil, 0o644))
	if !env.retroarchInstalled() {
		t.Error("with the folder and the entry it is done")
	}
}
