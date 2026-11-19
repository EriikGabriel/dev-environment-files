package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/retroarch"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// RetroArchConfig reads config/retroarch.yaml. A store that is not where the
// file says, but is on another drive under the same path (a drive letter
// that changed), is used from there.
func (e *Env) RetroArchConfig() (retroarch.Config, bool, error) {
	cfg, exists, err := retroarch.Load(retroarch.Path(e.Root))
	if err == nil {
		cfg.Store = snapshot.FindStore(cfg.Store, retroarch.ManifestFile)
	}
	return cfg, exists, err
}

// RetroArchRunning tells whether RetroArch is open: it writes saves, states
// and its settings while it runs.
func (e *Env) RetroArchRunning(ctx context.Context) bool {
	out, err := e.Runner.Output(ctx, e.script("Get-Process -Name 'retroarch' -ErrorAction SilentlyContinue | ForEach-Object { $_.Name }")...)
	return err == nil && strings.TrimSpace(out) != ""
}

func (e *Env) retroarchClosed(ctx context.Context) error {
	if e.RetroArchRunning(ctx) {
		return fmt.Errorf("close RetroArch first: it writes saves, states and retroarch.cfg while it is open")
	}
	return nil
}

// SaveRetroArch records the version and the cores in config/retroarch.yaml,
// copies the personal part to git (windows/retroarch, secrets blanked) and,
// when a store is set, mirrors the rest (BIOS and what is neither downloaded
// nor in git) and the games to it, with the secrets and a manifest.
func (e *Env) SaveRetroArch(ctx context.Context, out io.Writer) (retroarch.Manifest, error) {
	cfg, _, err := e.RetroArchConfig()
	if err != nil {
		return retroarch.Manifest{}, err
	}
	in := retroarch.Read(cfg, false)
	if !in.Found {
		return retroarch.Manifest{}, fmt.Errorf("%s has no %s", cfg.Path, retroarch.Exe)
	}
	if err := e.retroarchClosed(ctx); err != nil {
		return retroarch.Manifest{}, err
	}

	// what a restore downloads
	if v, err := e.Runner.Output(ctx, filepath.Join(cfg.Path, retroarch.Exe), "--version"); err == nil && retroarch.ParseVersion(v) != "" {
		cfg.Version = retroarch.ParseVersion(v)
	} else if cfg.Version == "" {
		fmt.Fprintln(out, "warning: the version of RetroArch could not be read; set version in config/retroarch.yaml or a restore cannot download it")
	}
	cfg.Cores = retroarch.CoreNames(cfg.Path)
	saved, _, err := retroarch.Load(retroarch.Path(e.Root))
	if err != nil {
		return retroarch.Manifest{}, err
	}
	saved.Version, saved.Cores = cfg.Version, cfg.Cores
	if err := retroarch.Save(retroarch.Path(e.Root), saved); err != nil {
		return retroarch.Manifest{}, err
	}
	fmt.Fprintf(out, "RetroArch %s with %d cores recorded in config/retroarch.yaml\n", cfg.Version, len(cfg.Cores))

	// the personal part, to git
	git, err := snapshot.Export(cfg.Path, retroarch.GitDir(e.Root), cfg.Git)
	if err != nil {
		return retroarch.Manifest{}, fmt.Errorf("copy the settings to %s: %w", retroarch.GitDir(e.Root), err)
	}
	reportExport(out, "windows/retroarch", git)

	host, _ := os.Hostname()
	m := retroarch.Manifest{
		SavedAt: time.Now().UTC().Truncate(time.Second), Source: cfg.Path, Games: cfg.Games, Computer: host,
		GameBytes: in.GameBytes, GameCount: in.GameCount, Cores: []string{}, Systems: []string{},
		Version: cfg.Version, Program: cfg.CopyProgram, Secrets: len(git.Secrets),
	}
	for _, c := range in.Cores {
		m.Cores = append(m.Cores, c.Name)
	}
	for _, s := range in.Systems {
		m.Systems = append(m.Systems, fmt.Sprintf("%s (%d)", s.Name, s.Games))
	}
	if cfg.Store == "" {
		fmt.Fprintln(out, "no store is set: the games, the BIOS and the secrets are only on this machine")
		return m, nil
	}

	// the rest, to the store
	exclude := cfg.StoreExclude()
	_, m.Bytes, m.Files = snapshot.Measure(cfg.Path, exclude)
	if err := os.MkdirAll(cfg.Snapshot(), 0o755); err != nil {
		return m, err
	}
	what := "the BIOS and what is not downloaded nor in git"
	if cfg.CopyProgram {
		what = "the program, the BIOS and what is not in git"
	}
	fmt.Fprintf(out, "copying %s to %s (%.2f GB, %d files); only what changed is copied again\n", what, cfg.Snapshot(), float64(m.Bytes)/(1<<30), m.Files)
	if err := e.robocopy(ctx, out, retroarch.RobocopyArgs(cfg.Path, cfg.Snapshot(), exclude, true)); err != nil {
		return m, err
	}
	if gone, err := retroarch.PruneStore(cfg); err != nil {
		return m, fmt.Errorf("clean the store: %w", err)
	} else if len(gone) > 0 {
		fmt.Fprintf(out, "removed from the store (now downloaded or in git): %s\n", strings.Join(gone, ", "))
	}
	if cfg.Games != "" {
		if _, err := os.Stat(cfg.Games); err != nil {
			return m, fmt.Errorf("the games folder %s does not exist (empty games in config/retroarch.yaml leaves the games out)", cfg.Games)
		}
		fmt.Fprintf(out, "copying %s to %s (%.1f GB, %d games)\n", cfg.Games, cfg.GamesSnapshot(), float64(in.GameBytes)/(1<<30), in.GameCount)
		if err := e.robocopy(ctx, out, retroarch.RobocopyArgs(cfg.Games, cfg.GamesSnapshot(), nil, true)); err != nil {
			return m, err
		}
	}
	if err := snapshot.WriteJSON(snapshot.SecretsFile(cfg.Store, retroarch.Program), git.Secrets); err != nil {
		return m, err
	}
	return m, retroarch.WriteManifest(cfg.Store, m)
}

// reportExport tells what went to git.
func reportExport(out io.Writer, dir string, r snapshot.Exported) {
	fmt.Fprintf(out, "git: %d files (%.1f MB) in %s; %d changed, %d removed\n", r.Files, float64(r.Bytes)/(1<<20), dir, len(r.Changed), len(r.Removed))
	if len(r.Secrets) > 0 {
		fmt.Fprintf(out, "git: %d passwords, keys or tokens left out of git (kept in the store)\n", len(r.Secrets))
	}
	if len(r.Skipped) > 0 {
		fmt.Fprintf(out, "git: too large for git, left out: %s\n", strings.Join(r.Skipped, ", "))
	}
}

// RestoreRetroArch puts RetroArch back: the program from the store (with
// copy_program) or downloaded, the missing cores downloaded, the BIOS and
// the games from the store, the personal part from git with the secrets of
// the store, then the playlists pointed at the games and the Start menu
// entry. mirror also deletes what a full copy in the store does not have.
func (e *Env) RestoreRetroArch(ctx context.Context, out io.Writer, mirror bool) error {
	cfg, _, err := e.RetroArchConfig()
	if err != nil {
		return err
	}
	var m *retroarch.Manifest
	if cfg.Store != "" {
		if m, err = retroarch.ReadManifest(cfg.Store); err != nil {
			return err
		}
	}
	gitDir := retroarch.GitDir(e.Root)
	inGit := len(snapshot.ProfileFiles(gitDir, cfg.Git))
	if m == nil && inGit == 0 {
		if cfg.Store == "" {
			return fmt.Errorf("nothing to restore: %s is empty and no store is set (save with retroarch-save first)", gitDir)
		}
		return fmt.Errorf("nothing to restore: %s has no snapshot (%s is missing) and %s is empty", cfg.Store, retroarch.ManifestFile, gitDir)
	}
	if err := e.retroarchClosed(ctx); err != nil {
		return err
	}
	full := m != nil && exists(filepath.Join(cfg.Snapshot(), retroarch.Exe))

	// the program
	if !full && !exists(filepath.Join(cfg.Path, retroarch.Exe)) {
		if err := e.downloadRetroArch(ctx, out, cfg); err != nil {
			return err
		}
	}
	// the store: the whole program, or the BIOS and the rest
	if m != nil {
		fmt.Fprintf(out, "copying the store (saved %s on %s) to %s\n", m.SavedAt.Local().Format("2006-01-02 15:04"), m.Computer, cfg.Path)
		if err := e.robocopy(ctx, out, retroarch.RobocopyArgs(cfg.Snapshot(), cfg.Path, cfg.RestoreExclude(), mirror && full)); err != nil {
			return err
		}
		if cfg.Games != "" {
			if exists(cfg.GamesSnapshot()) {
				fmt.Fprintf(out, "copying the games to %s\n", cfg.Games)
				if err := e.robocopy(ctx, out, retroarch.RobocopyArgs(cfg.GamesSnapshot(), cfg.Games, nil, mirror)); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(out, "the store has no games (%s)\n", cfg.GamesSnapshot())
			}
		}
	} else {
		fmt.Fprintln(out, "no store: the games and the BIOS are not restored; copy them to the folders by hand")
	}
	// cores the store did not bring
	if err := e.downloadCores(ctx, out, cfg); err != nil {
		return err
	}
	// the personal part
	if inGit > 0 {
		var secrets []snapshot.Secret
		if cfg.Store != "" {
			if secrets, err = snapshot.ReadSecrets(snapshot.SecretsFile(cfg.Store, retroarch.Program)); err != nil {
				return err
			}
		}
		res, err := snapshot.Import(gitDir, cfg.Path, cfg.Git, secrets)
		if err != nil {
			return fmt.Errorf("copy the settings from %s: %w", gitDir, err)
		}
		fmt.Fprintf(out, "settings, saves and playlists from git: %d files, %d updated, %d passwords or tokens put back\n", res.Files, res.Changed, res.Secrets)
	}
	if err := e.fixPlaylists(out, cfg); err != nil {
		return err
	}
	if cfg.StartMenu {
		return e.registerRetroArch(ctx, out, cfg)
	}
	return nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// downloadRetroArch puts the portable RetroArch of cfg.Version and the asset
// packs in cfg.Path.
func (e *Env) downloadRetroArch(ctx context.Context, out io.Writer, cfg retroarch.Config) error {
	if cfg.Version == "" {
		return fmt.Errorf("config/retroarch.yaml has no version to download: save once with RetroArch installed, or set it by hand")
	}
	tmp, err := os.MkdirTemp("", "envsetup-retroarch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, "RetroArch.7z")
	fmt.Fprintf(out, "downloading RetroArch %s from %s\n", cfg.Version, retroarch.Buildbot)
	if err := retroarch.Download(ctx, retroarch.ProgramURL(cfg.Version), archive, out); err != nil {
		return err
	}
	// the bsdtar of Windows reads 7z (Git's tar does not)
	tar := filepath.Join(os.Getenv("SystemRoot"), "System32", "tar.exe")
	if !exists(tar) {
		tar = "tar"
	}
	if err := e.Runner.Run(ctx, out, tar, "-xf", archive, "-C", tmp); err != nil {
		return fmt.Errorf("extract %s: %w", archive, err)
	}
	if err := os.MkdirAll(cfg.Path, 0o755); err != nil {
		return err
	}
	if err := e.robocopy(ctx, out, retroarch.RobocopyArgs(filepath.Join(tmp, retroarch.ProgramFolder), cfg.Path, nil, false)); err != nil {
		return err
	}
	for _, b := range cfg.Bundles {
		file := filepath.Join(tmp, b+".zip")
		if err := retroarch.Download(ctx, retroarch.BundleURL(b), file, out); err != nil {
			return err
		}
		n, skipped, err := retroarch.Unzip(file, filepath.Join(cfg.Path, retroarch.BundleDirs[b]))
		if err != nil {
			return fmt.Errorf("extract %s: %w", b, err)
		}
		if skipped > 0 {
			fmt.Fprintf(out, "%s: %d files, %d with names Windows refuses skipped\n", b, n, skipped)
		}
		os.Remove(file)
	}
	return nil
}

// downloadCores gets the cores of cfg.Cores the cores folder lacks.
func (e *Env) downloadCores(ctx context.Context, out io.Writer, cfg retroarch.Config) error {
	var missing []string
	for _, c := range cfg.Cores {
		if !exists(filepath.Join(cfg.Path, "cores", c+"_libretro.dll")) {
			missing = append(missing, c)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	tmp, err := os.MkdirTemp("", "envsetup-cores-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	fmt.Fprintf(out, "downloading %d cores: %s\n", len(missing), strings.Join(missing, ", "))
	for _, c := range missing {
		file := filepath.Join(tmp, c+".zip")
		if err := retroarch.Download(ctx, retroarch.CoreURL(c), file, out); err != nil {
			return err
		}
		if _, _, err := retroarch.Unzip(file, filepath.Join(cfg.Path, "cores")); err != nil {
			return fmt.Errorf("extract the core %s: %w", c, err)
		}
	}
	return nil
}

func (e *Env) fixPlaylists(out io.Writer, cfg retroarch.Config) error {
	fixes, err := retroarch.FixPlaylists(cfg)
	if err != nil {
		return fmt.Errorf("point the playlists at the games: %w", err)
	}
	for _, f := range fixes {
		if f.Games == 0 && f.Cores == 0 && len(f.Missing) == 0 {
			continue
		}
		fmt.Fprintf(out, "playlist %s: %d games and %d cores pointed at their new place", f.Name, f.Games, f.Cores)
		if len(f.Missing) > 0 {
			fmt.Fprintf(out, "; not found: %s", strings.Join(f.Missing, ", "))
		}
		fmt.Fprintln(out)
	}
	return nil
}

func (e *Env) registerRetroArch(ctx context.Context, out io.Writer, cfg retroarch.Config) error {
	script := filepath.Join(e.Root, "windows", "register-portable-app.ps1")
	cmd := fmt.Sprintf("& %s -Path %s -Name 'RetroArch' -Publisher 'Libretro'", psQuote(script), psQuote(filepath.Join(cfg.Path, retroarch.Exe)))
	if err := e.Runner.Run(ctx, out, e.script(cmd)...); err != nil {
		return fmt.Errorf("register RetroArch in the Start menu: %w", err)
	}
	return nil
}

// FixRetroArch takes care of one requirement (see retroarch.Requirements).
func (e *Env) FixRetroArch(ctx context.Context, out io.Writer, id string) error {
	cfg, _, err := e.RetroArchConfig()
	if err != nil {
		return err
	}
	switch id {
	case retroarch.ReqPlaylists:
		if err := e.retroarchClosed(ctx); err != nil {
			return err
		}
		return e.fixPlaylists(out, cfg)
	case retroarch.ReqStartMenu:
		if !exists(filepath.Join(cfg.Path, retroarch.Exe)) {
			return fmt.Errorf("%s has no %s", cfg.Path, retroarch.Exe)
		}
		return e.registerRetroArch(ctx, out, cfg)
	}
	return fmt.Errorf("unknown requirement %q", id)
}

// retroarchInstalled is the check of the catalogue method retroarch.
func (e *Env) retroarchInstalled() bool {
	cfg, _, err := e.RetroArchConfig()
	if err != nil {
		return false
	}
	return exists(filepath.Join(cfg.Path, retroarch.Exe))
}

// retroarchInstall restores RetroArch (it fixes the playlists too).
func (e *Env) retroarchInstall(ctx context.Context, out io.Writer) error {
	return e.RestoreRetroArch(ctx, out, false)
}
