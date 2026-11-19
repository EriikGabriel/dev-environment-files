package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/retroarch"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// The JSON shapes below are part of the contract with the desktop app; see
// app/src/types/retroarch.ts.

type retroarchSnapshotJSON struct {
	// Path and Games are where the copies are (store\RetroArch, store\Games).
	Path  string `json:"path"`
	Games string `json:"games"`
	// Manifest is nil when no snapshot was saved in the store yet.
	Manifest *retroarch.Manifest `json:"manifest"`
	Error    string              `json:"error,omitempty"`
}

type retroarchJSON struct {
	Config retroarch.Config `json:"config"`
	// Saved is false until config/retroarch.yaml exists.
	Saved   bool              `json:"saved"`
	Windows bool              `json:"windows"`
	Install retroarch.Install `json:"install"`
	// Snapshot is nil when no store is chosen.
	Snapshot     *retroarchSnapshotJSON  `json:"snapshot"`
	Running      bool                    `json:"running"`
	Requirements []retroarch.Requirement `json:"requirements"`
	// Git is the personal part kept in the repo (windows/retroarch).
	Git snapshot.Git `json:"git"`
	// ProgramDirs are the folders a restore downloads (without copy_program).
	ProgramDirs []string `json:"program_dirs"`
}

func describeRetroArch(cmd *cobra.Command, f flags, sizes bool) (retroarchJSON, error) {
	_, env, err := load(f)
	if err != nil {
		return retroarchJSON{}, err
	}
	cfg, exists, err := env.RetroArchConfig()
	if err != nil {
		return retroarchJSON{}, err
	}
	out := retroarchJSON{Config: cfg, Saved: exists, Windows: env.Info.OS == "windows", Requirements: []retroarch.Requirement{}, ProgramDirs: retroarch.ProgramDirs}
	out.Install = retroarch.Read(cfg, sizes)
	out.Git = snapshot.DescribeGit(env.Root, filepath.Join("windows", "retroarch"), cfg.Path, cfg.Git)
	if cfg.Store != "" {
		s := &retroarchSnapshotJSON{Path: cfg.Snapshot(), Games: cfg.GamesSnapshot()}
		if s.Manifest, err = retroarch.ReadManifest(cfg.Store); err != nil {
			s.Error = err.Error()
		}
		out.Snapshot = s
	}
	if out.Windows {
		out.Running = env.RetroArchRunning(cmd.Context())
		out.Requirements = retroarch.Requirements(cfg, out.Install)
	}
	return out, nil
}

func retroarchCommands(f *flags) []*cobra.Command {
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
	// progress goes to stderr when stdout carries the JSON answer
	progress := func() *os.File {
		if f.json {
			return os.Stderr
		}
		return os.Stdout
	}

	var sizes bool
	status := jsonFlag(&cobra.Command{
		Use:   "retroarch",
		Short: "Show the portable RetroArch: cores, games, playlists, the snapshot and what needs fixing",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := describeRetroArch(cmd, *f, sizes)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, d)
			}
			fmt.Printf("folder: %s (found: %v)\n", d.Config.Path, d.Install.Found)
			fmt.Printf("games:  %s (%d games, %.1f GB)\n", d.Config.Games, d.Install.GameCount, float64(d.Install.GameBytes)/(1<<30))
			if sizes {
				fmt.Printf("RetroArch copy: %.1f GB in %d files\n", float64(d.Install.Bytes)/(1<<30), d.Install.Files)
			}
			for _, c := range d.Install.Cores {
				fmt.Printf("  core  %s (%s)\n", c.Name, c.File)
			}
			for _, p := range d.Install.Playlists {
				fmt.Printf("  list  %s: %d games, %d not found\n", p.Name, p.Items, p.Missing)
			}
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
				fmt.Printf("  %s %s: %s\n", mark, r.Title, r.Detail)
			}
			return nil
		},
	})
	status.Flags().BoolVar(&sizes, "sizes", false, "also measure the RetroArch folder (walks every file)")

	var (
		pPath, pGames, pStore, pExclude string
		pStartMenu, pCopyProgram, pCopyGames bool
	)
	set := jsonFlag(&cobra.Command{
		Use:   "set-retroarch",
		Short: "Save the RetroArch settings in config/retroarch.yaml",
		Long:  "Only the flags that are given change. --exclude takes names separated by commas; an empty --games leaves the games out.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			path := retroarch.Path(root)
			cfg, _, err := retroarch.Load(path)
			if err != nil {
				return err
			}
			cleanOrEmpty := func(p string) string {
				if p == "" {
					return ""
				}
				return filepath.Clean(p)
			}
			ch := cmd.Flags().Changed
			if ch("path") {
				cfg.Path = filepath.Clean(pPath)
			}
			if ch("games") {
				cfg.Games = cleanOrEmpty(pGames)
			}
			if ch("store") {
				cfg.Store = cleanOrEmpty(pStore)
			}
			if ch("exclude") {
				cfg.Exclude = splitList(pExclude)
			}
			if ch("start-menu") {
				cfg.StartMenu = pStartMenu
			}
			if ch("copy-games") {
				cfg.CopyGames = pCopyGames
			}
			if ch("copy-program") {
				cfg.CopyProgram = pCopyProgram
			}
			if err := retroarch.Save(path, cfg); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, cfg)
			}
			fmt.Printf("RetroArch settings saved in %s\n", path)
			return nil
		},
	})
	set.Flags().StringVar(&pPath, "path", "", `the RetroArch folder (C:\Emuladores\RetroArch)`)
	set.Flags().StringVar(&pGames, "games", "", `the games folder (D:\Games\Retro); empty leaves the games out`)
	set.Flags().StringVar(&pStore, "store", "", "folder for the snapshot; empty forgets it")
	set.Flags().StringVar(&pExclude, "exclude", "", "folders (relative to the RetroArch folder) and file patterns left out, separated by commas")
	set.Flags().BoolVar(&pStartMenu, "start-menu", true, "register RetroArch in the Start menu after a restore")
	set.Flags().BoolVar(&pCopyProgram, "copy-program", false, "also keep the whole program in the store, so a restore does not download it")
	set.Flags().BoolVar(&pCopyGames, "copy-games", false, "also copy the games to the store on each save")

	save := jsonFlag(&cobra.Command{
		Use:   "retroarch-save",
		Short: "Copy the settings and saves to git (windows/retroarch) and the BIOS and games to the store; RetroArch must be closed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := windowsEnv("the RetroArch snapshot")
			if err != nil {
				return err
			}
			m, err := env.SaveRetroArch(cmd.Context(), progress())
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, m)
			}
			fmt.Printf("saved: RetroArch %s, %.2f GB in the store, games %.1f GB; commit windows/retroarch and config/retroarch.yaml\n", m.Version, float64(m.Bytes)/(1<<30), float64(m.GameBytes)/(1<<30))
			return nil
		},
	})

	var mirror bool
	restore := jsonFlag(&cobra.Command{
		Use:   "retroarch-restore",
		Short: "Put RetroArch back: download it (or copy it from the store), the BIOS and games from the store, the settings from git",
		Long:  "Files in the folders that the store does not have are kept, unless --mirror is given (with copy_program only).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := windowsEnv("the RetroArch snapshot")
			if err != nil {
				return err
			}
			if err := env.RestoreRetroArch(cmd.Context(), progress(), mirror); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"restored": true})
			}
			fmt.Println("RetroArch restored")
			return nil
		},
	})
	restore.Flags().BoolVar(&mirror, "mirror", false, "also delete from the folders what the snapshot does not have")

	fix := jsonFlag(&cobra.Command{
		Use:   "retroarch-fix <playlists|start-menu>",
		Short: "Point the playlists at the games again, or add RetroArch to the Start menu",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := windowsEnv("this fix")
			if err != nil {
				return err
			}
			if err := env.FixRetroArch(cmd.Context(), progress(), args[0]); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"fixed": args[0]})
			}
			return nil
		},
	})

	open := jsonFlag(&cobra.Command{
		Use:   "retroarch-open <folder|games|store|app>",
		Short: "Open the RetroArch folder, the games or the store in the file manager, or start RetroArch",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			cfg, _, err := retroarch.Load(retroarch.Path(root))
			if err != nil {
				return err
			}
			name := "xdg-open"
			if runtime.GOOS == "windows" {
				name = "explorer.exe"
			}
			var target string
			switch args[0] {
			case "folder":
				target = cfg.Path
			case "games":
				target = cfg.Games
			case "store":
				target = cfg.Store
			case "app":
				target = filepath.Join(cfg.Path, retroarch.Exe)
				name = target
			default:
				return fmt.Errorf("unknown target %q", args[0])
			}
			if target == "" {
				return fmt.Errorf("no %s is set in config/retroarch.yaml", args[0])
			}
			if _, err := os.Stat(target); err != nil {
				return fmt.Errorf("%s does not exist", target)
			}
			var c *exec.Cmd
			if args[0] == "app" {
				c = exec.Command(name)
				c.Dir = cfg.Path
			} else {
				c = exec.Command(name, target)
			}
			if err := c.Start(); err != nil {
				return err
			}
			go c.Wait()
			if f.json {
				return printJSON(os.Stdout, map[string]any{"opened": target})
			}
			return nil
		},
	})

	move := jsonFlag(&cobra.Command{
		Use:   "retroarch-move-games <folder>",
		Short: "Move the games to another folder and point RetroArch's playlists and the Playnite library at it",
		Long:  "The files are copied and checked before the old folder is deleted. RetroArch and Playnite must be closed.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := windowsEnv("moving the games")
			if err != nil {
				return err
			}
			if err := env.MoveRetroArchGames(cmd.Context(), progress(), args[0]); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"games": filepath.Clean(args[0])})
			}
			return nil
		},
	})

	return []*cobra.Command{status, set, save, restore, fix, open, move}
}
