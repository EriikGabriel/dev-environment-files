package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/playnite"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/policy"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// PlayniteConfig reads config/playnite.yaml; a store on a drive whose letter
// changed is found (see snapshot.FindStore).
func (e *Env) PlayniteConfig() (playnite.Config, bool, error) {
	cfg, exists, err := playnite.Load(playnite.Path(e.Root))
	if err == nil {
		cfg.Store = snapshot.FindStore(cfg.Store, playnite.ManifestFile)
	}
	return cfg, exists, err
}

// PlayniteRunning lists the Playnite programs that are open: their database
// cannot be copied consistently while they run.
func (e *Env) PlayniteRunning(ctx context.Context) []string {
	out, err := e.Runner.Output(ctx, e.script("Get-Process -Name 'Playnite.DesktopApp','Playnite.FullscreenApp' -ErrorAction SilentlyContinue | ForEach-Object { $_.Name }")...)
	if err != nil {
		return nil
	}
	return strings.Fields(out)
}

func (e *Env) playniteClosed(ctx context.Context) error {
	if open := e.PlayniteRunning(ctx); len(open) > 0 {
		return fmt.Errorf("close Playnite first (%s is running): its library cannot be copied while it is open", strings.Join(open, ", "))
	}
	return nil
}

// robocopy runs robocopy, whose exit codes below 8 mean success.
func (e *Env) robocopy(ctx context.Context, out io.Writer, args []string) error {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = psQuote(a)
	}
	cmd := "& robocopy " + strings.Join(q, " ") + "; if ($LASTEXITCODE -ge 8) { throw \"robocopy failed with code $LASTEXITCODE\" }; exit 0"
	return e.Runner.Run(ctx, out, e.script(cmd)...)
}

// SavePlaynite copies the settings to git (windows/playnite, secrets
// blanked) and, when a store is set, mirrors the rest of the folder to it
// with the secrets and the manifest.
func (e *Env) SavePlaynite(ctx context.Context, out io.Writer) (playnite.Manifest, error) {
	cfg, _, err := e.PlayniteConfig()
	if err != nil {
		return playnite.Manifest{}, err
	}
	in := playnite.Read(cfg, false)
	if !in.Found {
		return playnite.Manifest{}, fmt.Errorf("%s has no %s", cfg.Path, playnite.Exe)
	}
	if err := e.playniteClosed(ctx); err != nil {
		return playnite.Manifest{}, err
	}
	git, err := snapshot.Export(cfg.Path, playnite.GitDir(e.Root), cfg.Git)
	if err != nil {
		return playnite.Manifest{}, fmt.Errorf("copy the settings to %s: %w", playnite.GitDir(e.Root), err)
	}
	reportExport(out, "windows/playnite", git)
	e.becomeOwner(out)

	// what a restore without the store downloads: the program and the add-ons
	if v, err := e.Runner.Output(ctx, e.script(fmt.Sprintf("(Get-Item -LiteralPath %s).VersionInfo.FileVersion", psQuote(filepath.Join(cfg.Path, "Playnite.dll"))))...); err == nil && playnite.ParseVersion(v) != "" {
		cfg.Version = playnite.ParseVersion(v)
		saved, _, err := playnite.Load(playnite.Path(e.Root))
		if err != nil {
			return playnite.Manifest{}, err
		}
		if saved.Version != cfg.Version {
			saved.Version = cfg.Version
			if err := playnite.Save(playnite.Path(e.Root), saved); err != nil {
				return playnite.Manifest{}, err
			}
		}
	} else if cfg.Version == "" {
		fmt.Fprintln(out, "warning: the version of Playnite could not be read; set version in config/playnite.yaml or a restore without the HD cannot download it")
	}
	addons := playnite.ScanAddons(cfg.Path)
	if err := playnite.WriteAddons(playnite.AddonsFile(e.Root), playnite.AddonList{Playnite: cfg.Version, Addons: addons}); err != nil {
		return playnite.Manifest{}, err
	}
	fmt.Fprintf(out, "Playnite %s and %d themes and extensions listed in windows/playnite/addons.yaml\n", cfg.Version, len(addons))
	host, _ := os.Hostname()
	m := playnite.Manifest{SavedAt: time.Now().UTC().Truncate(time.Second), Source: cfg.Path, Computer: host, Extensions: []string{}, Secrets: len(git.Secrets)}
	if ra, ok, err := e.RetroArchConfig(); err == nil && ok {
		m.RetroArch, m.RetroGames = ra.Path, ra.Games
	}
	if cfg.Store == "" {
		fmt.Fprintln(out, "no store is set: the library, the media and the secrets are only on this machine")
		return m, nil
	}
	exclude := cfg.StoreExclude()
	_, m.Bytes, m.Files = snapshot.Measure(cfg.Path, exclude)
	if err := os.MkdirAll(cfg.Snapshot(), 0o755); err != nil {
		return m, err
	}
	fmt.Fprintf(out, "copying %s to %s (%.1f GB, %d files); only what changed is copied again\n", cfg.Path, cfg.Snapshot(), float64(m.Bytes)/(1<<30), m.Files)
	if err := e.robocopy(ctx, out, playnite.RobocopyArgs(cfg.Path, cfg.Snapshot(), exclude, true)); err != nil {
		return m, err
	}
	// robocopy does not delete what is excluded now (the caches)
	for _, x := range exclude {
		if snapshot.IsFilePattern(x) {
			continue
		}
		p := filepath.Join(cfg.Snapshot(), filepath.FromSlash(x))
		if exists(p) && snapshot.Inside(p, cfg.Snapshot()) && snapshot.Clean(p) != snapshot.Clean(cfg.Snapshot()) {
			if err := os.RemoveAll(p); err != nil {
				return m, err
			}
			fmt.Fprintf(out, "removed %s from the store (excluded)\n", x)
		}
	}
	if err := snapshot.WriteJSON(snapshot.SecretsFile(cfg.Store, playnite.Program), git.Secrets); err != nil {
		return m, err
	}
	for _, t := range in.Themes {
		if t.Active && t.Mode == "desktop" {
			m.DesktopTheme = t.Name
		} else if t.Active {
			m.FullscreenTheme = t.Name
		}
	}
	for _, x := range in.Extensions {
		m.Extensions = append(m.Extensions, x.Name+" "+x.Version)
	}
	return m, playnite.WriteManifest(cfg.Store, m)
}

// RestorePlaynite puts Playnite back.
//
// With the store (the HD): its copy goes to the Playnite folder (program,
// library, media, themes, extensions), the settings of git on top with the
// keys of the store, and the paths are fixed when the folders moved.
//
// Without it (no store set, the drive is not there or has no snapshot):
// the program is downloaded at the version of config/playnite.yaml, the
// add-ons of windows/playnite/addons.yaml come from the Playnite add-on
// database, and git brings the settings and the library databases (but the
// games, imported again by the library extensions and the scanners).
//
// mirror also deletes from the folder what the store copy does not have.
func (e *Env) RestorePlaynite(ctx context.Context, out io.Writer, mirror bool) error {
	cfg, _, err := e.PlayniteConfig()
	if err != nil {
		return err
	}
	var m *playnite.Manifest
	if cfg.Store != "" {
		if m, err = playnite.ReadManifest(cfg.Store); err != nil {
			return err
		}
		if m != nil && !exists(filepath.Join(cfg.Snapshot(), playnite.Exe)) {
			return fmt.Errorf("the snapshot in %s is incomplete: %s is missing", cfg.Snapshot(), playnite.Exe)
		}
	}
	if err := e.playniteClosed(ctx); err != nil {
		return err
	}
	gitDir := playnite.GitDir(e.Root)
	patterns := cfg.Git
	var secrets []snapshot.Secret
	if m != nil {
		fmt.Fprintf(out, "copying the snapshot of %s (%s) to %s\n", m.SavedAt.Local().Format("2006-01-02 15:04"), m.Computer, cfg.Path)
		if err := e.robocopy(ctx, out, playnite.RobocopyArgs(cfg.Snapshot(), cfg.Path, cfg.StoreExclude(), mirror)); err != nil {
			return err
		}
		// the databases of the store match its games.db: git does not overwrite them
		patterns = snapshot.Without(cfg.Git, playnite.LibraryDir)
		if secrets, err = snapshot.ReadSecrets(snapshot.SecretsFile(cfg.Store, playnite.Program)); err != nil {
			return err
		}
	} else {
		if cfg.Store == "" {
			fmt.Fprintln(out, "no store is set: Playnite is installed from the internet and git")
		} else {
			fmt.Fprintf(out, "%s has no Playnite snapshot (the drive is not there?): Playnite is installed from the internet and git\n", cfg.Store)
		}
		if err := e.installPlayniteFresh(ctx, out, cfg); err != nil {
			return err
		}
	}
	if len(snapshot.ProfileFiles(gitDir, patterns)) > 0 {
		if e.Guest {
			fmt.Fprintln(out, "configuration of someone else: account ids are left out of the settings")
		}
		res, err := snapshot.ImportWith(gitDir, cfg.Path, patterns, secrets, e.Guest)
		if err != nil {
			return fmt.Errorf("copy the settings from %s: %w", gitDir, err)
		}
		fmt.Fprintf(out, "from git: %d files, %d updated, %d keys or tokens put back\n", res.Files, res.Changed, res.Secrets)
	}
	if m != nil {
		changed, err := playnite.RewritePaths(cfg.Path, m.Source, cfg.Path)
		if err != nil {
			return fmt.Errorf("point the settings at %s: %w", cfg.Path, err)
		}
		if len(changed) > 0 {
			fmt.Fprintf(out, "%s replaced by %s in %d settings files\n", m.Source, cfg.Path, len(changed))
		}
		// the library keeps full paths to RetroArch and its games
		if ra, ok, err := e.RetroArchConfig(); err == nil && ok {
			if err := e.RelocatePlaynite(ctx, out, m.RetroArch, ra.Path); err != nil {
				return err
			}
			if err := e.RelocatePlaynite(ctx, out, m.RetroGames, ra.Games); err != nil {
				return err
			}
		}
	}
	if cfg.StartMenu {
		return e.registerPlaynite(ctx, out, cfg)
	}
	return nil
}

// installPlayniteFresh downloads the portable Playnite and the add-ons of
// windows/playnite/addons.yaml. Add-ons the database does not have are
// listed at the end: they were installed by hand.
func (e *Env) installPlayniteFresh(ctx context.Context, out io.Writer, cfg playnite.Config) error {
	list, err := playnite.ReadAddons(playnite.AddonsFile(e.Root))
	if err != nil {
		return err
	}
	version := cfg.Version
	if version == "" {
		version = list.Playnite
	}
	if !exists(filepath.Join(cfg.Path, playnite.Exe)) {
		if version == "" {
			return fmt.Errorf("config/playnite.yaml has no version to download: save once with Playnite installed, or set it by hand")
		}
		tmp, err := os.MkdirTemp("", "envsetup-playnite-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		archive := filepath.Join(tmp, "Playnite.7z")
		fmt.Fprintf(out, "downloading Playnite %s\n", version)
		if err := snapshot.Download(ctx, playnite.ProgramURL(version), archive, out); err != nil {
			return err
		}
		if err := os.MkdirAll(cfg.Path, 0o755); err != nil {
			return err
		}
		// the bsdtar of Windows reads 7z (Git's tar does not)
		tar := filepath.Join(os.Getenv("SystemRoot"), "System32", "tar.exe")
		if !exists(tar) {
			tar = "tar"
		}
		if err := e.Runner.Run(ctx, out, tar, "-xf", archive, "-C", cfg.Path); err != nil {
			return fmt.Errorf("extract %s: %w", archive, err)
		}
	}
	var todo []playnite.Addon
	for _, a := range list.Addons {
		if !a.Builtin && !exists(filepath.Join(cfg.Path, filepath.FromSlash(a.Folder))) {
			todo = append(todo, a)
		}
	}
	if len(todo) == 0 {
		return nil
	}
	catalog, err := playnite.Catalog(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "installing %d themes and extensions from the Playnite add-on database\n", len(todo))
	var manual []string
	for _, a := range todo {
		url := catalog[strings.ToLower(a.ID)]
		if url == "" {
			manual = append(manual, a.Name)
			continue
		}
		got, err := playnite.InstallAddon(ctx, a, url, cfg.Path, out)
		if err != nil {
			fmt.Fprintf(out, "  %s: %v\n", a.Name, err)
			manual = append(manual, a.Name)
			continue
		}
		note := ""
		if got != a.Version {
			note = fmt.Sprintf(" (was %s)", a.Version)
		}
		fmt.Fprintf(out, "  %s %s%s\n", a.Name, got, note)
	}
	if len(manual) > 0 {
		fmt.Fprintf(out, "not in the add-on database, install by hand: %s\n", strings.Join(manual, ", "))
	}
	return nil
}

func (e *Env) registerPlaynite(ctx context.Context, out io.Writer, cfg playnite.Config) error {
	script := filepath.Join(e.Root, "windows", "register-portable-app.ps1")
	cmd := fmt.Sprintf("& %s -Path %s -Name 'Playnite' -Publisher 'Playnite'", psQuote(script), psQuote(filepath.Join(cfg.Path, playnite.Exe)))
	if err := e.Runner.Run(ctx, out, e.script(cmd)...); err != nil {
		return fmt.Errorf("register Playnite in the Start menu: %w", err)
	}
	return nil
}

// FixPlaynite takes care of one requirement (see playnite.Requirements).
func (e *Env) FixPlaynite(ctx context.Context, out io.Writer, id string) error {
	cfg, _, err := e.PlayniteConfig()
	if err != nil {
		return err
	}
	switch id {
	case playnite.ReqMediaPlayer:
		return e.InstallMediaPlayer(ctx, out)
	case playnite.ReqStartMenu:
		if _, err := os.Stat(filepath.Join(cfg.Path, playnite.Exe)); err != nil {
			return fmt.Errorf("%s has no %s", cfg.Path, playnite.Exe)
		}
		return e.registerPlaynite(ctx, out, cfg)
	case playnite.ReqFfmpegNormalize:
		var old string
		for _, x := range playnite.Externals(cfg, playnite.Read(cfg, false).Extensions) {
			if strings.EqualFold(filepath.Base(x.Path), "ffmpeg-normalize.exe") {
				old = x.Path
			}
		}
		fmt.Fprintln(out, "installing ffmpeg-normalize with pip")
		res, err := e.Runner.Output(ctx, e.script(playnite.FfmpegNormalizeScript)...)
		if err != nil {
			return fmt.Errorf("install ffmpeg-normalize: %w", err)
		}
		lines := strings.Split(strings.TrimSpace(res), "\n")
		exe := strings.TrimSpace(lines[len(lines)-1])
		if old != "" && !strings.EqualFold(old, exe) {
			if err := e.playniteClosed(ctx); err != nil {
				return fmt.Errorf("ffmpeg-normalize is installed in %s, but the extension still points to %s: %w", exe, old, err)
			}
			if _, err := playnite.SetExternal(cfg, old, exe); err != nil {
				return err
			}
			fmt.Fprintf(out, "the extension now points to %s\n", exe)
		}
		return nil
	}
	return fmt.Errorf("unknown requirement %q", id)
}

// InstallMediaPlayer adds the Windows Media Player capability (one UAC
// prompt). Playnite plays trailers with it; the answer file used to remove it.
func (e *Env) InstallMediaPlayer(ctx context.Context, out io.Writer) error {
	dir, err := os.MkdirTemp("", "envsetup-wmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	script, err := writeScript(dir, "wmp.ps1", playnite.MediaPlayerScript)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "installing Windows Media Player from Windows Update: accept the elevation prompt")
	if err := e.Runner.Run(ctx, out, e.script(policy.ElevatedCommand(script))...); err != nil {
		return fmt.Errorf("the elevated script failed or the elevation was refused: %w", err)
	}
	return nil
}

// playniteInstalled is the check of the catalogue method playnite.
func (e *Env) playniteInstalled() bool {
	cfg, _, err := e.PlayniteConfig()
	if err != nil {
		return false
	}
	if _, err = os.Stat(filepath.Join(cfg.Path, playnite.Exe)); err != nil {
		return false
	}
	// an app of the system: found by Windows search and pinnable to the taskbar
	_, err = os.Stat(playnite.ThisHost().StartMenuLink())
	return !cfg.StartMenu || err == nil
}

// playniteInstall restores the snapshot and fixes what the folder does not
// carry and can be fixed without questions. When Playnite is already in its
// folder and only the Start menu entry is missing, it only adds that.
func (e *Env) playniteInstall(ctx context.Context, out io.Writer) error {
	if cfg, _, err := e.PlayniteConfig(); err == nil && cfg.StartMenu {
		_, errExe := os.Stat(filepath.Join(cfg.Path, playnite.Exe))
		_, errLink := os.Stat(playnite.ThisHost().StartMenuLink())
		if errExe == nil && errLink != nil {
			fmt.Fprintf(out, "Playnite is in %s: adding it to the Start menu and to Installed apps\n", cfg.Path)
			return e.registerPlaynite(ctx, out, cfg)
		}
	}
	if err := e.RestorePlaynite(ctx, out, false); err != nil {
		return err
	}
	cfg, _, err := e.PlayniteConfig()
	if err != nil {
		return err
	}
	for _, r := range playnite.Requirements(cfg, playnite.Read(cfg, false), playnite.ThisHost()) {
		if !r.OK && r.ID == playnite.ReqMediaPlayer {
			if err := e.InstallMediaPlayer(ctx, out); err != nil {
				return err
			}
		}
	}
	return nil
}
