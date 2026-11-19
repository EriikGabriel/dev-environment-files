package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/policy"
)

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

// policyCommands list, choose and apply the group policies of config/policies.yaml.
func policyCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)

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
			undo, err := backupFiles(path)
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

	var inRegistry bool
	openPolicy := jsonFlag(&cobra.Command{
		Use:   "open-policy <id>",
		Short: "Open a policy to change it by hand: in the Group Policy editor (elevated), or with --registry in the Registry Editor at its key",
		Long: "gpedit cannot be told where to open: its window is driven to the policy's folder and the policy is opened. " +
			"When a step fails, the editor is left as far as it got and the message says what is left to do.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, env, err := load(*f)
			if err != nil {
				return err
			}
			if env.Info.OS != "windows" {
				return fmt.Errorf("group policies only exist on Windows")
			}
			var report bytes.Buffer
			err = env.OpenPolicy(cmd.Context(), &report, args[0], inRegistry)
			msg := strings.TrimSpace(report.String())
			if err != nil {
				if msg != "" {
					return fmt.Errorf("%s (%w)", msg, err)
				}
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"id": args[0], "message": msg})
			}
			fmt.Println(msg)
			return nil
		},
	})
	openPolicy.Flags().BoolVar(&inRegistry, "registry", false, "open the Registry Editor at the key the policy writes")

	return []*cobra.Command{policies, setPolicy, applyPolicies, openPolicy}
}
