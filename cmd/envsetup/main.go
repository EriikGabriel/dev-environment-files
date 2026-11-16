// Command envsetup detects the host and applies a declarative environment.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/backend"
	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
	"github.com/EriikGabriel/dev-environment-files/internal/tui"
)

type flags struct {
	root    string
	profile string
	apps    []string
	noFiles bool
	noApps  bool
	choose  map[string]string
	set     []string
	yes     bool
	json    bool
	sudoPW  bool // read the sudo password from stdin (desktop app)
	// add
	group string
	copy  bool
}

func main() {
	if err := newRoot().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	var f flags
	root := &cobra.Command{
		Use:           "envsetup",
		Short:         "Detect this machine and apply a declarative environment",
		SilenceUsage:  true,
		SilenceErrors: false,
		// With no subcommand, open the TUI.
		RunE: func(*cobra.Command, []string) error {
			cfg, env, err := load(f)
			if err != nil {
				return err
			}
			prof, err := pickProfile(cfg, env, f.profile)
			if err != nil {
				return err
			}
			return tui.Run(cfg, env, prof)
		},
	}
	root.PersistentFlags().StringVar(&f.root, "root", "", "repo root containing config/ and dotfiles/ (default: auto-detect)")
	root.PersistentFlags().StringVarP(&f.profile, "profile", "p", "", "profile to use (default: the one matching this machine)")

	plan := &cobra.Command{
		Use:   "plan",
		Short: "Show what apply would do, without changing anything",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, env, err := buildPlan(f)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, planJSON{
					Platform: toPlatformJSON(env.Info), Root: env.Root, Profile: p.Profile, Choices: p.Choices,
					Steps: nonNil(p.Describe(cmd.Context())), Skipped: nonNilSkipped(p.Skipped),
				})
			}
			printPlan(p, cmd.Context())
			return nil
		},
	}
	addPlanFlags(plan, &f)
	plan.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	apply := &cobra.Command{
		Use:   "apply",
		Short: "Install the profile's apps and deploy its config files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Geteuid() == 0 && os.Getenv("SUDO_USER") != "" {
				return fmt.Errorf("run envsetup as your normal user, not through sudo: it calls sudo itself where needed, and per-user files must land in your home")
			}
			if f.json {
				return applyJSON(cmd.Context(), f)
			}
			p, _, err := buildPlan(f)
			if err != nil {
				return err
			}
			printPlan(p, cmd.Context())
			if !f.yes && !confirm("Apply?") {
				return fmt.Errorf("aborted")
			}
			res := p.Execute(cmd.Context(), os.Stdout, func(ev engine.Event) {
				switch ev.Status {
				case engine.Running:
					fmt.Printf("\n==> [%d/%d] %s: %s\n", ev.Index, ev.Total, ev.Title, ev.Detail)
				case engine.Done:
					fmt.Printf("    ok\n")
				case engine.UpToDate:
					fmt.Printf("[%d/%d] %s: %s\n", ev.Index, ev.Total, ev.Title, ev.Detail)
				case engine.Failed:
					fmt.Printf("    FAILED: %v\n", ev.Err)
				}
			})
			fmt.Printf("\n%d done, %d up to date, %d failed\n", res.Done, res.UpToDate, res.Failed)
			for _, fl := range res.Failures {
				fmt.Printf("  - %s: %v\n", fl.Title, fl.Err)
			}
			if res.Failed > 0 {
				return fmt.Errorf("%d steps failed", res.Failed)
			}
			return nil
		},
	}
	addPlanFlags(apply, &f)
	apply.Flags().BoolVarP(&f.yes, "yes", "y", false, "do not ask for confirmation")
	apply.Flags().BoolVar(&f.json, "json", false, "stream progress as JSON lines (implies --yes)")
	apply.Flags().BoolVar(&f.sudoPW, "sudo-password-stdin", false, "read the sudo password from the first line of stdin")

	detect := &cobra.Command{
		Use:   "detect",
		Short: "Print the detected platform",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			i := platform.Detect()
			if f.json {
				return printJSON(os.Stdout, toPlatformJSON(i))
			}
			fmt.Printf("os:      %s/%s\nid:      %s\nlike:    %s\nfamily:  %s\nname:    %s\n",
				i.OS, i.Arch, i.ID, strings.Join(i.Like, " "), i.Family, i.Name)
			return nil
		},
	}
	detect.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	steps := &cobra.Command{
		Use:   "steps",
		Short: "Describe the profile's setup steps and the state of each app",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, env, err := load(f)
			if err != nil {
				return err
			}
			id, err := pickProfile(cfg, env, f.profile)
			if err != nil {
				return err
			}
			ws, err := engine.Wizard(cmd.Context(), cfg, id, env)
			if err != nil {
				return err
			}
			p := cfg.Profiles[id]
			if f.json {
				return printJSON(os.Stdout, wizardJSON{Profile: id, Name: p.Name, Description: p.Description,
					Environment: p.Environment, Platform: toPlatformJSON(env.Info), Steps: ws})
			}
			for i, s := range ws {
				fmt.Printf("%d. %s (%s)\n", i+1, s.Title, s.ID)
				for _, a := range s.Apps {
					mark := map[bool]string{true: "x", false: " "}[a.Default]
					fmt.Printf("     [%s] %-22s %s\n", mark, a.ID, a.Detail)
				}
				for _, o := range s.Options {
					mark := map[bool]string{true: "*", false: " "}[o.ID == s.Default]
					fmt.Printf("     (%s) %s=%s\n", mark, s.ID, o.ID)
				}
			}
			return nil
		},
	}
	steps.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

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

	desktop := &cobra.Command{
		Use:   "desktop",
		Short: "Describe how the desktop looks (dock order, panel, wallpaper) for the preview",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, env, err := load(f)
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
			cfg, env, err := load(f)
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

	var dirStep, dirOption string
	setDirs := &cobra.Command{
		Use:   "set-dirs --step <id> [--option <id>] [dir...]",
		Short: "Replace the directories a step (or one of its options) creates",
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, env, err := load(f)
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

	info := &cobra.Command{
		Use:   "info",
		Short: "Print details about this machine",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			d := platform.Describe(platform.Detect())
			if f.json {
				return printJSON(os.Stdout, d)
			}
			for _, kv := range d.Pairs() {
				fmt.Printf("%-10s %s\n", kv[0]+":", kv[1])
			}
			return nil
		},
	}
	info.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	profiles := &cobra.Command{
		Use:   "profiles",
		Short: "List the available profiles",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cfg, _, err := load(f)
			if err != nil {
				return err
			}
			out := []profileJSON{}
			host := platform.Detect()
			matched, _ := cfg.MatchProfile(matchKeys(host))
			for _, id := range cfg.ProfileNames() {
				p := cfg.Profiles[id]
				forWindows := false
				for _, m := range p.Match {
					forWindows = forWindows || m == "windows"
				}
				out = append(out, profileJSON{ID: id, Name: p.Name, Description: p.Description,
					Environment: p.Environment, Match: p.Match, Matches: id == matched,
					Compatible: forWindows == (host.OS == "windows")})
			}
			if f.json {
				return printJSON(os.Stdout, out)
			}
			for _, p := range out {
				if !p.Compatible {
					continue // not for this operating system; -p still accepts it
				}
				fmt.Printf("%-12s %s\n", p.ID, p.Description)
			}
			return nil
		},
	}
	profiles.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

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
			cfg, env, err := load(f)
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
			cfg, env, err := load(f)
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

	root.AddCommand(plan, apply, detect, info, profiles, steps, add, show, setDirs, desktop, setDock, addApp, removeApp)
	root.AddCommand(windowsCommands(&f)...)
	root.SetContext(signalContext())
	return root
}

func addPlanFlags(c *cobra.Command, f *flags) {
	c.Flags().StringSliceVar(&f.apps, "apps", nil, "only these apps from the profile (comma separated)")
	c.Flags().BoolVar(&f.noFiles, "no-files", false, "skip dotfiles and vscode extensions")
	c.Flags().StringToStringVar(&f.choose, "choose", nil, "pick an option of a step, as step=option (\"none\" skips it); repeatable")
	c.Flags().BoolVar(&f.noApps, "no-apps", false, "skip every app (dotfiles only)")
	c.Flags().StringArrayVar(&f.set, "set", nil, "a value a step asks for, as key=value (git.name=...); scripts see it as $ENVSETUP_<KEY>; repeatable")
}

func signalContext() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ctx
}

func load(f flags) (*config.Config, *engine.Env, error) {
	root, err := findRoot(f.root)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, nil, err
	}
	home, err := engine.DefaultHome()
	if err != nil {
		return nil, nil, err
	}
	return cfg, &engine.Env{Info: platform.Detect(), Root: root, Home: home, Runner: backend.ExecRunner{}}, nil
}

func buildPlan(f flags) (*engine.Plan, *engine.Env, error) {
	cfg, env, err := load(f)
	if err != nil {
		return nil, nil, err
	}
	prof, err := pickProfile(cfg, env, f.profile)
	if err != nil {
		return nil, nil, err
	}
	apps := f.apps
	if f.noApps {
		apps = []string{} // non-nil and empty: select nothing, not everything
	}
	vars := map[string]string{}
	for _, kv := range f.set {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, nil, fmt.Errorf("--set wants key=value, got %q", kv)
		}
		vars[k] = v
	}
	p, err := engine.BuildPlan(cfg, prof, env, engine.Options{Apps: apps, NoFiles: f.noFiles, Choices: f.choose, Vars: vars})
	return p, env, err
}

func printPlan(p *engine.Plan, ctx context.Context) {
	for _, l := range p.Describe(ctx) {
		mark := "="
		if l.Pending {
			mark = "+"
		}
		fmt.Printf("%s %-28s %s\n", mark, l.Title, l.Detail)
	}
	for _, s := range p.Skipped {
		fmt.Printf("- %-28s skipped: %s\n", s.ID, s.Reason)
	}
}

func confirm(q string) bool {
	fmt.Printf("%s [y/N] ", q)
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	ans = strings.ToLower(strings.TrimSpace(ans))
	return ans == "y" || ans == "yes"
}

// findRoot locates the repo root: the --root flag, else the nearest parent of
// the working directory (or of the executable) that has config/apps.yaml.
func findRoot(flag string) (string, error) {
	has := func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, "config", "apps.yaml"))
		return err == nil
	}
	if flag != "" {
		if !has(flag) {
			return "", fmt.Errorf("%s has no config/apps.yaml", flag)
		}
		return filepath.Abs(flag)
	}
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, d := range starts {
		for ; ; d = filepath.Dir(d) {
			if has(d) {
				return d, nil
			}
			if d == filepath.Dir(d) {
				break
			}
		}
	}
	return "", fmt.Errorf("could not find config/apps.yaml; run inside the repo or pass --root")
}

func nonNil(l []engine.Line) []engine.Line {
	if l == nil {
		return []engine.Line{}
	}
	return l
}

func nonNilSkipped(l []engine.Skipped) []engine.Skipped {
	if l == nil {
		return []engine.Skipped{}
	}
	return l
}

// applyJSON runs apply for the desktop app: no prompts, progress as JSON lines.
func applyJSON(ctx context.Context, f flags) error {
	if f.sudoPW {
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		cleanup, err := backend.InstallSudoShim(strings.TrimRight(line, "\r\n"))
		if err != nil {
			return err
		}
		defer cleanup()
		if err := backend.CheckSudo(); err != nil {
			return err
		}
	}
	backend.NoStdin = true
	p, _, err := buildPlan(f)
	if err != nil {
		return err
	}
	out := newNDJSON(os.Stdout)
	logFile, logPath := openRunLog()
	if logFile != nil {
		defer logFile.Close()
	}
	lines := &logLines{out: out}
	if logFile != nil {
		lines.file = logFile
	}
	note := func(format string, a ...any) {
		if logFile != nil {
			fmt.Fprintf(logFile, "%s  "+format+"\n", append([]any{time.Now().Format("15:04:05")}, a...)...)
		}
	}
	note("apply started: profile %s, %d steps", p.Profile, len(p.Steps))
	res := p.Execute(ctx, lines, func(ev engine.Event) {
		lines.index = ev.Index
		e := streamEvent{Type: "step", Index: ev.Index, Total: ev.Total, Title: ev.Title,
			Status: statusName(ev.Status), Detail: ev.Detail, Group: ev.Group, DurationMs: ev.Elapsed.Milliseconds()}
		if ev.Err != nil {
			e.Error = ev.Err.Error()
		}
		out.emit(e)
		switch ev.Status {
		case engine.Running:
			note("[%d/%d] START   %s: %s", ev.Index, ev.Total, ev.Title, ev.Detail)
		case engine.Done:
			note("[%d/%d] CHANGED %s (%s)", ev.Index, ev.Total, ev.Title, ev.Elapsed.Round(time.Millisecond))
		case engine.UpToDate:
			note("[%d/%d] SAME    %s: %s", ev.Index, ev.Total, ev.Title, ev.Detail)
		case engine.Failed:
			note("[%d/%d] FAILED  %s: %v", ev.Index, ev.Total, ev.Title, ev.Err)
		}
	})
	if ctx.Err() != nil {
		note("apply interrupted")
	}
	note("finished: %d changed, %d already up to date, %d failed", res.Done, res.UpToDate, res.Failed)
	out.emit(streamEvent{Type: "done", Done: &res.Done, UpToDate: &res.UpToDate, Failed: &res.Failed, LogFile: logPath})
	if res.Failed > 0 {
		return fmt.Errorf("%d steps failed", res.Failed)
	}
	return nil
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

// matchKeys are the keys profiles are matched against for a host.
func matchKeys(i platform.Info) []string {
	if i.OS != "linux" {
		return []string{i.OS}
	}
	return i.Candidates()
}

// pickProfile returns the requested profile, or the one matching the host.
func pickProfile(cfg *config.Config, env *engine.Env, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	if id, ok := cfg.MatchProfile(matchKeys(env.Info)); ok {
		return id, nil
	}
	return "", fmt.Errorf("no profile matches this machine (%s); pass --profile (available: %s)",
		env.Info.ID, strings.Join(cfg.ProfileNames(), ", "))
}

// openRunLog creates the text log of an apply run under the user's state
// directory. A failure to create it only disables the file log.
func openRunLog() (*os.File, string) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" && runtime.GOOS == "windows" {
		dir, _ = os.UserCacheDir() // %LocalAppData%
	}
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	dir = filepath.Join(dir, "envsetup")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, ""
	}
	path := filepath.Join(dir, "apply-"+time.Now().Format("20060102-150405")+".log")
	f, err := os.Create(path)
	if err != nil {
		return nil, ""
	}
	return f, path
}
