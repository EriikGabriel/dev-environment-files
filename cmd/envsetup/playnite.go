package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/playnite"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// The JSON shapes below are part of the contract with the desktop app; see
// app/src/types.ts.

type playniteSnapshotJSON struct {
	// Path is where the copy is (store\Playnite).
	Path string `json:"path"`
	// Manifest is nil when no snapshot was saved in the store yet.
	Manifest *playnite.Manifest `json:"manifest"`
	Error    string             `json:"error,omitempty"`
}

type playniteJSON struct {
	Config playnite.Config `json:"config"`
	// Saved is false until config/playnite.yaml exists.
	Saved   bool             `json:"saved"`
	Windows bool             `json:"windows"`
	Install playnite.Install `json:"install"`
	// Snapshot is nil when no store is chosen.
	Snapshot     *playniteSnapshotJSON  `json:"snapshot"`
	Running      []string               `json:"running"`
	Requirements []playnite.Requirement `json:"requirements"`
	// Git is the settings kept in the repo (windows/playnite).
	Git snapshot.Git `json:"git"`
}

func describePlaynite(cmd *cobra.Command, f flags, sizes bool) (playniteJSON, error) {
	_, env, err := load(f)
	if err != nil {
		return playniteJSON{}, err
	}
	cfg, exists, err := env.PlayniteConfig()
	if err != nil {
		return playniteJSON{}, err
	}
	out := playniteJSON{Config: cfg, Saved: exists, Windows: env.Info.OS == "windows", Running: []string{}, Requirements: []playnite.Requirement{}}
	out.Install = playnite.Read(cfg, sizes)
	out.Git = snapshot.DescribeGit(env.Root, filepath.Join("windows", "playnite"), cfg.Path, cfg.Git)
	if cfg.Store != "" {
		s := &playniteSnapshotJSON{Path: cfg.Snapshot()}
		if s.Manifest, err = playnite.ReadManifest(cfg.Store); err != nil {
			s.Error = err.Error()
		}
		out.Snapshot = s
	}
	if out.Windows {
		if r := env.PlayniteRunning(cmd.Context()); r != nil {
			out.Running = r
		}
		out.Requirements = playnite.Requirements(cfg, out.Install, playnite.ThisHost())
	}
	return out, nil
}

func playniteCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)
	windowsEnv := func(what string) (*engine.Env, error) {
		_, env, err := load(*f)
		if err != nil {
			return nil, err
		}
		if env.Info.OS != "windows" {
			return nil, fmt.Errorf("%s only works on Windows", what)
		}
		return env, nil
	}

	var sizes bool
	status := jsonFlag(&cobra.Command{
		Use:   "playnite",
		Short: "Show the portable Playnite: themes, extensions, the snapshot and what it needs outside its folder",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := describePlaynite(cmd, *f, sizes)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, d)
			}
			fmt.Printf("folder: %s (found: %v)\n", d.Config.Path, d.Install.Found)
			if sizes {
				fmt.Printf("snapshot size: %.1f GB in %d files\n", float64(d.Install.Bytes)/(1<<30), d.Install.Files)
			}
			for _, t := range d.Install.Themes {
				mark := " "
				if t.Active {
					mark = "*"
				}
				fmt.Printf("  %s %-10s %s %s\n", mark, t.Mode, t.Name, t.Version)
			}
			fmt.Printf("%d extensions\n", len(d.Install.Extensions))
			if d.Snapshot == nil {
				fmt.Println("store: not chosen")
			} else if d.Snapshot.Manifest == nil {
				fmt.Printf("store: %s (no snapshot yet)\n", d.Config.Store)
			} else {
				m := d.Snapshot.Manifest
				fmt.Printf("snapshot: %s, from %s on %s\n", m.SavedAt.Local().Format("2006-01-02 15:04"), m.Source, m.Computer)
			}
			for _, r := range d.Requirements {
				mark := "ok "
				if !r.OK {
					mark = "!! "
				}
				fmt.Printf("  %s %s %s\n", mark, r.Title, r.Detail)
			}
			return nil
		},
	})
	status.Flags().BoolVar(&sizes, "sizes", false, "also measure the folders (walks every file)")

	var (
		pPath, pStore, pExclude string
		pStartMenu              bool
	)
	set := jsonFlag(&cobra.Command{
		Use:   "set-playnite",
		Short: "Save the Playnite settings in config/playnite.yaml",
		Long:  "Only the flags that are given change. --exclude takes names separated by commas.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			path := playnite.Path(root)
			cfg, _, err := playnite.Load(path)
			if err != nil {
				return err
			}
			ch := cmd.Flags().Changed
			if ch("path") {
				cfg.Path = filepath.Clean(pPath)
			}
			if ch("store") {
				cfg.Store = pStore
				if pStore != "" {
					cfg.Store = filepath.Clean(pStore)
				}
			}
			if ch("exclude") {
				cfg.Exclude = splitList(pExclude)
			}
			if ch("start-menu") {
				cfg.StartMenu = pStartMenu
			}
			if err := playnite.Save(path, cfg); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, cfg)
			}
			fmt.Printf("Playnite settings saved in %s\n", path)
			return nil
		},
	})
	set.Flags().StringVar(&pPath, "path", "", `the Playnite folder (C:\Playnite)`)
	set.Flags().StringVar(&pStore, "store", "", "folder for the snapshot; empty forgets it")
	set.Flags().StringVar(&pExclude, "exclude", "", "folders (relative to the Playnite folder) and file patterns left out, separated by commas")
	set.Flags().BoolVar(&pStartMenu, "start-menu", true, "register Playnite in the Start menu after a restore")

	save := jsonFlag(&cobra.Command{
		Use:   "playnite-save",
		Short: "Copy the Playnite folder to the store (only what changed); Playnite must be closed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := windowsEnv("the Playnite snapshot")
			if err != nil {
				return err
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr
			}
			m, err := env.SavePlaynite(cmd.Context(), out)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, m)
			}
			fmt.Printf("snapshot saved: %.1f GB in %d files\n", float64(m.Bytes)/(1<<30), m.Files)
			return nil
		},
	})

	var mirror bool
	restore := jsonFlag(&cobra.Command{
		Use:   "playnite-restore",
		Short: "Copy the snapshot back to the Playnite folder and register it in the Start menu",
		Long: "Files in the folder that the snapshot does not have are kept, unless --mirror is given.\n" +
			"When the folder is not the one the snapshot came from, the paths in the settings are fixed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := windowsEnv("the Playnite snapshot")
			if err != nil {
				return err
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr
			}
			if err := env.RestorePlaynite(cmd.Context(), out, mirror); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"restored": true})
			}
			fmt.Println("Playnite restored")
			return nil
		},
	})
	restore.Flags().BoolVar(&mirror, "mirror", false, "also delete from the folder what the snapshot does not have")

	fix := jsonFlag(&cobra.Command{
		Use:   "playnite-fix <media-player|start-menu|ffmpeg-normalize>",
		Short: "Take care of something Playnite needs outside its folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := windowsEnv("this fix")
			if err != nil {
				return err
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr
			}
			if err := env.FixPlaynite(cmd.Context(), out, args[0]); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"fixed": args[0]})
			}
			return nil
		},
	})

	open := jsonFlag(&cobra.Command{
		Use:   "playnite-open <folder|store>",
		Short: "Open the Playnite folder or the snapshot store in the file manager",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			cfg, _, err := playnite.Load(playnite.Path(root))
			if err != nil {
				return err
			}
			var dir string
			switch args[0] {
			case "folder":
				dir = cfg.Path
			case "store":
				dir = cfg.Store
			default:
				return fmt.Errorf("unknown target %q", args[0])
			}
			if dir == "" {
				return fmt.Errorf("no %s is set in config/playnite.yaml", args[0])
			}
			if _, err := os.Stat(dir); err != nil {
				return fmt.Errorf("%s does not exist", dir)
			}
			name := "xdg-open"
			if runtime.GOOS == "windows" {
				name = "explorer.exe"
			}
			c := exec.Command(name, dir)
			if err := c.Start(); err != nil {
				return err
			}
			go c.Wait()
			if f.json {
				return printJSON(os.Stdout, map[string]any{"opened": dir})
			}
			return nil
		},
	})

	// progress goes to stderr when stdout carries the JSON answer
	progress := func() *os.File {
		if f.json {
			return os.Stderr
		}
		return os.Stdout
	}
	closeCmd := jsonFlag(&cobra.Command{
		Use:   "playnite-close",
		Short: "Close Playnite the way its own Exit does (it saves the library first)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := windowsEnv("closing Playnite")
			if err != nil {
				return err
			}
			if err := env.ClosePlaynite(cmd.Context(), progress()); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"closed": true})
			}
			return nil
		},
	})

	var from, to string
	relocate := jsonFlag(&cobra.Command{
		Use:   "playnite-relocate",
		Short: "Rewrite the paths of the Playnite library (games, emulators, scanners) from one folder to another",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := windowsEnv("editing the Playnite library")
			if err != nil {
				return err
			}
			if err := env.RelocatePlaynite(cmd.Context(), progress(), from, to); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"from": from, "to": to})
			}
			return nil
		},
	})
	relocate.Flags().StringVar(&from, "from", "", "the old folder")
	relocate.Flags().StringVar(&to, "to", "", "the new folder")
	relocate.MarkFlagRequired("from")
	relocate.MarkFlagRequired("to")

	return []*cobra.Command{status, set, save, restore, fix, open, closeCmd, relocate}
}
