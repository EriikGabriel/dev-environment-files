package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/windhawk"
)

// windhawkJSON is part of the contract with the desktop app; see
// app/src/types.ts.
type windhawkJSON struct {
	// Path is the export, relative to the repo.
	Path string `json:"path"`
	// Exists is false until an export is in the repo.
	Exists  bool `json:"exists"`
	Windows bool `json:"windows"`
	// CLI is windhawk-cli.exe ("" when Windhawk 2 is not installed).
	CLI string `json:"cli"`
	// Read is false when the machine could not be compared (Error says why).
	Read  bool   `json:"read"`
	Error string `json:"error,omitempty"`
	// AppSettings are Windhawk's own settings in the repo.
	AppSettings        map[string]any       `json:"app_settings"`
	AppSettingsChanged bool                 `json:"app_settings_changed"`
	Mods               []windhawk.ModStatus `json:"mods"`
	Pending            bool                 `json:"pending"`
	Warnings           []string             `json:"warnings"`
}

func describeWindhawk(cmd *cobra.Command, f flags) (windhawkJSON, error) {
	_, env, err := load(f)
	if err != nil {
		return windhawkJSON{}, err
	}
	path := windhawk.BackupPath(env.Root)
	rel, _ := filepath.Rel(env.Root, path)
	out := windhawkJSON{Path: filepath.ToSlash(rel), Windows: env.Info.OS == "windows", CLI: windhawk.CLI(), AppSettings: map[string]any{}, Warnings: []string{}}
	want, err := windhawk.Load(path)
	if err != nil {
		return out, err
	}
	out.Exists = want != nil
	if want != nil && want.AppSettings != nil {
		out.AppSettings = want.AppSettings
	}
	var have *windhawk.Backup
	if out.Windows {
		if have, err = env.WindhawkCurrent(cmd.Context()); err != nil {
			out.Error = err.Error()
		} else {
			out.Read = true
		}
	}
	out.Mods, out.AppSettingsChanged = windhawk.Compare(want, have)
	out.Pending = windhawk.Pending(out.Mods)
	return out, nil
}

// windhawkCommands keep the Windhawk mods in the repo (windows/windhawk/backup.json)
// and turn the dock of the mod Ilha e Dock estilo Mac on and off.
func windhawkCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)

	status := jsonFlag(&cobra.Command{
		Use:   "windhawk",
		Short: "Compare the Windhawk export of the repo with the mods installed on this machine",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := describeWindhawk(cmd, *f)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, d)
			}
			if !d.Exists {
				fmt.Printf("%s is missing: run windhawk-export\n", d.Path)
			}
			if d.Error != "" {
				fmt.Println(d.Error)
			}
			for _, m := range d.Mods {
				fmt.Printf("  %-8s %-45s %s\n", m.State, m.ID, m.Version)
				for _, c := range m.Changes {
					fmt.Printf("           %s\n", c)
				}
			}
			for _, w := range d.Warnings {
				fmt.Println("warning:", w)
			}
			return nil
		},
	})

	export := jsonFlag(&cobra.Command{
		Use:   "windhawk-export",
		Short: "Replace windows/windhawk/backup.json with the mods and settings Windhawk has on this machine",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, env, err := load(*f)
			if err != nil {
				return err
			}
			b, err := env.ExportWindhawk(cmd.Context())
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"mods": len(b.Mods)})
			}
			fmt.Printf("%d mods exported to %s\n", len(b.Mods), windhawk.BackupPath(env.Root))
			return nil
		},
	})

	imp := jsonFlag(&cobra.Command{
		Use:   "windhawk-import",
		Short: "Install the mods of windows/windhawk/backup.json with their settings (asks for elevation once)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, env, err := load(*f)
			if err != nil {
				return err
			}
			if env.Info.OS != "windows" {
				return fmt.Errorf("Windhawk only exists on Windows")
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr
			}
			if err := env.ImportWindhawk(cmd.Context(), out); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"imported": true})
			}
			fmt.Println("Windhawk mods and settings imported")
			return nil
		},
	})

	var dockRestore bool
	taskbarDock := jsonFlag(&cobra.Command{
		Use:   "taskbar-dock",
		Short: "Turn on the dock of the Windhawk mod Ilha e Dock estilo Mac (mac-island-dock), or undo it with --restore",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, env, err := load(*f)
			if err != nil {
				return err
			}
			if env.Info.OS != "windows" {
				return fmt.Errorf("the taskbar dock is for Windows")
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr
			}
			if err := env.TaskbarDock(cmd.Context(), out, dockRestore); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"dock": env.DockApplied(cmd.Context())})
			}
			return nil
		},
	})
	taskbarDock.Flags().BoolVar(&dockRestore, "restore", false, "put back the taskbar mode the mod had before")

	return []*cobra.Command{status, export, imp, taskbarDock}
}
