package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/appupdate"
)

type updateInstaller struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type updateJSON struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Tag       string `json:"tag"`
	Available bool   `json:"available"`
	Notes     string `json:"notes"`
	Page      string `json:"page"`
	// Installer is the file the app can install itself from (Windows); empty
	// elsewhere, where the new version comes from the package or install.sh.
	Installer *updateInstaller `json:"installer"`
}

// notesLimit keeps the notes of a release short enough for a banner.
const notesLimit = 1500

func checkUpdate(cmd *cobra.Command, current string) (updateJSON, appupdate.Release, error) {
	var c appupdate.Client
	rel, err := c.Latest(cmd.Context())
	if err != nil {
		return updateJSON{}, rel, err
	}
	newer, err := appupdate.Newer(current, rel.Version)
	if err != nil {
		return updateJSON{}, rel, err
	}
	notes := rel.Notes
	if len(notes) > notesLimit {
		notes = strings.TrimSpace(notes[:notesLimit]) + "…"
	}
	out := updateJSON{Current: strings.TrimPrefix(current, "v"), Latest: rel.Version, Tag: rel.Tag, Available: newer, Notes: notes, Page: rel.Page}
	if a, ok := rel.Installer(runtime.GOOS); ok {
		out.Installer = &updateInstaller{Name: a.Name, Size: a.Size}
	}
	return out, rel, nil
}

// appUpdateCommands let the app update itself: look for a newer release and
// download its installer, checked against the checksum GitHub has for it.
func appUpdateCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)
	var current string

	check := jsonFlag(&cobra.Command{
		Use:   "app-update-check",
		Short: "Tell whether a newer release of the app is published",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u, _, err := checkUpdate(cmd, current)
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, u)
			}
			if u.Available {
				fmt.Printf("%s is available (this is %s)\n%s\n", u.Tag, u.Current, u.Page)
			} else {
				fmt.Printf("up to date (%s)\n", u.Current)
			}
			return nil
		},
	})
	check.Flags().StringVar(&current, "current", "", "the version that is running, like 0.2.1")
	_ = check.MarkFlagRequired("current")

	var dlCurrent string
	download := jsonFlag(&cobra.Command{
		Use:   "app-update-download",
		Short: "Download the installer of the latest release (Windows), checked against its SHA-256",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u, rel, err := checkUpdate(cmd, dlCurrent)
			if err != nil {
				return err
			}
			if !u.Available {
				return fmt.Errorf("already up to date (%s)", u.Current)
			}
			a, ok := rel.Installer(runtime.GOOS)
			if !ok {
				return fmt.Errorf("this system installs %s from its package: see %s", u.Tag, u.Page)
			}
			var c appupdate.Client
			path, err := c.Download(cmd.Context(), a, appupdate.Dir())
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]string{"path": path, "tag": u.Tag, "version": u.Latest})
			}
			fmt.Println(path)
			return nil
		},
	})
	download.Flags().StringVar(&dlCurrent, "current", "", "the version that is running, like 0.2.1")
	_ = download.MarkFlagRequired("current")

	return []*cobra.Command{check, download}
}
