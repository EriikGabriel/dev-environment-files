package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	if err := e.Runner.Run(ctx, out, e.script(layout.FinishScript(cfg))...); err != nil {
		return err
	}
	if sum := e.layoutSum(); sum != "" {
		_ = os.MkdirAll(filepath.Dir(e.layoutMarker()), 0o755)
		_ = os.WriteFile(e.layoutMarker(), []byte(sum), 0o644)
	}
	return nil
}

// layoutMarker remembers which config/layout.yaml was last applied here: the
// shell keeps no state that tells it back reliably (a pin may also have been
// moved by hand since), so changing the file makes the item pending again.
func (e *Env) layoutMarker() string {
	dir := e.StateDir
	if dir == "" {
		dir = StateDir()
	}
	return filepath.Join(dir, "layout-applied")
}

func (e *Env) layoutSum() string {
	raw, err := os.ReadFile(layout.Path(e.Root))
	if err != nil {
		return ""
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

// layoutApplied is the check of the catalogue method layout.
func (e *Env) layoutApplied() bool {
	b, err := os.ReadFile(e.layoutMarker())
	return err == nil && string(b) == e.layoutSum() && len(b) > 0
}
