package main

import (
	"github.com/spf13/cobra"
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

// withJSON returns a helper that adds --json to a command; the subcommands
// the desktop app calls all have it.
func withJSON(f *flags) func(*cobra.Command) *cobra.Command {
	return func(c *cobra.Command) *cobra.Command {
		c.Flags().BoolVar(&f.json, "json", false, "machine-readable output")
		return c
	}
}

func addPlanFlags(c *cobra.Command, f *flags) {
	c.Flags().StringSliceVar(&f.apps, "apps", nil, "only these apps from the profile (comma separated)")
	c.Flags().BoolVar(&f.noFiles, "no-files", false, "skip dotfiles and vscode extensions")
	c.Flags().StringToStringVar(&f.choose, "choose", nil, "pick an option of a step, as step=option (\"none\" skips it); repeatable")
	c.Flags().BoolVar(&f.noApps, "no-apps", false, "skip every app (dotfiles only)")
	c.Flags().StringArrayVar(&f.set, "set", nil, "a value a step asks for, as key=value (git.name=...); scripts see it as $ENVSETUP_<KEY>; repeatable")
}
