package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/install"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/layout"
)

type layoutJSON struct {
	// Available is false when the profile has no `media` section.
	Available bool          `json:"available"`
	Items     []layout.Item `json:"items"`
	// Taskbar is the saved order, without the ids that are no longer on offer.
	Taskbar          []string `json:"taskbar"`
	ClearStart       bool     `json:"clear_start"`
	HideDesktopIcons bool     `json:"hide_desktop_icons"`
	// Saved is false until config/layout.yaml exists.
	Saved    bool          `json:"saved"`
	Shell    install.Shell `json:"shell"`
	Installs []install.App `json:"installs"`
}

// describeLayout is the saved layout plus the apps that can be pinned: the
// Ninite list, what the answer file installs and the profile's ticked apps.
func describeLayout(f flags, apps []string) (layoutJSON, error) {
	md, err := loadMedia(f)
	if err != nil {
		return layoutJSON{}, err
	}
	out := layoutJSON{Items: []layout.Item{}, Taskbar: []string{}, Installs: []install.App{}}
	if md.m.Unattend == "" {
		return out, nil
	}
	out.Available = true
	if md.unattend != nil {
		if ov, err := install.ReadUnattend(md.unattend); err == nil {
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

func splitList(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// layoutCommands show, save and apply the Windows desktop layout (taskbar
// pins, Start menu, desktop icons) of config/layout.yaml.
func layoutCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)

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
					if ov, err := install.ReadUnattend(md.unattend); err == nil {
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

	return []*cobra.Command{layoutCmd, setLayout, applyLayout}
}
