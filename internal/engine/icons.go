package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/EriikGabriel/dev-environment-files/internal/icons"
	"github.com/EriikGabriel/dev-environment-files/internal/policy"
)

// iconsHome is where the made icons and the manifest live. It is under
// ProgramData so every user can read the icons a shared shortcut points to.
func iconsHome() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "envsetup", "icons")
}

// writeScript saves a PowerShell script next to its work files. The BOM
// makes Windows PowerShell read it as UTF-8.
func writeScript(dir, name, body string) (string, error) {
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, body...), 0o600)
}

// ScanIcons finds the shortcuts of the config's scopes and saves the icon
// each one shows. It runs as the user.
func (e *Env) ScanIcons(ctx context.Context, out io.Writer, cfg icons.Config) ([]icons.Shortcut, error) {
	work := scanDir()
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	home := iconsHome()
	script, err := writeScript(work, "scan.ps1", icons.ScanScript(work, filepath.Join(home, "manifest.json"), filepath.Join(home, "ico"), cfg.Scopes))
	if err != nil {
		return nil, err
	}
	os.Remove(filepath.Join(work, "scan.json"))
	if err := e.Runner.Run(ctx, out, e.script("& "+psQuote(script))...); err != nil {
		return nil, fmt.Errorf("scan the shortcuts: %w", err)
	}
	return CachedIconScan()
}

// scanDir keeps the result of the last scan and the PNGs it extracted.
func scanDir() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	return filepath.Join(cache, "envsetup", "icons-scan")
}

// CachedIconScan reads the result of the last scan without scanning again,
// so the preview can be redrawn in another style at once. It is empty when
// there was no scan yet.
func CachedIconScan() ([]icons.Shortcut, error) {
	raw, err := os.ReadFile(filepath.Join(scanDir(), "scan.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []icons.Shortcut
	if err := json.Unmarshal(raw, &list); err != nil {
		var one icons.Shortcut
		if err2 := json.Unmarshal(raw, &one); err2 != nil {
			return nil, fmt.Errorf("unreadable scan result: %w", err)
		}
		list = []icons.Shortcut{one}
	}
	return list, nil
}

func psQuote(s string) string {
	b := []byte{'\''}
	for _, c := range []byte(s) {
		if c == '\'' {
			b = append(b, '\'')
		}
		b = append(b, c)
	}
	return string(append(b, '\''))
}

// ApplyIcons scans, draws the icons and points the shortcuts at them. The
// last step runs elevated: one UAC prompt.
func (e *Env) ApplyIcons(ctx context.Context, out io.Writer) error {
	cfg, _, err := icons.Load(icons.Path(e.Root))
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return fmt.Errorf("the icons are turned off in config/icons.yaml: turn them on first")
	}
	pack, err := icons.LoadPack(icons.Resolve(e.Root, cfg.Pack))
	if err != nil {
		return fmt.Errorf("the icon pack: %w", err)
	}
	fmt.Fprintln(out, "looking for the shortcuts")
	list, err := e.ScanIcons(ctx, out, cfg)
	if err != nil {
		return err
	}
	home := iconsHome()
	job, problems, err := icons.WriteICOs(filepath.Join(home, "ico"), cfg, e.Root, pack, list, filepath.Join(home, "manifest.json"))
	if err != nil {
		return err
	}
	for _, p := range problems {
		fmt.Fprintln(out, "skipped:", p)
	}
	if len(job.Entries) == 0 && len(job.System) == 0 {
		return fmt.Errorf("no shortcut to change was found")
	}
	dir, err := os.MkdirTemp("", "envsetup-icons-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	jobFile := filepath.Join(dir, "job.json")
	if err := os.WriteFile(jobFile, job.JSON(), 0o600); err != nil {
		return err
	}
	script, err := writeScript(dir, "apply.ps1", icons.ApplyScript(jobFile))
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "changing the icon of %d shortcuts: accept the elevation prompt\n", len(job.Entries))
	if err := e.Runner.Run(ctx, out, e.script(policy.ElevatedCommand(script))...); err != nil {
		return fmt.Errorf("the elevated script failed or the elevation was refused: %w", err)
	}
	return nil
}

// RestoreIcons gives every shortcut its original icon back.
func (e *Env) RestoreIcons(ctx context.Context, out io.Writer) error {
	dir, err := os.MkdirTemp("", "envsetup-icons-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	script, err := writeScript(dir, "restore.ps1", icons.RestoreScript(filepath.Join(iconsHome(), "manifest.json"), filepath.Join(iconsHome(), "ico")))
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "putting the Windows icons back: accept the elevation prompt")
	if err := e.Runner.Run(ctx, out, e.script(policy.ElevatedCommand(script))...); err != nil {
		return fmt.Errorf("the elevated script failed or the elevation was refused: %w", err)
	}
	return nil
}

// IconsApplied tells whether envsetup changed icons on this machine: there
// is a manifest, or icons it made are still there.
func IconsApplied() bool {
	if _, err := os.Stat(filepath.Join(iconsHome(), "manifest.json")); err == nil {
		return true
	}
	made, _ := filepath.Glob(filepath.Join(iconsHome(), "ico", "*.ico"))
	return len(made) > 0
}
