package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/policy"
)

// PoliciesPath is where the group-policy list lives in the repo.
func PoliciesPath(root string) string { return filepath.Join(root, "config", "policies.yaml") }

// PolicyState reports, for every policy, whether the registry already
// matches its choice.
func (e *Env) PolicyState(ctx context.Context, f *policy.File) (map[string]bool, error) {
	out, err := e.Runner.Output(ctx, e.script(policy.StateScript(f.Policies))...)
	if err != nil {
		return nil, fmt.Errorf("read the policies from the registry: %w", err)
	}
	return policy.ParseState(out)
}

func (e *Env) policiesApplied(ctx context.Context) bool {
	f, err := policy.Load(PoliciesPath(e.Root))
	if err != nil {
		return false
	}
	st, err := e.PolicyState(ctx, f)
	if err != nil {
		return false
	}
	for _, p := range f.Policies {
		if !st[p.ID] {
			return false
		}
	}
	return true
}

// ApplyPolicies writes the choice of every policy to the registry. The keys
// are under HKLM, so the script runs elevated: Windows shows one UAC prompt.
func (e *Env) ApplyPolicies(ctx context.Context, out io.Writer) error {
	f, err := policy.Load(PoliciesPath(e.Root))
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "envsetup-policies-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "apply.ps1")
	// The BOM makes Windows PowerShell read the file as UTF-8.
	if err := os.WriteFile(script, append([]byte{0xEF, 0xBB, 0xBF}, policy.ApplyScript(f.Policies)...), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(out, "writing %d group policies to the registry: accept the elevation prompt\n", len(f.Policies))
	if err := e.Runner.Run(ctx, out, e.script(policy.ElevatedCommand(script))...); err != nil {
		return fmt.Errorf("the elevated script failed or the elevation was refused: %w", err)
	}
	st, err := e.PolicyState(ctx, f)
	if err != nil {
		return err
	}
	for _, p := range f.Policies {
		if !st[p.ID] {
			return fmt.Errorf("policy %s is still not as chosen after applying", p.ID)
		}
	}
	return nil
}

// OpenPolicy opens a policy where it can be changed by hand: the Group
// Policy editor at the policy (registry false) or the Registry Editor at the
// key it writes (registry true). See windows/open-policy.ps1.
func (e *Env) OpenPolicy(ctx context.Context, out io.Writer, id string, registry bool) error {
	f, err := policy.Load(PoliciesPath(e.Root))
	if err != nil {
		return err
	}
	var p *policy.Policy
	for i := range f.Policies {
		if f.Policies[i].ID == id {
			p = &f.Policies[i]
		}
	}
	if p == nil {
		return fmt.Errorf("unknown policy %q", id)
	}
	script := filepath.Join(e.Root, "windows", "open-policy.ps1")
	var cmd string
	if registry {
		key := p.Key()
		if key == "" {
			return fmt.Errorf("%s writes no registry value", id)
		}
		cmd = fmt.Sprintf("& %s -Registry -Key %s", psQuote(script), psQuote(key))
	} else {
		cmd = fmt.Sprintf("& %s -Path %s -Title %s", psQuote(script), psQuote(p.Path), psQuote(p.Title))
	}
	return e.Runner.Run(ctx, out, e.script(cmd)...)
}
