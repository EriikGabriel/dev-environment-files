package engine

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
)

var (
	ubuntu = platform.Info{OS: "linux", Arch: "amd64", ID: "ubuntu", Like: []string{"debian"}, Family: "debian"}
	cachy  = platform.Info{OS: "linux", Arch: "amd64", ID: "cachyos", Like: []string{"arch"}, Family: "arch"}
)

// realConfig loads the repository's own config so tests catch YAML mistakes.
func realConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestResolveNamesPerDistro(t *testing.T) {
	cfg := realConfig(t)
	tests := []struct {
		app    string
		info   platform.Info
		method string
		name   string
	}{
		{"fd", ubuntu, "native", "fd-find"},
		{"fd", cachy, "native", "fd"},
		{"redis", ubuntu, "native", "redis-server"},
		{"redis", cachy, "native", "redis"},
		{"openjdk-21", cachy, "native", "jdk21-openjdk"},
		{"git", ubuntu, "native", "git"}, // "*" fallback
		{"google-chrome", ubuntu, "deb-url", "google-chrome-stable"},
		{"google-chrome", cachy, "aur", "google-chrome"},
		{"wezterm", ubuntu, "flatpak", "org.wezfurlong.wezterm"},
		{"wezterm", cachy, "native", "wezterm"},
		{"jetbrains-mono-nerd", ubuntu, "script", ""},
		{"jetbrains-mono-nerd", cachy, "native", "ttf-jetbrains-mono-nerd"},
	}
	for _, tt := range tests {
		t.Run(tt.app+"/"+tt.info.ID, func(t *testing.T) {
			r, err := Resolve(tt.app, cfg.Apps[tt.app], tt.info)
			if err != nil {
				t.Fatal(err)
			}
			if r.Method != tt.method || r.Name != tt.name {
				t.Errorf("got method=%q name=%q, want method=%q name=%q", r.Method, r.Name, tt.method, tt.name)
			}
		})
	}
}

func TestResolveUnavailableAndSourceExpansion(t *testing.T) {
	cfg := realConfig(t)

	_, err := Resolve("python-venv", cfg.Apps["python-venv"], cachy)
	if _, ok := err.(Unavailable); !ok {
		t.Errorf("python-venv on arch: err = %v, want Unavailable", err)
	}

	r, err := Resolve("vscode", cfg.Apps["vscode"], ubuntu)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(r.Source, "os=linux-deb-x64") {
		t.Errorf("amd64 source = %q, want x64 suffix", r.Source)
	}
	arm := ubuntu
	arm.Arch = "arm64"
	r, _ = Resolve("vscode", cfg.Apps["vscode"], arm)
	if !strings.HasSuffix(r.Source, "os=linux-deb-arm64") {
		t.Errorf("arm64 source = %q, want arm64 suffix", r.Source)
	}
}

func TestResolveRejectsWrongFamilyMethod(t *testing.T) {
	app := config.App{
		Method: map[string]string{"*": "aur"},
		Names:  map[string]string{"*": "x"},
	}
	if _, err := Resolve("x", app, ubuntu); err == nil {
		t.Error("aur on ubuntu should be unavailable")
	}
}

// fakeRunner records commands and answers Installed queries from a set.
type fakeRunner struct {
	installed map[string]bool // package names considered installed
	ran       [][]string
	has       map[string]bool
}

func (f *fakeRunner) Run(_ context.Context, _ io.Writer, cmd ...string) error {
	f.ran = append(f.ran, cmd)
	return nil
}

func (f *fakeRunner) Output(_ context.Context, cmd ...string) (string, error) {
	last := cmd[len(cmd)-1]
	if f.installed[last] {
		if cmd[0] == "dpkg-query" {
			return "install ok installed", nil
		}
		return "ok", nil
	}
	return "", os.ErrNotExist
}

func (f *fakeRunner) Has(bin string) bool { return f.has[bin] }

func TestBuildPlanAndExecuteIsIdempotent(t *testing.T) {
	cfg := &config.Config{
		Apps: map[string]config.App{
			"fd":  {Names: map[string]string{"debian": "fd-find", "arch": "fd"}},
			"git": {Names: map[string]string{"*": "git"}},
		},
		Profiles: map[string]config.Profile{"t": {Apps: []string{"git", "fd"}}},
	}
	fr := &fakeRunner{installed: map[string]bool{"git": true}}
	env := &Env{Info: cachy, Root: t.TempDir(), Home: t.TempDir(), Runner: fr}

	p, err := BuildPlan(cfg, "t", env, Options{})
	if err != nil {
		t.Fatal(err)
	}
	res := p.Execute(context.Background(), io.Discard, func(Event) {})
	if res.Done != 1 || res.UpToDate != 1 || res.Failed != 0 {
		t.Fatalf("result = %+v, want 1 done, 1 up to date", res)
	}
	// Refresh (-Syu) once, then install only the missing package.
	var joined []string
	for _, c := range fr.ran {
		joined = append(joined, strings.Join(c, " "))
	}
	got := strings.Join(joined, "|")
	if !strings.Contains(got, "pacman -Syu") || !strings.Contains(got, "pacman -S --needed --noconfirm fd") ||
		strings.Contains(got, "noconfirm git") {
		t.Errorf("unexpected commands: %s", got)
	}

	// Second run: everything is installed now, so no install command may run.
	fr.installed["fd"] = true
	fr.ran = nil
	res = p.Execute(context.Background(), io.Discard, func(Event) {})
	if res.Done != 0 || res.UpToDate != 2 || len(fr.ran) != 0 {
		t.Errorf("second run: %+v, commands %v", res, fr.ran)
	}
}

func TestBuildPlanUnsupportedPlatform(t *testing.T) {
	cfg := &config.Config{Profiles: map[string]config.Profile{"t": {}}}
	env := &Env{Info: platform.Info{OS: "windows", Arch: "amd64", Family: "windows"}}
	if _, err := BuildPlan(cfg, "t", env, Options{}); err == nil {
		t.Error("windows should be reported as unsupported for now")
	}
}

func TestFileStepLinkIsIdempotentAndBacksUp(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	src := filepath.Join(root, "dotfiles", "rc")
	must(t, os.MkdirAll(filepath.Dir(src), 0o755))
	must(t, os.WriteFile(src, []byte("new"), 0o644))
	dst := filepath.Join(home, ".config", "rc")
	must(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	must(t, os.WriteFile(dst, []byte("old"), 0o644))

	s := &fileStep{src: src, dst: dst, mode: "link"}
	if pending, _ := s.Pending(context.Background()); !pending {
		t.Fatal("existing different file should be pending")
	}
	must(t, s.Run(context.Background(), io.Discard))
	if pending, _ := s.Pending(context.Background()); pending {
		t.Error("after linking, step should be up to date")
	}
	if target, _ := os.Readlink(dst); target != src {
		t.Errorf("link target = %q, want %q", target, src)
	}
	baks, _ := filepath.Glob(dst + ".envsetup-*.bak")
	if len(baks) != 1 {
		t.Fatalf("want exactly one backup, got %v", baks)
	}
	if b, _ := os.ReadFile(baks[0]); string(b) != "old" {
		t.Errorf("backup content = %q, want old", b)
	}
}

func TestFileStepCopy(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a"), filepath.Join(dir, "sub", "b")
	must(t, os.WriteFile(src, []byte("x"), 0o644))
	s := &fileStep{src: src, dst: dst, mode: "copy"}
	must(t, s.Run(context.Background(), io.Discard))
	if pending, _ := s.Pending(context.Background()); pending {
		t.Error("copied file with same content should be up to date")
	}
}

func TestVSCodeStepSkipsBuiltins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ext.json")
	must(t, os.WriteFile(path, []byte(`[
		{"identifier":{"id":"vscode.bat"}},
		{"identifier":{"id":"esbenp.prettier-vscode"}},
		{"identifier":{"id":"Dracula-Theme.theme"}}]`), 0o644))
	fr := &fakeRunner{has: map[string]bool{"code": true}}
	s := &vscodeStep{env: &Env{Runner: fr}, path: path}
	got, err := s.wanted()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "esbenp.prettier-vscode,Dracula-Theme.theme" {
		t.Errorf("wanted = %v", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func planTitles(t *testing.T, profile string, info platform.Info, opt Options) (*Plan, string) {
	t.Helper()
	env := &Env{Info: info, Root: "/repo", Home: "/home/u", Runner: &fakeRunner{}}
	p, err := BuildPlan(realConfig(t), profile, env, opt)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Title())
	}
	return p, "," + strings.Join(out, ",") + ","
}

func TestBuildPlanDefaultsAndChoices(t *testing.T) {
	// CachyOS defaults: fish, kitty and zed; optional apps stay out.
	p, got := planTitles(t, "dev-cachyos", cachy, Options{})
	for _, want := range []string{",fish,", ",kitty,", ",zed,", ",git,", ".config/zed/settings.json", ".config/niri,",
		",godothub,", ",mactahoe-icons,", ",/home/u/www/projects,", "www/projects/api collections"} {
		if !strings.Contains(got, want) {
			t.Errorf("default cachyos plan misses %q", want)
		}
	}
	for _, not := range []string{",htop,", ",vscode,", ",wezterm,", ".zshrc", "Code/User"} {
		if strings.Contains(got, not) {
			t.Errorf("default cachyos plan should not have %q", not)
		}
	}
	if p.Choices["editor"] != "zed" || p.Choices["shell"] != "fish" {
		t.Errorf("choices = %v", p.Choices)
	}

	// Explicit selection: exactly the listed toggles, a different editor, no shell.
	p, got = planTitles(t, "dev-cachyos", cachy, Options{
		Apps:    []string{"git", "htop"},
		Choices: map[string]string{"editor": "vscode", "shell": "none"},
	})
	// directories are created before the files that may live inside them
	if strings.Index(got, ",/home/u/www/projects,") > strings.Index(got, ".config/kitty/kitty.conf") {
		t.Error("dirs must come before files")
	}
	for _, want := range []string{",git,", ",htop,", ",vscode,", "vscode extensions", ",kitty,"} {
		if !strings.Contains(got, want) {
			t.Errorf("custom plan misses %q in %s", want, got)
		}
	}
	for _, not := range []string{",curl,", ",fish,", ",zed,", "config.fish", ".config/niri,", "fastfetch", "api collections"} {
		if strings.Contains(got, not) {
			t.Errorf("custom plan should not have %q", not)
		}
	}
	if p.Choices["shell"] != "none" {
		t.Errorf("shell choice = %q", p.Choices["shell"])
	}

	env := &Env{Info: cachy, Runner: &fakeRunner{}}
	for _, bad := range []map[string]string{{"editor": "emacs"}, {"nope": "x"}} {
		if _, err := BuildPlan(realConfig(t), "dev-cachyos", env, Options{Choices: bad}); err == nil {
			t.Errorf("choices %v must be an error", bad)
		}
	}
}

func TestUbuntuProfileFollowsLegacy(t *testing.T) {
	p, got := planTitles(t, "dev-ubuntu", ubuntu, Options{})
	for _, want := range []string{",zsh,", ",powerlevel10k,", ",wezterm,", ",vscode,", ",google-chrome,",
		",whitesur-gtk-theme,", ",gnome-extensions,", ",gnome-dock-apps,", ".zshrc", ".wezterm.lua"} {
		if !strings.Contains(got, want) {
			t.Errorf("ubuntu plan misses %q", want)
		}
	}
	if len(p.Skipped) != 0 {
		t.Errorf("every default ubuntu app should resolve, skipped: %+v", p.Skipped)
	}
	// The dock is configured after the apps it pins.
	if strings.Index(got, ",gnome-dock-apps,") < strings.Index(got, ",google-chrome,") {
		t.Error("dock apps must come after the apps they pin")
	}
}

func TestWizardDescribesSteps(t *testing.T) {
	cfg := realConfig(t)
	env := &Env{Info: cachy, Root: "/repo", Home: "/home/u", Runner: &fakeRunner{installed: map[string]bool{"git": true}}}
	steps, err := Wizard(context.Background(), cfg, "dev-cachyos", env)
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].ID != "base" || steps[len(steps)-1].ID != "desktop" {
		t.Errorf("unexpected step order: first %q last %q", steps[0].ID, steps[len(steps)-1].ID)
	}
	byID := map[string]WizardApp{}
	for _, a := range steps[0].Apps {
		byID[a.ID] = a
	}
	if !byID["git"].Installed || !byID["git"].Default || byID["htop"].Default || byID["curl"].Installed {
		t.Errorf("app states wrong: git=%+v htop=%+v curl=%+v", byID["git"], byID["htop"], byID["curl"])
	}
}
