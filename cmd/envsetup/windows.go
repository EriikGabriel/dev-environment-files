package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/icons"
	"github.com/EriikGabriel/dev-environment-files/internal/layout"
	"github.com/EriikGabriel/dev-environment-files/internal/policy"
	"github.com/EriikGabriel/dev-environment-files/internal/windhawk"
	"github.com/EriikGabriel/dev-environment-files/internal/winsetup"
)

// The JSON shapes below are part of the contract with the desktop app; see
// app/src/types.ts.

type unattendJSON struct {
	Path      string `json:"path"` // relative to the repo root
	Generator string `json:"generator"`
	// Error is set when the file is missing or is not an answer file.
	Error string `json:"error,omitempty"`
	winsetup.Overview
}

type niniteJSON struct {
	App       string   `json:"app"` // catalogue id that holds the list
	Selected  []string `json:"selected"`
	Unknown   []string `json:"unknown"` // selected, but not in the catalogue below
	Page      string   `json:"page"`
	Installer string   `json:"installer"`
	// OnDrive is the installer's file name at the root of the USB drive.
	OnDrive string               `json:"on_drive"`
	Catalog []winsetup.NiniteApp `json:"catalog"`
}

type ventoyJSON struct {
	Path        string                 `json:"path"`
	Error       string                 `json:"error,omitempty"`
	AutoInstall []winsetup.AutoInstall `json:"auto_install"`
}

type mediaJSON struct {
	// Available is false when the profile has no `media` section.
	Available bool             `json:"available"`
	Profile   string           `json:"profile"`
	Unattend  *unattendJSON    `json:"unattend,omitempty"`
	Ventoy    *ventoyJSON      `json:"ventoy,omitempty"`
	Ninite    *niniteJSON      `json:"ninite,omitempty"`
	Drives    []winsetup.Drive `json:"drives"`
}

// media is a profile's installation media, resolved against the repo.
type media struct {
	cfg      *config.Config
	root     string
	profile  string
	m        config.Media
	unattend []byte // nil when unreadable
	ventoy   []byte
	slugs    []string
}

func loadMedia(f flags) (*media, error) {
	cfg, env, err := load(f)
	if err != nil {
		return nil, err
	}
	prof, err := pickProfile(cfg, env, f.profile)
	if err != nil {
		return nil, err
	}
	p, ok := cfg.Profiles[prof]
	if !ok {
		return nil, fmt.Errorf("unknown profile %q", prof)
	}
	md := &media{cfg: cfg, root: env.Root, profile: prof, m: p.Media}
	if md.m.Unattend != "" {
		md.unattend, _ = os.ReadFile(filepath.Join(md.root, md.m.Unattend))
	}
	if md.m.Ventoy != "" {
		md.ventoy, _ = os.ReadFile(filepath.Join(md.root, md.m.Ventoy))
	}
	if md.m.Ninite != "" {
		// A list that no longer parses shows up as empty; set-ninite fixes it.
		md.slugs, _ = winsetup.ParseNinite([]string{cfg.Apps[md.m.Ninite].Names["windows"]})
	}
	return md, nil
}

func (md *media) need(what, path string, data []byte) error {
	switch {
	case path == "":
		return fmt.Errorf("profile %s has no media.%s", md.profile, what)
	case data == nil:
		return fmt.Errorf("%s cannot be read", path)
	}
	return nil
}

func (md *media) describe() mediaJSON {
	out := mediaJSON{Profile: md.profile, Drives: []winsetup.Drive{}}
	if md.m.Unattend == "" {
		return out
	}
	out.Available = true
	u := &unattendJSON{Path: md.m.Unattend, Generator: winsetup.GeneratorURL}
	if md.unattend == nil {
		u.Error = "file not found"
		u.Sections, u.Scripts = []winsetup.Section{}, []winsetup.Script{}
	} else {
		ov, err := winsetup.ReadUnattend(md.unattend)
		u.Overview = ov
		if err != nil {
			u.Error = err.Error()
		}
	}
	out.Unattend = u

	if md.m.Ventoy != "" {
		v := &ventoyJSON{Path: md.m.Ventoy, AutoInstall: []winsetup.AutoInstall{}}
		if md.ventoy == nil {
			v.Error = "file not found"
		} else if cfg, err := winsetup.ReadVentoy(md.ventoy); err != nil {
			v.Error = err.Error()
		} else {
			v.AutoInstall = cfg.AutoInstall
		}
		out.Ventoy = v
	}
	if md.m.Ninite != "" {
		sel := md.slugs
		if sel == nil {
			sel = []string{}
		}
		out.Ninite = &niniteJSON{App: md.m.Ninite, Selected: sel, Unknown: winsetup.UnknownNinite(sel),
			Page: winsetup.NinitePage(sel), Installer: winsetup.NiniteInstaller(sel),
			OnDrive: winsetup.NiniteFile, Catalog: winsetup.NiniteCatalog}
	}
	if md.unattend != nil && md.ventoy != nil {
		for _, d := range winsetup.FindDrives() {
			if st, err := winsetup.Inspect(d, md.unattend, md.ventoy); err == nil {
				out.Drives = append(out.Drives, st)
			}
		}
	}
	return out
}

type policyJSON struct {
	policy.Policy
	// Applied tells whether the registry already matches the choice. It is
	// false everywhere when the state cannot be read (not on Windows).
	Applied bool `json:"applied"`
}

type policiesJSON struct {
	Source string `json:"source"`
	// Readable is false when this machine's registry could not be read.
	Readable bool         `json:"readable"`
	Policies []policyJSON `json:"policies"`
}

func describePolicies(cmd *cobra.Command, f flags) (policiesJSON, error) {
	_, env, err := load(f)
	if err != nil {
		return policiesJSON{}, err
	}
	file, err := policy.Load(engine.PoliciesPath(env.Root))
	if err != nil {
		return policiesJSON{}, err
	}
	out := policiesJSON{Source: file.Source, Policies: []policyJSON{}}
	var state map[string]bool
	if env.Info.OS == "windows" {
		if state, err = env.PolicyState(cmd.Context(), file); err == nil {
			out.Readable = true
		}
	}
	for _, p := range file.Policies {
		out.Policies = append(out.Policies, policyJSON{Policy: p, Applied: state[p.ID]})
	}
	return out, nil
}

type iconsPackJSON struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
	Error string `json:"error,omitempty"`
	// System are the system items the pack has an icon for.
	System []string `json:"system"`
}

// iconPackJSON is a page of the icons of the pack.
type iconPackJSON struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
	// Total is how many icons the query found; Icons holds a page of them.
	Total int            `json:"total"`
	Icons []iconPackIcon `json:"icons"`
}

type iconPackIcon struct {
	Name string `json:"name"`
	File string `json:"file"`
	// Image is a small PNG as a data URL; empty when it cannot be drawn.
	Image string `json:"image"`
}

type iconsJSON struct {
	Config icons.Config `json:"config"`
	// Saved is false until config/icons.yaml exists.
	Saved bool          `json:"saved"`
	Pack  iconsPackJSON `json:"pack"`
	// BuiltinPack is the pack the repo ships, to offer it.
	BuiltinPack string `json:"builtin_pack"`
	// Applied tells whether envsetup changed icons on this machine.
	Applied bool `json:"applied"`
	// Dock tells whether the taskbar is the dock of the Windhawk mod "Ilha e
	// Dock estilo Mac" (taskbar-dock); DockMode is that mod's taskbarMode,
	// "" when the mod is not installed or Windhawk 2 is missing.
	Dock     bool   `json:"dock"`
	DockMode string `json:"dock_mode"`
	// Scanned is set when Entries holds the shortcuts found on this machine.
	Scanned bool          `json:"scanned"`
	Entries []icons.Entry `json:"entries"`
}

// describeIcons reads the choice. With scan it looks for the shortcuts of this
// machine (a dozen seconds); with cached it draws the preview from the last
// scan.
func describeIcons(cmd *cobra.Command, f flags, scan, cached bool) (iconsJSON, error) {
	_, env, err := load(f)
	if err != nil {
		return iconsJSON{}, err
	}
	cfg, exists, err := icons.Load(icons.Path(env.Root))
	if err != nil {
		return iconsJSON{}, err
	}
	out := iconsJSON{BuiltinPack: icons.BuiltinPack, Config: cfg, Saved: exists, Applied: engine.IconsApplied(), Entries: []icons.Entry{}, Pack: iconsPackJSON{System: []string{}}}
	if env.Info.OS == "windows" {
		out.DockMode, _ = env.DockMode(cmd.Context())
		out.Dock = out.DockMode == windhawk.DockOn
	}
	pack, err := icons.LoadPack(icons.Resolve(env.Root, cfg.Pack))
	if err != nil {
		out.Pack.Path, out.Pack.Error = cfg.Pack, err.Error()
	} else if pack != nil {
		out.Pack.Path, out.Pack.Count = pack.Dir, pack.Count()
		seen := map[string]bool{}
		for _, s := range icons.SystemIcons(pack) {
			if !seen[s.Item] {
				seen[s.Item] = true
				out.Pack.System = append(out.Pack.System, s.Item)
			}
		}
	}
	if scan {
		if env.Info.OS != "windows" {
			return out, fmt.Errorf("the shortcuts can only be scanned on Windows")
		}
		list, err := env.ScanIcons(cmd.Context(), os.Stderr, cfg)
		if err != nil {
			return out, err
		}
		out.Scanned = true
		out.Entries = icons.Plan(cfg, env.Root, pack, list)
	} else if cached {
		list, err := engine.CachedIconScan()
		if err != nil {
			return out, err
		}
		if list != nil {
			out.Scanned = true
			out.Entries = icons.Plan(cfg, env.Root, pack, list)
		}
	}
	return out, nil
}

// splitList splits a comma-separated flag, dropping empty parts.
func splitList(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

type layoutJSON struct {
	// Available is false when the profile has no `media` section.
	Available bool          `json:"available"`
	Items     []layout.Item `json:"items"`
	// Taskbar is the saved order, without the ids that are no longer on offer.
	Taskbar          []string `json:"taskbar"`
	ClearStart       bool     `json:"clear_start"`
	HideDesktopIcons bool     `json:"hide_desktop_icons"`
	// Saved is false until config/layout.yaml exists.
	Saved    bool               `json:"saved"`
	Shell    winsetup.Shell     `json:"shell"`
	Installs []winsetup.Install `json:"installs"`
}

// describeLayout is the saved layout plus the apps that can be pinned: the
// Ninite list, what the answer file installs and the profile's ticked apps.
func describeLayout(f flags, apps []string) (layoutJSON, error) {
	md, err := loadMedia(f)
	if err != nil {
		return layoutJSON{}, err
	}
	out := layoutJSON{Items: []layout.Item{}, Taskbar: []string{}, Installs: []winsetup.Install{}}
	if md.m.Unattend == "" {
		return out, nil
	}
	out.Available = true
	if md.unattend != nil {
		if ov, err := winsetup.ReadUnattend(md.unattend); err == nil {
			out.Shell, out.Installs = ov.Shell, ov.Installs
		}
	}
	cfg, exists, err := layout.Load(layout.Path(md.root))
	if err != nil {
		return out, err
	}
	if !exists {
		// until it is saved, the answer file's own choice is the starting point
		cfg.ClearStart = out.Shell.StartPinsEmpty
	}
	out.Saved, out.ClearStart, out.HideDesktopIcons = exists, cfg.ClearStart, cfg.HideDesktopIcons
	out.Items = layout.Candidates(layout.Inputs{Ninite: md.slugs, Apps: apps, Installs: out.Installs})
	known := map[string]bool{}
	for _, it := range out.Items {
		known[it.ID] = it.Pinnable
	}
	for _, id := range cfg.Taskbar {
		if known[id] {
			out.Taskbar = append(out.Taskbar, id)
		}
	}
	return out, nil
}

// windowsCommands are the subcommands for the Windows installation media
// and the group policies.
func windowsCommands(f *flags) []*cobra.Command {
	jsonFlag := func(c *cobra.Command) *cobra.Command {
		c.Flags().BoolVar(&f.json, "json", false, "machine-readable output")
		return c
	}

	mediaCmd := jsonFlag(&cobra.Command{
		Use:   "media",
		Short: "Describe the unattended Windows installation: the answer file, Ninite and the Ventoy drives",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			md, err := loadMedia(*f)
			if err != nil {
				return err
			}
			d := md.describe()
			if f.json {
				return printJSON(os.Stdout, d)
			}
			if !d.Available {
				fmt.Printf("profile %s has no installation media\n", d.Profile)
				return nil
			}
			fmt.Printf("answer file: %s\n", d.Unattend.Path)
			if d.Unattend.Error != "" {
				fmt.Printf("  %s\n", d.Unattend.Error)
			}
			for _, s := range d.Unattend.Sections {
				fmt.Printf("\n%s\n", s.Title)
				for _, it := range s.Items {
					if it.Value != "" {
						fmt.Printf("  %s: %s\n", it.Label, it.Value)
					} else {
						fmt.Printf("  %s\n", it.Label)
					}
				}
			}
			if len(d.Unattend.Scripts) > 0 {
				fmt.Printf("\nScripts\n")
				for _, s := range d.Unattend.Scripts {
					first, _, _ := strings.Cut(strings.TrimSpace(s.Body), "\n")
					fmt.Printf("  [%s, %s] %s\n", s.Phase, s.Type, first)
				}
			}
			if d.Ninite != nil {
				fmt.Printf("\nNinite: %s\n  %s\n", strings.Join(d.Ninite.Selected, ", "), d.Ninite.Page)
			}
			for _, dr := range d.Drives {
				fmt.Printf("\nVentoy drive %s: answer file %s, ventoy.json %s, Ninite %v\n", dr.Path, dr.Unattend, dr.Config, dr.Ninite)
			}
			return nil
		},
	})

	setNinite := jsonFlag(&cobra.Command{
		Use:   "set-ninite [app|link ...]",
		Short: "Save the list of Ninite apps (names from ninite.com, or a ninite.com link)",
		RunE: func(_ *cobra.Command, args []string) error {
			md, err := loadMedia(*f)
			if err != nil {
				return err
			}
			if md.m.Ninite == "" {
				return fmt.Errorf("profile %s has no media.ninite", md.profile)
			}
			slugs, err := winsetup.ParseNinite(args)
			if err != nil {
				return err
			}
			appsPath := filepath.Join(md.root, "config", "apps.yaml")
			undo, err := snapshot(appsPath)
			if err != nil {
				return err
			}
			if err := config.SetAppName(appsPath, md.m.Ninite, "windows", strings.Join(slugs, " ")); err != nil {
				return err
			}
			if _, err := config.Load(md.root); err != nil {
				undo()
				return fmt.Errorf("the catalogue would no longer load, nothing was changed: %w", err)
			}
			if f.json {
				if slugs == nil {
					slugs = []string{}
				}
				return printJSON(os.Stdout, map[string]any{"selected": slugs, "unknown": winsetup.UnknownNinite(slugs)})
			}
			fmt.Printf("%d Ninite apps saved: %s\n", len(slugs), winsetup.NinitePage(slugs))
			return nil
		},
	})

	setUnattend := jsonFlag(&cobra.Command{
		Use:   "set-unattend <file>",
		Short: "Replace the profile's answer file with a new autounattend.xml (downloaded from the generator)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			md, err := loadMedia(*f)
			if err != nil {
				return err
			}
			if md.m.Unattend == "" {
				return fmt.Errorf("profile %s has no media.unattend", md.profile)
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			ov, err := winsetup.ReadUnattend(data)
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			dest := filepath.Join(md.root, md.m.Unattend)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"path": md.m.Unattend, "has_password": ov.HasPassword, "has_link": ov.URL != ""})
			}
			fmt.Printf("%s replaced\n", md.m.Unattend)
			if ov.HasPassword {
				fmt.Println("warning: the file has an account password, readable by anyone; do not commit it to a public repo")
			}
			return nil
		},
	})

	var drive string
	var noNinite bool
	writeMedia := jsonFlag(&cobra.Command{
		Use:   "write-media --drive <path>",
		Short: "Put the answer file, ventoy.json and the Ninite installer on a Ventoy USB drive",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			md, err := loadMedia(*f)
			if err != nil {
				return err
			}
			if err := md.need("unattend", md.m.Unattend, md.unattend); err != nil {
				return err
			}
			if err := md.need("ventoy", md.m.Ventoy, md.ventoy); err != nil {
				return err
			}
			if _, err := winsetup.ReadUnattend(md.unattend); err != nil {
				return fmt.Errorf("%s: %w", md.m.Unattend, err)
			}
			slugs := md.slugs
			if noNinite {
				slugs = nil
			}
			written, err := winsetup.WriteDrive(cmd.Context(), drive, md.unattend, md.ventoy, slugs, winsetup.Download)
			if f.json && err == nil {
				st, _ := winsetup.Inspect(drive, md.unattend, md.ventoy)
				return printJSON(os.Stdout, map[string]any{"written": written, "drive": st})
			}
			for _, w := range written {
				fmt.Printf("%-10s %s\n", w.Action, w.Path)
				if w.Backup != "" {
					fmt.Printf("           previous content kept in %s\n", w.Backup)
				}
			}
			return err
		},
	})
	writeMedia.Flags().StringVar(&drive, "drive", "", `root of the Ventoy drive (D:\ or /run/media/you/Ventoy)`)
	writeMedia.MarkFlagRequired("drive")
	writeMedia.Flags().BoolVar(&noNinite, "no-ninite", false, "do not download the Ninite installer")

	openURL := jsonFlag(&cobra.Command{
		Use:   "open-url <unattend|generator|ninite|ninite-home|policies-source>",
		Short: "Open one of the sites the setup uses in the browser",
		Long: "unattend opens the generator with the settings of the profile's answer file,\n" +
			"generator opens it empty, ninite opens ninite.com with the saved selection,\n" +
			"and policies-source opens the video the group policies come from.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var link string
			switch args[0] {
			case "generator":
				link = winsetup.GeneratorURL
			case "ninite-home":
				link = winsetup.NiniteHome
			case "policies-source":
				root, err := findRoot(f.root)
				if err != nil {
					return err
				}
				file, err := policy.Load(engine.PoliciesPath(root))
				if err != nil {
					return err
				}
				link = file.Source
			case "unattend", "ninite":
				md, err := loadMedia(*f)
				if err != nil {
					return err
				}
				if args[0] == "ninite" {
					link = winsetup.NinitePage(md.slugs)
					break
				}
				if err := md.need("unattend", md.m.Unattend, md.unattend); err != nil {
					return err
				}
				ov, err := winsetup.ReadUnattend(md.unattend)
				if err != nil {
					return err
				}
				if link = ov.URL; link == "" {
					return fmt.Errorf("%s has no generator link (it is the comment at the top of files made by %s)", md.m.Unattend, winsetup.GeneratorURL)
				}
			default:
				return fmt.Errorf("unknown target %q", args[0])
			}
			if err := winsetup.OpenURL(link); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"opened": args[0]})
			}
			return nil
		},
	})

	policies := jsonFlag(&cobra.Command{
		Use:   "policies",
		Short: "List the group policies of config/policies.yaml, with the choice and the state of each",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := describePolicies(cmd, *f)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, d)
			}
			group := ""
			for _, p := range d.Policies {
				if p.Group != group {
					group = p.Group
					fmt.Printf("\n%s\n", group)
				}
				mark := " "
				if p.Applied {
					mark = "="
				} else if d.Readable {
					mark = "+"
				}
				fmt.Printf("  %s %-30s %-8s %s\n", mark, p.ID, p.Choice, p.Title)
			}
			return nil
		},
	})

	setPolicy := jsonFlag(&cobra.Command{
		Use:   "set-policy <id> <video|off|option>",
		Short: "Choose what a group policy is set to (applied by apply-policies or by apply)",
		Long: "Several policies can be changed at once with id=choice pairs:\n" +
			"  envsetup set-policy widgets=off cortana=video",
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			pairs := [][2]string{}
			if len(args) == 2 && !strings.Contains(args[0], "=") {
				pairs = append(pairs, [2]string{args[0], args[1]})
			} else {
				for _, a := range args {
					id, choice, ok := strings.Cut(a, "=")
					if !ok {
						return fmt.Errorf("expected id=choice, got %q", a)
					}
					pairs = append(pairs, [2]string{id, choice})
				}
			}
			path := engine.PoliciesPath(root)
			undo, err := snapshot(path)
			if err != nil {
				return err
			}
			for _, p := range pairs {
				if err := policy.SetChoice(path, p[0], p[1]); err != nil {
					undo()
					return err
				}
			}
			if _, err := policy.Load(path); err != nil {
				undo()
				return fmt.Errorf("the file would no longer load, nothing was changed: %w", err)
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"changed": len(pairs)})
			}
			fmt.Printf("%d policies changed in %s\n", len(pairs), path)
			return nil
		},
	})

	applyPolicies := jsonFlag(&cobra.Command{
		Use:   "apply-policies",
		Short: "Write the chosen group policies to the registry (Windows asks for elevation once)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, env, err := load(*f)
			if err != nil {
				return err
			}
			if env.Info.OS != "windows" {
				return fmt.Errorf("group policies can only be applied on Windows")
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr // keep stdout for the JSON answer
			}
			if err := env.ApplyPolicies(cmd.Context(), out); err != nil {
				return err
			}
			if f.json {
				d, err := describePolicies(cmd, *f)
				if err != nil {
					return err
				}
				return printJSON(os.Stdout, d)
			}
			fmt.Println("policies applied; some only take effect after signing out or restarting")
			return nil
		},
	})

	var layoutApps string
	layoutCmd := jsonFlag(&cobra.Command{
		Use:   "layout",
		Short: "Show the desktop layout (taskbar pins, Start menu, desktop icons) and the apps that can be pinned",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			d, err := describeLayout(*f, splitList(layoutApps))
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, d)
			}
			name := map[string]string{}
			for _, it := range d.Items {
				name[it.ID] = it.Name
			}
			fmt.Println("taskbar:")
			for _, id := range d.Taskbar {
				fmt.Printf("  %s (%s)\n", name[id], id)
			}
			fmt.Printf("clear Start pins: %v\nhide desktop icons: %v\n", d.ClearStart, d.HideDesktopIcons)
			return nil
		},
	})
	layoutCmd.Flags().StringVar(&layoutApps, "apps", "", "ids of the profile's apps that are ticked, separated by commas")

	var taskbarList string
	var clearStart, hideIcons bool
	setLayout := jsonFlag(&cobra.Command{
		Use:   "set-layout",
		Short: "Save the desktop layout in config/layout.yaml (applied by apply-layout)",
		Long: "Only the flags that are given change. --taskbar takes the ids in order,\n" +
			"separated by commas; an empty value leaves the taskbar as it is.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			path := layout.Path(root)
			cfg, exists, err := layout.Load(path)
			if err != nil {
				return err
			}
			if !exists {
				// keep what the answer file asks for unless it is changed now
				if md, err := loadMedia(*f); err == nil && md.unattend != nil {
					if ov, err := winsetup.ReadUnattend(md.unattend); err == nil {
						cfg.ClearStart = ov.Shell.StartPinsEmpty
					}
				}
			}
			if cmd.Flags().Changed("taskbar") {
				cfg.Taskbar = splitList(taskbarList)
			}
			if cmd.Flags().Changed("clear-start") {
				cfg.ClearStart = clearStart
			}
			if cmd.Flags().Changed("hide-icons") {
				cfg.HideDesktopIcons = hideIcons
			}
			if err := layout.Save(path, cfg); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, cfg)
			}
			fmt.Printf("layout saved in %s\n", path)
			return nil
		},
	})
	setLayout.Flags().StringVar(&taskbarList, "taskbar", "", "ids of the apps pinned to the taskbar, in order, separated by commas")
	setLayout.Flags().BoolVar(&clearStart, "clear-start", false, "remove the apps pinned to the Start menu")
	setLayout.Flags().BoolVar(&hideIcons, "hide-icons", false, "hide the desktop icons")

	applyLayout := jsonFlag(&cobra.Command{
		Use:   "apply-layout",
		Short: "Apply config/layout.yaml to this Windows machine (asks for elevation once)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, env, err := load(*f)
			if err != nil {
				return err
			}
			if env.Info.OS != "windows" {
				return fmt.Errorf("the layout can only be applied on Windows")
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr // keep stdout for the JSON answer
			}
			if err := env.ApplyLayout(cmd.Context(), out); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"applied": true})
			}
			fmt.Println("layout applied; if the taskbar did not change, sign out and in again")
			return nil
		},
	})

	var scanIcons, cachedIcons bool
	iconsCmd := jsonFlag(&cobra.Command{
		Use:   "icons",
		Short: "Show the icon style and, with --scan, the shortcuts of this machine with the icon each would get",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := describeIcons(cmd, *f, scanIcons, cachedIcons)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, d)
			}
			fmt.Printf("enabled: %v, style: %s, pack: %q (%d icons)\n", d.Config.Enabled, d.Config.Style, d.Pack.Path, d.Pack.Count)
			for _, e := range d.Entries {
				fmt.Printf("  %-10s %-8s %s\n", e.Scope, e.From, e.Name)
			}
			return nil
		},
	})
	iconsCmd.Flags().BoolVar(&scanIcons, "scan", false, "look for the shortcuts of this machine (Windows only)")
	iconsCmd.Flags().BoolVar(&cachedIcons, "cached", false, "draw the preview from the last scan, without scanning again")

	var (
		icEnabled, icSystem, icClear bool
		icStyle, icPack              string
		icScopes, icSkip             string
		icOverrides                  map[string]string
	)
	setIcons := jsonFlag(&cobra.Command{
		Use:   "set-icons",
		Short: "Save the icon style in config/icons.yaml (applied by apply-icons)",
		Long:  "Only the flags that are given change.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			path := icons.Path(root)
			cfg, _, err := icons.Load(path)
			if err != nil {
				return err
			}
			ch := cmd.Flags().Changed
			if ch("enabled") {
				cfg.Enabled = icEnabled
			}
			if ch("style") {
				cfg.Style = icStyle
			}
			if ch("pack") {
				cfg.Pack = icPack
			}
			if ch("scopes") {
				cfg.Scopes = splitList(icScopes)
			}
			if ch("system-icons") {
				cfg.SystemIcons = icSystem
			}
			if ch("skip") {
				cfg.Skip = splitList(icSkip)
			}
			if icClear {
				cfg.Overrides = map[string]string{}
			}
			if ch("override") {
				for k, v := range icOverrides {
					cfg.Overrides[k] = v
				}
			}
			if err := icons.Save(path, cfg); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, cfg)
			}
			fmt.Printf("icon style saved in %s\n", path)
			return nil
		},
	})
	setIcons.Flags().BoolVar(&icEnabled, "enabled", false, "turn the icon style on or off")
	setIcons.Flags().StringVar(&icStyle, "style", icons.StyleLight, "plate of the drawn icons: light or dark")
	setIcons.Flags().StringVar(&icPack, "pack", "", "folder of .ico/.png icons named after the apps")
	setIcons.Flags().StringVar(&icScopes, "scopes", "", "desktop, start-menu and/or taskbar, separated by commas")
	setIcons.Flags().BoolVar(&icSystem, "system-icons", true, "also set This PC, Recycle Bin... from the pack")
	setIcons.Flags().StringVar(&icSkip, "skip", "", "shortcut names that keep their icon, separated by commas")
	setIcons.Flags().StringToStringVar(&icOverrides, "override", nil, "shortcut=file: an image for one shortcut (repeatable)")
	setIcons.Flags().BoolVar(&icClear, "clear-overrides", false, "forget the overrides before adding the given ones")

	applyIcons := jsonFlag(&cobra.Command{
		Use:   "apply-icons",
		Short: "Change the icon of the shortcuts of this Windows machine (asks for elevation once)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, env, err := load(*f)
			if err != nil {
				return err
			}
			if env.Info.OS != "windows" {
				return fmt.Errorf("the icons can only be applied on Windows")
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr // keep stdout for the JSON answer
			}
			if err := env.ApplyIcons(cmd.Context(), out); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"applied": true})
			}
			fmt.Println("icons applied; Explorer restarted to show them")
			return nil
		},
	})

	restoreIcons := jsonFlag(&cobra.Command{
		Use:   "restore-icons",
		Short: "Give the shortcuts the icons they had before apply-icons (asks for elevation once)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, env, err := load(*f)
			if err != nil {
				return err
			}
			if env.Info.OS != "windows" {
				return fmt.Errorf("the icons can only be restored on Windows")
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr
			}
			if err := env.RestoreIcons(cmd.Context(), out); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"restored": true})
			}
			fmt.Println("original icons restored")
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

	var (
		ipQuery         string
		ipLimit, ipFrom int
	)
	iconPack := jsonFlag(&cobra.Command{
		Use:   "icon-pack",
		Short: "List the icons of the icon pack, all or the ones a name finds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			cfg, _, err := icons.Load(icons.Path(root))
			if err != nil {
				return err
			}
			if cfg.Pack == "" {
				return fmt.Errorf("no icon pack is set in config/icons.yaml")
			}
			pack, err := icons.LoadPack(icons.Resolve(root, cfg.Pack))
			if err != nil {
				return fmt.Errorf("the icon pack: %w", err)
			}
			found := pack.Search(ipQuery)
			out := iconPackJSON{Path: pack.Dir, Count: pack.Count(), Total: len(found), Icons: []iconPackIcon{}}
			for i := max(ipFrom, 0); i < len(found) && len(out.Icons) < ipLimit; i++ {
				ic := iconPackIcon{Name: found[i].Name, File: found[i].File}
				if im, err := icons.Image(icons.Choice{From: icons.FromPack, File: ic.File}, "", ""); err == nil {
					ic.Image = icons.DataURL(im, icons.PreviewSide)
				}
				out.Icons = append(out.Icons, ic)
			}
			if f.json {
				return printJSON(os.Stdout, out)
			}
			for _, ic := range out.Icons {
				fmt.Println(ic.Name)
			}
			fmt.Printf("%d of %d found, %d in %s\n", len(out.Icons), out.Total, out.Count, out.Path)
			return nil
		},
	})
	iconPack.Flags().StringVar(&ipQuery, "query", "", "part of the name of an app")
	iconPack.Flags().IntVar(&ipLimit, "limit", 60, "how many to list")
	iconPack.Flags().IntVar(&ipFrom, "offset", 0, "how many to pass over first")

	return []*cobra.Command{mediaCmd, setNinite, setUnattend, writeMedia, openURL, policies, setPolicy, applyPolicies, layoutCmd, setLayout, applyLayout, iconsCmd, setIcons, applyIcons, restoreIcons, iconPack, taskbarDock}
}
