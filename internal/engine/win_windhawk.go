package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/policy"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/windhawk"
)

// windhawkCLI finds windhawk-cli.exe; WindhawkCLI replaces it in tests.
func (e *Env) windhawkCLI() (string, error) {
	if e.WindhawkCLI != "" {
		return e.WindhawkCLI, nil
	}
	if p := windhawk.CLI(); p != "" {
		return p, nil
	}
	// winget only has Windhawk 1.x, which has no CLI: the catalogue item
	// "windhawk" installs the newest Windhawk 2 from GitHub instead.
	return "", fmt.Errorf("windhawk-cli.exe was not found: it only exists from Windhawk 2 on. Install the item Windhawk (step Aparência) or run %s",
		filepath.Join(e.Root, "windows", "install-windhawk.ps1"))
}

// exportWindhawk exports what Windhawk has on this machine, without
// elevation, and returns the file as written and parsed.
func (e *Env) exportWindhawk(ctx context.Context) ([]byte, *windhawk.Backup, error) {
	cli, err := e.windhawkCLI()
	if err != nil {
		return nil, nil, err
	}
	dir, err := os.MkdirTemp("", "envsetup-windhawk-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "current.json")
	if out, err := e.Runner.Output(ctx, cli, "--json", "--quiet", "data", "export", "--out", file, "--force"); err != nil {
		return nil, nil, fmt.Errorf("export the Windhawk settings: %s", windhawk.CLIError(out+" "+err.Error()))
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	b, err := windhawk.Parse(raw)
	return raw, b, err
}

// WindhawkCurrent is what Windhawk has on this machine, to compare it with
// the repo.
func (e *Env) WindhawkCurrent(ctx context.Context) (*windhawk.Backup, error) {
	_, b, err := e.exportWindhawk(ctx)
	return b, err
}

// ExportWindhawk replaces the export of the repo with what Windhawk has on
// this machine.
func (e *Env) ExportWindhawk(ctx context.Context) (*windhawk.Backup, error) {
	raw, b, err := e.exportWindhawk(ctx)
	if err != nil {
		return nil, err
	}
	dest := windhawk.BackupPath(e.Root)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, err
	}
	return b, os.WriteFile(dest, raw, 0o644)
}

// runElevated runs windhawk-cli calls in one elevated script (one UAC
// prompt) and turns a failure into the CLI's own message.
func (e *Env) runElevated(ctx context.Context, out io.Writer, what string, calls ...[]string) error {
	cli, err := e.windhawkCLI()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "envsetup-windhawk-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	log := filepath.Join(dir, "cli.log")
	script, err := writeScript(dir, "windhawk.ps1", windhawk.ElevatedScript(cli, log, calls...))
	if err != nil {
		return err
	}
	runErr := e.Runner.Run(ctx, out, e.script(policy.ElevatedCommand(script))...)
	stdout, stderr := windhawk.ReadLogs(log, len(calls))
	if s := strings.TrimSpace(stderr); s != "" {
		fmt.Fprintln(out, s)
	}
	if runErr != nil {
		msg := ""
		if strings.TrimSpace(stdout+stderr) != "" {
			msg = windhawk.CLIError(stdout + "\n" + stderr)
		}
		if msg == "" {
			msg = runErr.Error() + " (or the elevation was refused)"
		}
		return fmt.Errorf("%s failed: %s", what, msg)
	}
	return nil
}

// ImportWindhawk installs the mods of the repo's export and applies their
// settings and Windhawk's (one UAC prompt). Mods installed and not in the
// export are left alone.
func (e *Env) ImportWindhawk(ctx context.Context, out io.Writer) error {
	if _, err := e.windhawkCLI(); err != nil {
		return err
	}
	file := windhawk.BackupPath(e.Root)
	b, err := windhawk.Load(file)
	if err != nil {
		return err
	}
	if b == nil {
		return fmt.Errorf("%s is missing: export the settings from Windhawk first", file)
	}
	fmt.Fprintf(out, "installing %d Windhawk mods and their settings: accept the elevation prompt\n", len(b.Mods))
	return e.runElevated(ctx, out, "the Windhawk import", windhawk.ImportArgs(file))
}

// DockMode is the taskbar mode of the mod "Ilha e Dock estilo Mac" on this
// machine; "" when the mod is not installed.
func (e *Env) DockMode(ctx context.Context) (string, error) {
	cli, err := e.windhawkCLI()
	if err != nil {
		return "", err
	}
	res, err := e.Runner.Output(ctx, cli, "--json", "mod", "settings", "get", windhawk.DockMod, windhawk.DockSetting)
	if strings.Contains(res, "MOD_NOT_INSTALLED") {
		return "", nil
	}
	if err != nil && strings.TrimSpace(res) == "" {
		return "", fmt.Errorf("read the dock mode: %w", err)
	}
	v, perr := windhawk.ParseSetting(res, windhawk.DockSetting)
	if perr != nil {
		return "", fmt.Errorf("read the dock mode: %w", perr)
	}
	mode, _ := v.(string)
	if mode == "" {
		mode = windhawk.DockOff // the mod's default
	}
	return mode, nil
}

// DockApplied tells whether the taskbar is the dock of the mod.
func (e *Env) DockApplied(ctx context.Context) bool {
	m, err := e.DockMode(ctx)
	return err == nil && m == windhawk.DockOn
}

// dockMemory keeps the mode the taskbar had before TaskbarDock turned the
// dock on, to give it back.
func (e *Env) dockMemory() string {
	dir := e.StateDir
	if dir == "" {
		dir = StateDir()
	}
	return filepath.Join(dir, "windhawk-dock-previous")
}

// TaskbarDock turns the dock of "Ilha e Dock estilo Mac" on (installing the
// mod when it is missing) or, with restore, puts back the taskbar mode it had
// before (the Windows taskbar when unknown). One UAC prompt.
func (e *Env) TaskbarDock(ctx context.Context, out io.Writer, restore bool) error {
	mode, err := e.DockMode(ctx)
	if err != nil {
		return err
	}
	if restore {
		prev := windhawk.DockOff
		if raw, err := os.ReadFile(e.dockMemory()); err == nil && strings.TrimSpace(string(raw)) != "" {
			prev = strings.TrimSpace(string(raw))
		}
		if mode == "" {
			os.Remove(e.dockMemory())
			fmt.Fprintln(out, "the mod is not installed: the taskbar is already the Windows one")
			return nil
		}
		fmt.Fprintf(out, "taskbar mode back to %s: accept the elevation prompt\n", prev)
		if err := e.runElevated(ctx, out, "undoing the dock", windhawk.SetDockArgs(prev)); err != nil {
			return err
		}
		os.Remove(e.dockMemory())
		return nil
	}
	var calls [][]string
	if mode == "" {
		fmt.Fprintln(out, "installing the mod Ilha e Dock estilo Mac (mac-island-dock) from the Windhawk repository")
		calls = append(calls, windhawk.InstallDockArgs())
	} else if mode != windhawk.DockOn {
		if err := os.MkdirAll(filepath.Dir(e.dockMemory()), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(e.dockMemory(), []byte(mode), 0o644); err != nil {
			return err
		}
	}
	calls = append(calls, windhawk.SetDockArgs(windhawk.DockOn))
	fmt.Fprintln(out, "turning the dock on: accept the elevation prompt")
	return e.runElevated(ctx, out, "turning the dock on", calls...)
}

// windhawkApplied is the check of the catalogue method windhawk: every mod
// of the export is installed as it is there.
func (e *Env) windhawkApplied(ctx context.Context) bool {
	want, err := windhawk.Load(windhawk.BackupPath(e.Root))
	if err != nil || want == nil {
		return false
	}
	have, err := e.WindhawkCurrent(ctx)
	if err != nil {
		return false
	}
	mods, _ := windhawk.Compare(want, have)
	return !windhawk.Pending(mods)
}
