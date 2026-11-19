package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/icons"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/windhawk"
)

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

// iconCommands style the icons of the Windows shortcuts (config/icons.yaml).
func iconCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)

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
					cfg.Overrides[k] = repoRelative(root, v)
				}
			}
			// files inside the repo are kept relative to it, so the config
			// still works in another clone or in the copy the app makes
			for k, v := range cfg.Overrides {
				cfg.Overrides[k] = repoRelative(root, v)
			}
			if ch("pack") {
				cfg.Pack = repoRelative(root, cfg.Pack)
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

	return []*cobra.Command{iconsCmd, setIcons, applyIcons, restoreIcons, iconPack}
}
