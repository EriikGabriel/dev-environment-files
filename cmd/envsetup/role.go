package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

// roleJSON is part of the contract with app/src/types/core.ts (Role).
type roleJSON struct {
	// Owner is true when the configuration of the data folder is the user's.
	Owner bool `json:"owner"`
	// Explicit is true when local.yaml says so; false when it is guessed
	// (a git clone is the owner's, the installer's copy someone else's).
	Explicit bool `json:"explicit"`
	Clone    bool `json:"clone"`
}

func roleCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)
	describe := func() (roleJSON, error) {
		root, err := findRoot(f.root)
		if err != nil {
			return roleJSON{}, err
		}
		owner, explicit := config.IsOwner(root)
		return roleJSON{Owner: owner, Explicit: explicit, Clone: config.IsClone(root)}, nil
	}
	print := func(r roleJSON) error {
		if f.json {
			return printJSON(os.Stdout, r)
		}
		who := "someone else's configuration: restores leave out the owner's saves, play history and accounts"
		if r.Owner {
			who = "your own configuration"
		}
		how := "guessed"
		if r.Explicit {
			how = "set in local.yaml"
		}
		fmt.Printf("%s (%s)\n", who, how)
		return nil
	}
	role := jsonFlag(&cobra.Command{
		Use:   "role",
		Short: "Tell whether the configuration of the data folder is yours or someone else's",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			r, err := describe()
			if err != nil {
				return err
			}
			return print(r)
		},
	})
	var owner, auto bool
	set := jsonFlag(&cobra.Command{
		Use:   "set-role",
		Short: "Say whether the configuration is yours (--owner) or someone else's (--owner=false); --auto guesses again",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			switch {
			case auto:
				if err := os.Remove(config.LocalPath(root)); err != nil && !os.IsNotExist(err) {
					return err
				}
			case cmd.Flags().Changed("owner"):
				if err := config.SaveLocal(root, config.Local{Owner: &owner}); err != nil {
					return err
				}
			default:
				return fmt.Errorf("give --owner, --owner=false or --auto")
			}
			r, err := describe()
			if err != nil {
				return err
			}
			return print(r)
		},
	})
	set.Flags().BoolVar(&owner, "owner", true, "the configuration is yours")
	set.Flags().BoolVar(&auto, "auto", false, "forget the choice: a git clone is the owner's, the installer's copy someone else's")
	return []*cobra.Command{role, set}
}
