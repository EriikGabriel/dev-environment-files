package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/EriikGabriel/dev-environment-files/internal/backend"
)

// DenoPackage is Deno's id in winget.
const DenoPackage = "DenoLand.Deno"

// DenoPath finds deno: on the PATH, or where its installers put it. The PATH
// of a program that is already running does not see an install made after
// it started, so the usual places are looked at too: ~/.deno/bin (the
// official install script) and winget's links and package folder.
func DenoPath() string {
	if p, err := exec.LookPath("deno"); err == nil {
		return p
	}
	exe := "deno"
	if runtime.GOOS == "windows" {
		exe = "deno.exe"
	}
	var candidates []string
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".deno", "bin", exe))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		candidates = append(candidates, filepath.Join(local, "Microsoft", "WinGet", "Links", exe))
		if m, _ := filepath.Glob(filepath.Join(local, "Microsoft", "WinGet", "Packages", DenoPackage+"_*", exe)); len(m) > 0 {
			candidates = append(candidates, m...)
		}
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// EnsureDeno installs Deno when it is missing. Only on Windows, where the app
// does it by itself in the background; elsewhere it only says where it is.
// winget installs it for the user, with no elevation and no window.
func (e *Env) EnsureDeno(ctx context.Context, out io.Writer) (path string, installed bool, err error) {
	if p := DenoPath(); p != "" {
		return p, false, nil
	}
	if e.Info.OS != "windows" {
		return "", false, nil
	}
	fmt.Fprintln(out, "installing Deno with winget ("+DenoPackage+")")
	if err := (backend.Winget{}).Install(ctx, e.Runner, out, DenoPackage); err != nil {
		return "", false, fmt.Errorf("install Deno: %w", err)
	}
	if p := DenoPath(); p != "" {
		return p, true, nil
	}
	return "", false, fmt.Errorf("winget finished, but deno was not found on the PATH, in ~/.deno/bin or in winget's folders")
}
