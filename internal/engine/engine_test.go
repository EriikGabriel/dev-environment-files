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
		",whitesur-gtk-theme,", ",gnome-extensions,", ",dock (ordem dos apps),", ".zshrc", ".wezterm.lua"} {
		if !strings.Contains(got, want) {
			t.Errorf("ubuntu plan misses %q", want)
		}
	}
	if len(p.Skipped) != 0 {
		t.Errorf("every default ubuntu app should resolve, skipped: %+v", p.Skipped)
	}
	// The dock is configured after the apps it pins.
	if strings.Index(got, ",dock (ordem dos apps),") < strings.Index(got, ",google-chrome,") {
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

func TestFindPinned(t *testing.T) {
	text := "[bar]\npinned = [\"x\"]\n\n[dock]\nenabled = true\npinned = [\"a\", \"b.c\",\n  \"d\"]\nauto_hide = true\n"
	start, end, found, vals := findPinned(text)
	if !found || start != 5 || end != 7 || strings.Join(vals, ",") != "a,b.c,d" {
		t.Errorf("got start=%d end=%d found=%v vals=%v", start, end, found, vals)
	}
	if _, _, found, _ := findPinned("[dock]\nenabled = true\n"); found {
		t.Error("no pinned line should report not found")
	}
}

func TestDockStepRewritesThroughSymlink(t *testing.T) {
	home, repo := t.TempDir(), t.TempDir()
	real := filepath.Join(repo, "config.toml")
	must(t, os.WriteFile(real, []byte("[theme]\nmode = \"dark\"\n\n[dock]\nenabled = true\npinned = [\"old\"]\nauto_hide = true\n"), 0o644))
	link := filepath.Join(home, ".config", "noctalia", "config.toml")
	must(t, os.MkdirAll(filepath.Dir(link), 0o755))
	must(t, os.Symlink(real, link))

	s := &dockStep{env: &Env{Home: home, Runner: &fakeRunner{}}, kind: "niri-noctalia", pinned: []string{"kitty", "dev.zed.Zed"}}
	if pending, _ := s.Pending(context.Background()); !pending {
		t.Fatal("different order must be pending")
	}
	must(t, s.Run(context.Background(), io.Discard))
	b, _ := os.ReadFile(real)
	want := "[theme]\nmode = \"dark\"\n\n[dock]\nenabled = true\npinned = [\"kitty\", \"dev.zed.Zed\"]\nauto_hide = true\n"
	if string(b) != want {
		t.Errorf("repo file after apply:\n%s", b)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink must survive the rewrite")
	}
	if pending, _ := s.Pending(context.Background()); pending {
		t.Error("after applying, the dock step must be up to date")
	}
}

func TestPreviewUsesRealEntriesAndIcons(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_DIRS", filepath.Join(home, "share"))
	apps := filepath.Join(home, ".local", "share", "applications")
	must(t, os.MkdirAll(apps, 0o755))
	must(t, os.WriteFile(filepath.Join(apps, "godothub.desktop"), []byte("[Desktop Entry]\nName=GodotHub\nIcon=myicon\nStartupWMClass=godothub\n"), 0o644))
	icons := filepath.Join(home, ".local", "share", "icons", "Tema", "apps", "scalable")
	must(t, os.MkdirAll(icons, 0o755))
	must(t, os.WriteFile(filepath.Join(icons, "myicon.svg"), []byte("<svg/>"), 0o644))
	wp := filepath.Join(home, "wall.png")
	must(t, os.WriteFile(wp, []byte{0x89, 'P', 'N', 'G'}, 0o644))

	cfg := &config.Config{Profiles: map[string]config.Profile{"p": {Desktop: config.Desktop{
		Kind: "gnome", Wallpaper: "~/wall.png", IconTheme: "Tema",
		Dock: config.Dock{Pinned: []string{"GodotHub", "nope"}},
	}}}}
	p, err := Preview(context.Background(), cfg, "p", &Env{Home: home, Root: home})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Dock.Items) != 2 || !p.Dock.Items[0].Found || p.Dock.Items[0].Name != "GodotHub" ||
		!strings.HasPrefix(p.Dock.Items[0].Icon, "data:image/svg+xml;base64,") {
		t.Errorf("first item = %+v", p.Dock.Items[0])
	}
	if p.Dock.Items[1].Found || p.Dock.Items[1].Icon != "" {
		t.Errorf("missing app should be a placeholder: %+v", p.Dock.Items[1])
	}
	if !strings.HasPrefix(p.Wallpaper, "data:image/png;base64,") || p.WallpaperError != "" {
		t.Errorf("wallpaper = %q err %q", p.Wallpaper[:min(24, len(p.Wallpaper))], p.WallpaperError)
	}
}
