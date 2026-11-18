// Command envsetup detects the machine and applies a declarative
// environment: the apps, dotfiles and settings of a profile in config/.
// The desktop app (app/) runs it as a sidecar with --json.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRoot().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	f := &flags{}
	root := &cobra.Command{
		Use:           "envsetup",
		Short:         "Detect this machine and apply a declarative environment",
		SilenceUsage:  true,
		SilenceErrors: false,
		// With no subcommand, show the help: the desktop app is the interface.
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	root.PersistentFlags().StringVar(&f.root, "root", "", "repo root containing config/ and dotfiles/ (default: auto-detect)")
	root.PersistentFlags().StringVarP(&f.profile, "profile", "p", "", "profile to use (default: the one matching this machine)")

	for _, group := range [][]*cobra.Command{
		machineCommands(f),  // machine.go: detect, info, profiles, steps
		applyCommands(f),    // apply.go: plan, apply
		editCommands(f),     // edit.go: add, show, set-dirs, desktop, set-dock, add-app, remove-app
		installCommands(f),  // install.go: the Windows installation USB
		policyCommands(f),   // policies.go
		layoutCommands(f),   // layout.go
		iconCommands(f),     // icons.go
		windhawkCommands(f), // windhawk.go: mods and the taskbar dock
		playniteCommands(f), // playnite.go
	} {
		root.AddCommand(group...)
	}
	root.SetContext(signalContext())
	return root
}

func signalContext() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ctx
}
