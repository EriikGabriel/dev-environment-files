package engine

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestEnsureDenoInstallsWithWingetOnWindowsOnly(t *testing.T) {
	// no deno anywhere: an empty PATH, home and LocalAppData
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if p := DenoPath(); p != "" {
		t.Fatalf("deno found at %s", p)
	}

	fr := &fakeRunner{has: map[string]bool{"winget": true}}
	env := &Env{Info: win, Runner: fr}
	_, _, err := env.EnsureDeno(context.Background(), io.Discard)
	ran := ranLines(fr)
	if !strings.Contains(ran, "winget install --id DenoLand.Deno --exact --silent") {
		t.Errorf("Deno must be installed with winget, silently:\n%s", ran)
	}
	// the fake winget installs nothing, so it is reported
	if err == nil || !strings.Contains(err.Error(), "deno was not found") {
		t.Errorf("err = %v", err)
	}

	fr = &fakeRunner{has: map[string]bool{"winget": true}}
	env = &Env{Info: cachy, Runner: fr}
	if p, installed, err := env.EnsureDeno(context.Background(), io.Discard); p != "" || installed || err != nil || len(fr.ran) != 0 {
		t.Errorf("nothing is installed outside Windows: %q %v %v %v", p, installed, err, fr.ran)
	}
}
