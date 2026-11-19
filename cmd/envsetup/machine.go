package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
)

// machineCommands describe this machine and the profiles that fit it.
func machineCommands(f *flags) []*cobra.Command {
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
			cfg, _, err := load(*f)
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

	steps := &cobra.Command{
		Use:   "steps",
		Short: "Describe the profile's setup steps and the state of each app",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, env, err := load(*f)
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

	return []*cobra.Command{detect, info, profiles, steps}
}
