package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/winsetup"
)

// StateDir is where envsetup keeps its logs and markers: $XDG_STATE_HOME,
// %LocalAppData% on Windows, else ~/.local/state, plus "envsetup". It
// returns "" when none of them can be found.
func StateDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" && runtime.GOOS == "windows" {
		dir, _ = os.UserCacheDir()
	}
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "envsetup")
}

func (e *Env) niniteMarker() string {
	dir := e.StateDir
	if dir == "" {
		dir = StateDir()
	}
	return filepath.Join(dir, "ninite-done")
}

// niniteDone reports whether the Ninite installer already ran here for this
// exact list. Ninite has no per-app state to ask, so envsetup remembers the
// last list it ran: changing the list makes the item pending again.
func (e *Env) niniteDone(slugs string) bool {
	b, err := os.ReadFile(e.niniteMarker())
	return err == nil && strings.TrimSpace(string(b)) == slugs
}

// niniteInstall downloads the installer for the list and runs it. Ninite
// asks for elevation and shows its own window, which the user closes when it
// finishes; the item is done when that window is gone.
func (e *Env) niniteInstall(ctx context.Context, out io.Writer, slugs string) error {
	list := strings.Fields(slugs)
	dir, err := os.MkdirTemp("", "envsetup-ninite-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	exe := filepath.Join(dir, winsetup.NiniteFile)
	link := winsetup.NiniteInstaller(list)
	fmt.Fprintf(out, "downloading %s\n", link)
	fetch := e.Fetch
	if fetch == nil {
		fetch = winsetup.Download
	}
	if err := fetch(ctx, link, exe); err != nil {
		return fmt.Errorf("download the Ninite installer: %w", err)
	}
	fmt.Fprintln(out, "running Ninite: accept the elevation prompt and close its window when it finishes")
	// Start-Process goes through the shell, which is what raises the UAC
	// prompt; starting the .exe directly fails with "requires elevation".
	run := "Start-Process -FilePath '" + strings.ReplaceAll(exe, "'", "''") + "' -Wait"
	if err := e.Runner.Run(ctx, out, e.script(run)...); err != nil {
		return err
	}
	marker := e.niniteMarker()
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(slugs+"\n"), 0o644)
}
