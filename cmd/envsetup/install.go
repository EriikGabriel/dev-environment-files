// The JSON shapes in these files are part of the contract with the desktop
// app; see app/src/types.ts.
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
	"github.com/EriikGabriel/dev-environment-files/internal/windows/install"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/ninite"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/policy"
)

type unattendJSON struct {
	Path      string `json:"path"` // relative to the repo root
	Generator string `json:"generator"`
	// Error is set when the file is missing or is not an answer file.
	Error string `json:"error,omitempty"`
	install.Overview
}

type niniteJSON struct {
	App       string   `json:"app"` // catalogue id that holds the list
	Selected  []string `json:"selected"`
	Unknown   []string `json:"unknown"` // selected, but not in the catalogue below
	Page      string   `json:"page"`
	Installer string   `json:"installer"`
	// OnDrive is the installer's file name at the root of the USB drive.
	OnDrive string       `json:"on_drive"`
	Catalog []ninite.App `json:"catalog"`
}

type ventoyJSON struct {
	Path        string                `json:"path"`
	Error       string                `json:"error,omitempty"`
	AutoInstall []install.AutoInstall `json:"auto_install"`
}

type mediaJSON struct {
	// Available is false when the profile has no `media` section.
	Available bool            `json:"available"`
	Profile   string          `json:"profile"`
	Unattend  *unattendJSON   `json:"unattend,omitempty"`
	Ventoy    *ventoyJSON     `json:"ventoy,omitempty"`
	Ninite    *niniteJSON     `json:"ninite,omitempty"`
	Drives    []install.Drive `json:"drives"`
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
		md.slugs, _ = ninite.Parse([]string{cfg.Apps[md.m.Ninite].Names["windows"]})
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
	out := mediaJSON{Profile: md.profile, Drives: []install.Drive{}}
	if md.m.Unattend == "" {
		return out
	}
	out.Available = true
	u := &unattendJSON{Path: md.m.Unattend, Generator: install.GeneratorURL}
	if md.unattend == nil {
		u.Error = "file not found"
		u.Sections, u.Scripts = []install.Section{}, []install.Script{}
	} else {
		ov, err := install.ReadUnattend(md.unattend)
		u.Overview = ov
		if err != nil {
			u.Error = err.Error()
		}
	}
	out.Unattend = u

	if md.m.Ventoy != "" {
		v := &ventoyJSON{Path: md.m.Ventoy, AutoInstall: []install.AutoInstall{}}
		if md.ventoy == nil {
			v.Error = "file not found"
		} else if cfg, err := install.ReadVentoy(md.ventoy); err != nil {
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
		out.Ninite = &niniteJSON{App: md.m.Ninite, Selected: sel, Unknown: ninite.Unknown(sel),
			Page: ninite.Page(sel), Installer: ninite.Installer(sel),
			OnDrive: ninite.File, Catalog: ninite.Catalog}
	}
	if md.unattend != nil && md.ventoy != nil {
		for _, d := range install.FindDrives() {
			if st, err := install.Inspect(d, md.unattend, md.ventoy); err == nil {
				out.Drives = append(out.Drives, st)
			}
		}
	}
	return out
}

// installCommands prepare the unattended Windows installation: the answer
// file, Ninite and the Ventoy USB drive.
func installCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)

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
			slugs, err := ninite.Parse(args)
			if err != nil {
				return err
			}
			appsPath := filepath.Join(md.root, "config", "apps.yaml")
			undo, err := backupFiles(appsPath)
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
				return printJSON(os.Stdout, map[string]any{"selected": slugs, "unknown": ninite.Unknown(slugs)})
			}
			fmt.Printf("%d Ninite apps saved: %s\n", len(slugs), ninite.Page(slugs))
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
			ov, err := install.ReadUnattend(data)
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
			if _, err := install.ReadUnattend(md.unattend); err != nil {
				return fmt.Errorf("%s: %w", md.m.Unattend, err)
			}
			slugs := md.slugs
			if noNinite {
				slugs = nil
			}
			written, err := install.WriteDrive(cmd.Context(), drive, md.unattend, md.ventoy, slugs, install.Download)
			if f.json && err == nil {
				st, _ := install.Inspect(drive, md.unattend, md.ventoy)
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
				link = install.GeneratorURL
			case "ninite-home":
				link = ninite.Home
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
					link = ninite.Page(md.slugs)
					break
				}
				if err := md.need("unattend", md.m.Unattend, md.unattend); err != nil {
					return err
				}
				ov, err := install.ReadUnattend(md.unattend)
				if err != nil {
					return err
				}
				if link = ov.URL; link == "" {
					return fmt.Errorf("%s has no generator link (it is the comment at the top of files made by %s)", md.m.Unattend, install.GeneratorURL)
				}
			default:
				return fmt.Errorf("unknown target %q", args[0])
			}
			if err := platform.OpenURL(link); err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"opened": args[0]})
			}
			return nil
		},
	})

	return []*cobra.Command{mediaCmd, setNinite, setUnattend, writeMedia, openURL}
}
