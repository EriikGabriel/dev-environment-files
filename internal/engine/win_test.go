package engine

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/testutil"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/policy"
)

// elevatingRunner is a fakeRunner on which the elevated policy script
// "works": after it runs, the registry answers that every policy matches.
type elevatingRunner struct {
	*fakeRunner
	state func(applied bool) string
}

func (r *elevatingRunner) Run(ctx context.Context, out io.Writer, cmd ...string) error {
	if strings.Contains(cmd[len(cmd)-1], "-Verb RunAs") {
		r.answers["ConvertTo-Json -Compress\n"] = r.state(true)
	}
	return r.fakeRunner.Run(ctx, out, cmd...)
}

func TestNiniteAndPoliciesOnWindows(t *testing.T) {
	root := testutil.RepoRoot(t)
	file, err := policy.Load(PoliciesPath(root))
	must(t, err)
	state := func(applied bool) string {
		st := map[string]bool{}
		for _, p := range file.Policies {
			st[p.ID] = applied
		}
		b, _ := json.Marshal(st)
		return string(b)
	}
	fr := &elevatingRunner{
		fakeRunner: &fakeRunner{answers: map[string]string{"ConvertTo-Json -Compress\n": state(false)}},
		state:      state,
	}
	var fetched string
	env := &Env{Info: win, Root: root, Home: t.TempDir(), Runner: fr, StateDir: t.TempDir(),
		Fetch: func(_ context.Context, url, dest string) error {
			fetched = url
			return os.WriteFile(dest, []byte("MZ"), 0o644)
		}}
	opt := Options{Apps: []string{"ninite", "group-policies"}, NoFiles: true}
	cfg := realConfig(t)
	p, err := BuildPlan(cfg, "gaming-windows", env, opt)
	must(t, err)
	if len(p.Steps) != 2 || len(p.Skipped) != 0 {
		t.Fatalf("want the two items, got %d steps, skipped %+v", len(p.Steps), p.Skipped)
	}
	lines := p.Describe(context.Background())
	if !lines[0].Pending || !strings.Contains(lines[0].Detail, "Ninite installer for") || !lines[1].Pending {
		t.Errorf("both start pending: %+v", lines)
	}

	res := p.Execute(context.Background(), io.Discard, func(Event) {})
	if res.Failed != 0 || res.Done != 2 {
		t.Fatalf("both should run: %+v", res)
	}
	slugs := strings.Fields(cfg.Apps["ninite"].Names["windows"])
	if fetched != "https://ninite.com/"+strings.Join(slugs, "-")+"/ninite.exe" {
		t.Errorf("wrong installer downloaded: %s", fetched)
	}
	ran := ranLines(fr.fakeRunner)
	if !strings.Contains(ran, "Start-Process -FilePath '") || !strings.Contains(ran, "NiniteInstaller.exe' -Wait") {
		t.Errorf("Ninite must be started through the shell, to get the elevation prompt:\n%s", ran)
	}
	if !strings.Contains(ran, "-Verb RunAs") || !strings.Contains(ran, "apply.ps1") {
		t.Errorf("the policy script must run elevated:\n%s", ran)
	}

	// A second run has nothing to do: Ninite ran for this list and the
	// registry matches.
	fr.ran = nil
	res = p.Execute(context.Background(), io.Discard, func(Event) {})
	if res.UpToDate != 2 || len(fr.ran) != 0 {
		t.Errorf("second run must be a no-op: %+v\n%s", res, ranLines(fr.fakeRunner))
	}
	// A different list makes Ninite pending again.
	if env.niniteDone("chrome") {
		t.Error("the marker is for one exact list")
	}
}

func TestNiniteAndPoliciesAreWindowsOnly(t *testing.T) {
	cfg := realConfig(t)
	for _, id := range []string{"ninite", "group-policies"} {
		if _, err := Resolve(id, cfg.Apps[id], ubuntu); err == nil {
			t.Errorf("%s must not resolve on Linux", id)
		}
	}
	bad := config.App{Method: map[string]string{"windows": config.MethodNinite}, Names: map[string]string{"windows": "steam; calc"}}
	if _, err := Resolve("x", bad, win); err == nil {
		t.Error("a list that is not Ninite app names must not resolve")
	}
}

func TestSetAppNameEditsOneLine(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(testutil.RepoRoot(t), "config", "apps.yaml"))
	must(t, err)
	path := filepath.Join(t.TempDir(), "apps.yaml")
	must(t, os.WriteFile(path, src, 0o644))
	must(t, config.SetAppName(path, "ninite", "windows", ".net8 chrome"))
	got, _ := os.ReadFile(path)
	a, b := strings.Split(string(src), "\n"), strings.Split(string(got), "\n")
	changed := []string{}
	for i := range a {
		if i < len(b) && a[i] != b[i] {
			changed = append(changed, strings.TrimSpace(b[i]))
		}
	}
	if len(a) != len(b) || len(changed) != 1 || changed[0] != `names: { windows: ".net8 chrome" }` {
		t.Errorf("want only the names line changed, got %q", changed)
	}
	// an entry written on a single line cannot be edited this way
	if err := config.SetAppName(path, "steam", "windows", "x"); err == nil {
		t.Error("a one-line catalogue entry must be refused, not mangled")
	}
}
