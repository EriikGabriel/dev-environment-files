package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// steamDir is where the Steam client lives.
func (e *Env) steamDir() string {
	if e.SteamDir != "" {
		return e.SteamDir
	}
	base := os.Getenv("ProgramFiles(x86)")
	if base == "" {
		base = `C:\Program Files (x86)`
	}
	return filepath.Join(base, "Steam")
}

var vdfPath = regexp.MustCompile(`"path"\s+"([^"]+)"`)

// steamLibraries lists the Steam library folders: the client's own, plus the
// ones on other drives recorded in libraryfolders.vdf.
func (e *Env) steamLibraries() []string {
	root := e.steamDir()
	libs := []string{root}
	b, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return libs
	}
	for _, m := range vdfPath.FindAllStringSubmatch(string(b), -1) {
		if dir := strings.ReplaceAll(m[1], `\\`, `\`); dir != root {
			libs = append(libs, dir)
		}
	}
	return libs
}

// steamInstalled reports whether a Steam app is in one of the libraries.
func (e *Env) steamInstalled(appID string) bool {
	for _, lib := range e.steamLibraries() {
		if _, err := os.Stat(filepath.Join(lib, "steamapps", "appmanifest_"+appID+".acf")); err == nil {
			return true
		}
	}
	return false
}

// steamInstall asks the Steam client to install an app. Steam shows its own
// dialog (drive, shortcuts) and the app must be owned by the account, so this
// only starts the installation: the item counts as installed on a later run.
func (e *Env) steamInstall(ctx context.Context, out io.Writer, appID string) error {
	if _, err := os.Stat(filepath.Join(e.steamDir(), "steam.exe")); err != nil {
		return fmt.Errorf("Steam is not installed in %s; install it first", e.steamDir())
	}
	fmt.Fprintf(out, "opening Steam to install app %s: confirm the dialog there (the app must be in your library)\n", appID)
	return e.Runner.Run(ctx, out, "cmd", "/c", "start", "", "steam://install/"+appID)
}
