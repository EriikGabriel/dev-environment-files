package engine

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/playnite"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/retroarch"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// RelocatePlaynite rewrites the paths of the Playnite library (games,
// emulators, game scanners) that start with from to start with to
// (windows/playnite-relocate.ps1). Nothing happens when Playnite is not
// installed or the folders are the same.
func (e *Env) RelocatePlaynite(ctx context.Context, out io.Writer, from, to string) error {
	if from == "" || to == "" || snapshot.Clean(from) == snapshot.Clean(to) {
		return nil
	}
	cfg, _, err := e.PlayniteConfig()
	if err != nil {
		return err
	}
	if !exists(filepath.Join(cfg.Path, playnite.Exe)) {
		return nil
	}
	if err := e.playniteClosed(ctx); err != nil {
		return err
	}
	script := filepath.Join(e.Root, "windows", "playnite-relocate.ps1")
	fmt.Fprintf(out, "Playnite: %s -> %s in the library\n", from, to)
	cmd := fmt.Sprintf("& %s -Playnite %s -From %s -To %s", psQuote(script), psQuote(cfg.Path), psQuote(from), psQuote(to))
	if err := e.Runner.Run(ctx, out, e.script(cmd)...); err != nil {
		return fmt.Errorf("point the Playnite library at %s: %w", to, err)
	}
	return nil
}

// MoveRetroArchGames moves the games folder of RetroArch to dest: the files
// are copied, checked and only then deleted from the old folder; then
// config/retroarch.yaml, the playlists and the Playnite library point at the
// new folder. RetroArch and Playnite must be closed.
func (e *Env) MoveRetroArchGames(ctx context.Context, out io.Writer, dest string) error {
	cfg, _, err := e.RetroArchConfig()
	if err != nil {
		return err
	}
	dest = filepath.Clean(dest)
	src := cfg.Games
	if src != "" && snapshot.Clean(src) == snapshot.Clean(dest) {
		return fmt.Errorf("the games are already in %s", dest)
	}
	for _, other := range []string{cfg.Path, cfg.Store} {
		if snapshot.Overlap(dest, other) {
			return fmt.Errorf("%s overlaps %s: choose a folder of its own", dest, other)
		}
	}
	if src != "" && snapshot.Overlap(src, dest) {
		return fmt.Errorf("%s and %s are one inside the other", src, dest)
	}
	if err := e.retroarchClosed(ctx); err != nil {
		return err
	}
	if exists(filepath.Join(e.playnitePath(), playnite.Exe)) {
		if err := e.playniteClosed(ctx); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	moved := src != "" && exists(src)
	if moved {
		fmt.Fprintf(out, "copying %s to %s\n", src, dest)
		if err := e.robocopy(ctx, out, retroarch.RobocopyArgs(src, dest, nil, false)); err != nil {
			return err
		}
		if missing := missingCopies(src, dest); len(missing) > 0 {
			return fmt.Errorf("the copy is incomplete, %s was kept: %s", src, strings.Join(missing, ", "))
		}
		if err := os.RemoveAll(src); err != nil {
			return fmt.Errorf("the games are in %s, but %s could not be deleted: %w", dest, src, err)
		}
		fmt.Fprintf(out, "%s deleted\n", src)
	}

	saved, _, err := retroarch.Load(retroarch.Path(e.Root))
	if err != nil {
		return err
	}
	saved.Games = dest
	if err := retroarch.Save(retroarch.Path(e.Root), saved); err != nil {
		return err
	}
	cfg.Games = dest
	fmt.Fprintf(out, "config/retroarch.yaml: games = %s\n", dest)
	if exists(filepath.Join(cfg.Path, retroarch.Exe)) {
		if err := e.fixPlaylists(out, cfg); err != nil {
			return err
		}
	}
	if moved {
		return e.RelocatePlaynite(ctx, out, src, dest)
	}
	return nil
}

// ClosePlaynite asks the running Playnite to exit (Playnite.DesktopApp.exe
// --shutdown, what its Exit menu does: the library is saved first) and
// waits up to a minute for it.
func (e *Env) ClosePlaynite(ctx context.Context, out io.Writer) error {
	if len(e.PlayniteRunning(ctx)) == 0 {
		fmt.Fprintln(out, "Playnite is not open")
		return nil
	}
	exe := filepath.Join(e.playnitePath(), playnite.Exe)
	cmd := fmt.Sprintf("Start-Process -FilePath %s -ArgumentList '--shutdown'; "+
		"$end = (Get-Date).AddSeconds(60); "+
		"while ((Get-Process -Name 'Playnite.DesktopApp','Playnite.FullscreenApp' -ErrorAction SilentlyContinue) -and (Get-Date) -lt $end) { Start-Sleep -Milliseconds 500 }", psQuote(exe))
	if err := e.Runner.Run(ctx, out, e.script(cmd)...); err != nil {
		return err
	}
	if open := e.PlayniteRunning(ctx); len(open) > 0 {
		return fmt.Errorf("Playnite did not close (%s): close it from its menu", strings.Join(open, ", "))
	}
	fmt.Fprintln(out, "Playnite closed")
	return nil
}

func (e *Env) playnitePath() string {
	cfg, _, err := e.PlayniteConfig()
	if err != nil {
		return ""
	}
	return cfg.Path
}

// missingCopies lists the files of src that dst lacks or has with another
// size (relative paths, at most 5).
func missingCopies(src, dst string) []string {
	var out []string
	filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(out) >= 5 {
			return nil
		}
		rel, _ := filepath.Rel(src, p)
		a, errA := d.Info()
		b, errB := os.Stat(filepath.Join(dst, rel))
		if errA != nil || errB != nil || a.Size() != b.Size() {
			out = append(out, rel)
		}
		return nil
	})
	return out
}
