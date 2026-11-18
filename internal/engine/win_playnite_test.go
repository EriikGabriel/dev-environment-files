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
	if l := p.Describe(context.Background()); !l[0].Pending || !strings.Contains(l[0].Detail, "Playnite snapshot") {
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

func TestPlayniteSaveRefusesWithoutStore(t *testing.T) {
	root := t.TempDir()
	env := &Env{Info: win, Root: root, Runner: &fakeRunner{}}
	if _, err := env.SavePlaynite(context.Background(), io.Discard); err == nil || !strings.Contains(err.Error(), "store") {
		t.Errorf("err = %v", err)
	}
}
