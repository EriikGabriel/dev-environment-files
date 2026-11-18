package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/layout"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/policy"
)

// ApplyLayout writes config/layout.yaml to the machine: the taskbar pins, the
// Start menu pins and the desktop icons. The policies live under HKLM, so
// one step runs elevated (one UAC prompt); the rest runs as the user.
func (e *Env) ApplyLayout(ctx context.Context, out io.Writer) error {
	cfg, _, err := layout.Load(layout.Path(e.Root))
	if err != nil {
		return err
	}
	pins := len(layout.Pinned(cfg.Taskbar)) > 0
	if pins {
		fmt.Fprintln(out, "looking for the shortcuts of the pinned apps")
		if err := e.Runner.Run(ctx, out, e.script(layout.PrepareScript(cfg))...); err != nil {
			return err
		}
	}
	dir, err := os.MkdirTemp("", "envsetup-layout-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "layout.ps1")
	// The BOM makes Windows PowerShell read the file as UTF-8.
	if err := os.WriteFile(script, append([]byte{0xEF, 0xBB, 0xBF}, layout.AdminScript(cfg)...), 0o600); err != nil {
		return err
	}
	fmt.Fprintln(out, "writing the layout policies: accept the elevation prompt")
	if err := e.Runner.Run(ctx, out, e.script(policy.ElevatedCommand(script))...); err != nil {
		return fmt.Errorf("the elevated script failed or the elevation was refused: %w", err)
	}
	fmt.Fprintln(out, "applying the layout; the taskbar restarts")
	return e.Runner.Run(ctx, out, e.script(layout.FinishScript(cfg))...)
}
