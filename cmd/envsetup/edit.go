package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
)

// editCommands change the profile and the repo: dotfiles, directories,
// the dock and the apps of each step.
func editCommands(f *flags) []*cobra.Command {
	add := &cobra.Command{
		Use:   "add <path>",
		Short: "Move a config file or directory from your home into dotfiles/ and register it",
		Long: "Copies <path> into dotfiles/, adds it to the profile's files list and, unless\n" +
			"--copy is given, replaces the original with a symlink into the repo.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			home, err := engine.DefaultHome()
			if err != nil {
				return err
			}
			prof := f.profile
			if prof == "" {
				cfg, err := config.Load(root)
				if err != nil {
					return err
				}
				if prof, err = pickProfile(cfg, &engine.Env{Info: platform.Detect()}, ""); err != nil {
					return err
				}
			}
			res, err := engine.Capture(root, home, args[0], engine.AddOptions{
				Profile: prof, Group: f.group, Link: !f.copy,
			})
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, res)
			}
			fmt.Printf("%s -> %s (%s)\n", res.Dest, res.Src, res.Mode)
			return nil
		},
	}
	add.Flags().StringVar(&f.group, "group", "", "folder under dotfiles/ (default: derived from the path)")
	add.Flags().BoolVar(&f.copy, "copy", false, "keep the original and deploy by copy instead of symlink")
	add.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	show := &cobra.Command{
		Use:   "show <path>",
		Short: "Print a file or list a directory of the repo (path relative to the root)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			v, err := engine.View(root, args[0])
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, v)
			}
			switch v.Kind {
			case "text":
				fmt.Print(v.Content)
			case "dir":
				for _, e := range v.Entries {
					fmt.Println(e.Name)
				}
			default:
				fmt.Printf("%s: %s file, %d bytes\n", v.Path, v.Kind, v.Size)
			}
			return nil
		},
	}
	show.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	var dirStep, dirOption string
	setDirs := &cobra.Command{
		Use:   "set-dirs --step <id> [--option <id>] [dir...]",
		Short: "Replace the directories a step (or one of its options) creates",
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, env, err := load(*f)
			if err != nil {
				return err
			}
			prof, err := pickProfile(cfg, env, f.profile)
			if err != nil {
				return err
			}
			path := filepath.Join(env.Root, "config", "profiles", prof+".yaml")
			if err := config.SetDirs(path, dirStep, dirOption, args); err != nil {
				return err
			}
			if _, err := config.Load(env.Root); err != nil {
				return fmt.Errorf("profile no longer loads after the edit: %w", err)
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"dirs": append([]string{}, args...)})
			}
			fmt.Printf("%d directories saved in %s\n", len(args), path)
			return nil
		},
	}
	setDirs.Flags().StringVar(&dirStep, "step", "", "step id")
	setDirs.Flags().StringVar(&dirOption, "option", "", "option id (omit for the step's own dirs)")
	setDirs.MarkFlagRequired("step")
	setDirs.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	desktop := &cobra.Command{
		Use:   "desktop",
		Short: "Describe how the desktop looks (dock order, panel, wallpaper) for the preview",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, env, err := load(*f)
			if err != nil {
				return err
			}
			prof, err := pickProfile(cfg, env, f.profile)
			if err != nil {
				return err
			}
			p, err := engine.Preview(cmd.Context(), cfg, prof, env)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, p)
			}
			fmt.Printf("%s: %d dock items, wallpaper %q\n", p.Kind, len(p.Dock.Items), p.WallpaperSource)
			for _, it := range p.Dock.Items {
				fmt.Printf("  %s (%s)\n", it.Name, it.ID)
			}
			return nil
		},
	}
	desktop.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	setDock := &cobra.Command{
		Use:   "set-dock [id...]",
		Short: "Save the dock order in the profile (applied on the next apply)",
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, env, err := load(*f)
			if err != nil {
				return err
			}
			prof, err := pickProfile(cfg, env, f.profile)
			if err != nil {
				return err
			}
			path := filepath.Join(env.Root, "config", "profiles", prof+".yaml")
			if err := config.SetDockPinned(path, args); err != nil {
				return err
			}
			if _, err := config.Load(env.Root); err != nil {
				return fmt.Errorf("profile no longer loads after the edit: %w", err)
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"pinned": append([]string{}, args...)})
			}
			fmt.Printf("%d dock entries saved in %s\n", len(args), path)
			return nil
		},
	}
	setDock.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	var newApp struct {
		step, id, name, method, description string
		optional                            bool
	}
	addApp := &cobra.Command{
		Use:   "add-app --step <id> --id <app> [--name <package>]",
		Short: "Add an app to a step of the profile, creating its catalogue entry when it is new",
		Long: "Puts <app> in the step's list of toggleable items. When the app is not in\n" +
			"config/apps.yaml yet, --name is required and an entry is created for this kind\n" +
			"of machine (winget ID on Windows, package name on Linux, Steam AppID with\n" +
			"--method steam).",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cfg, env, err := load(*f)
			if err != nil {
				return err
			}
			prof, err := pickProfile(cfg, env, f.profile)
			if err != nil {
				return err
			}
			profPath := filepath.Join(env.Root, "config", "profiles", prof+".yaml")
			appsPath := filepath.Join(env.Root, "config", "apps.yaml")
			undo, err := snapshot(profPath, appsPath)
			if err != nil {
				return err
			}
			_, known := cfg.Apps[newApp.id]
			if !known {
				if newApp.name == "" {
					return fmt.Errorf("%q is not in the catalogue: pass --name with its package name", newApp.id)
				}
				// The entry is written for the machines the profile targets, so
				// the Windows profile can be edited from Linux too.
				family := env.Info.Family
				for _, m := range cfg.Profiles[prof].Match {
					if m == "windows" {
						family = "windows"
					}
				}
				method, err := methodFor(family, newApp.method)
				if err != nil {
					return err
				}
				if err := config.AppendApp(appsPath, newApp.id, family, method, newApp.name, newApp.description); err != nil {
					return err
				}
			}
			if err := config.AddStepApp(profPath, newApp.step, newApp.id, newApp.optional); err != nil {
				undo()
				return err
			}
			if _, err := config.Load(env.Root); err != nil {
				undo()
				return fmt.Errorf("the config would no longer load, nothing was changed: %w", err)
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"id": newApp.id, "step": newApp.step, "created": !known})
			}
			fmt.Printf("%s added to step %s of %s\n", newApp.id, newApp.step, prof)
			return nil
		},
	}
	addApp.Flags().StringVar(&newApp.step, "step", "", "step id")
	addApp.Flags().StringVar(&newApp.id, "id", "", "app id in the catalogue (lowercase, no spaces)")
	addApp.Flags().StringVar(&newApp.name, "name", "", "package name, winget ID or Steam AppID (for a new app)")
	addApp.Flags().StringVar(&newApp.method, "method", "", "install method for a new app (default: the native one)")
	addApp.Flags().StringVar(&newApp.description, "description", "", "short description for a new app")
	addApp.Flags().BoolVar(&newApp.optional, "optional", false, "start unselected")
	addApp.MarkFlagRequired("step")
	addApp.MarkFlagRequired("id")
	addApp.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	var rmStep string
	removeApp := &cobra.Command{
		Use:   "remove-app --step <id> <app>",
		Short: "Take an app out of a step of the profile (its catalogue entry is kept)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, env, err := load(*f)
			if err != nil {
				return err
			}
			prof, err := pickProfile(cfg, env, f.profile)
			if err != nil {
				return err
			}
			profPath := filepath.Join(env.Root, "config", "profiles", prof+".yaml")
			undo, err := snapshot(profPath)
			if err != nil {
				return err
			}
			if err := config.RemoveStepApp(profPath, rmStep, args[0]); err != nil {
				return err
			}
			if _, err := config.Load(env.Root); err != nil {
				undo()
				return fmt.Errorf("the profile would no longer load, nothing was changed: %w", err)
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"id": args[0], "step": rmStep})
			}
			fmt.Printf("%s removed from step %s of %s\n", args[0], rmStep, prof)
			return nil
		},
	}
	removeApp.Flags().StringVar(&rmStep, "step", "", "step id")
	removeApp.MarkFlagRequired("step")
	removeApp.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	return []*cobra.Command{add, show, setDirs, desktop, setDock, addApp, removeApp}
}

// snapshot remembers the content of files about to be edited; the returned
// function puts it back.
func snapshot(paths ...string) (undo func(), err error) {
	saved := make([][]byte, len(paths))
	for i, p := range paths {
		if saved[i], err = os.ReadFile(p); err != nil {
			return nil, err
		}
	}
	return func() {
		for i, p := range paths {
			os.WriteFile(p, saved[i], 0o644)
		}
	}, nil
}

// methodFor validates the install method of an app created on this kind of
// machine. "" is the native one (winget on Windows).
func methodFor(family, method string) (string, error) {
	allowed := map[string][]string{
		"windows": {config.MethodWinget, config.MethodScoop, config.MethodSteam},
		"arch":    {config.MethodNative, config.MethodAUR},
		"debian":  {config.MethodNative, config.MethodFlatpak},
	}[family]
	if allowed == nil {
		return "", fmt.Errorf("apps cannot be added on this platform (%s)", family)
	}
	if method == "" || method == allowed[0] {
		return "", nil
	}
	for _, m := range allowed {
		if m == method {
			return m, nil
		}
	}
	return "", fmt.Errorf("method %q cannot be used here (choose from: %s)", method, strings.Join(allowed, ", "))
}
