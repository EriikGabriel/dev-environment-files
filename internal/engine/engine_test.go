package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
	"github.com/EriikGabriel/dev-environment-files/internal/testutil"
)

var (
	ubuntu = platform.Info{OS: "linux", Arch: "amd64", ID: "ubuntu", Like: []string{"debian"}, Family: "debian", Codename: "noble"}
	cachy  = platform.Info{OS: "linux", Arch: "amd64", ID: "cachyos", Like: []string{"arch"}, Family: "arch"}
	win    = platform.Info{OS: "windows", Arch: "amd64", ID: "windows", Family: "windows", Name: "Windows"}
)

// realConfig loads the repository's own config so tests catch YAML mistakes.
func realConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(testutil.RepoRoot(t))
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
	answers   map[string]string // Output for commands whose last argument ends with the key
}

func (f *fakeRunner) Run(_ context.Context, _ io.Writer, cmd ...string) error {
	f.ran = append(f.ran, cmd)
	return nil
}

func (f *fakeRunner) Output(_ context.Context, cmd ...string) (string, error) {
	last := cmd[len(cmd)-1]
	for suffix, answer := range f.answers {
		if strings.HasSuffix(last, suffix) {
			return answer, nil
		}
	}
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
	env := &Env{Info: platform.Info{OS: "darwin", Arch: "arm64", Family: "unknown"}}
	if _, err := BuildPlan(cfg, "t", env, Options{}); err == nil {
		t.Error("macOS should be reported as unsupported")
	}
}

func TestFileStepLinkIsIdempotentAndBacksUp(t *testing.T) {
	mustHaveSymlink(t)
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

// mustHaveSymlink skips a test that needs symlinks where they cannot be
// made: on Windows without Developer Mode or elevation (like the standard
// library's testenv.MustHaveSymlink).
func mustHaveSymlink(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks are not allowed here (on Windows, turn on Developer Mode to run this test): %v", err)
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
		// paths are joined with the separator of the machine running the
		// test; compare them in the Linux form the profiles are written in
		out = append(out, filepath.ToSlash(s.Title()))
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
	mustHaveSymlink(t)
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

func ranLines(fr *fakeRunner) string {
	var out []string
	for _, c := range fr.ran {
		out = append(out, strings.Join(c, " "))
	}
	return strings.Join(out, "\n")
}

func TestGamingProfileOnWindows(t *testing.T) {
	steam := t.TempDir()
	fr := &fakeRunner{has: map[string]bool{"winget": true}}
	env := &Env{Info: win, Root: "/repo", Home: t.TempDir(), Runner: fr, SteamDir: steam}
	p, err := BuildPlan(realConfig(t), "gaming-windows", env, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Skipped) != 0 {
		t.Fatalf("every default gaming app should resolve on Windows, skipped: %+v", p.Skipped)
	}
	var titles []string
	for _, s := range p.Steps {
		titles = append(titles, s.Title())
	}
	got := "," + strings.Join(titles, ",") + ","
	for _, want := range []string{",steam,", ",amd-software,", ",opera-gx,", ",discord,", ",epic-games,", ",wallpaper-engine,"} {
		if !strings.Contains(got, want) {
			t.Errorf("gaming plan misses %q in %s", want, got)
		}
	}
	if strings.Contains(got, ",gog-galaxy,") {
		t.Error("optional apps must stay out by default")
	}

	// Steam is not there yet, so the Steam app fails and the rest goes on.
	// The items that restore data kept outside the catalogue (the Windhawk
	// export, the Playnite and RetroArch snapshots) have none under the fake
	// root and may fail too, depending on what this machine has (a clean CI
	// runner has none of them).
	res := p.Execute(context.Background(), io.Discard, func(Event) {})
	var failed []string
	for _, f := range res.Failures {
		if f.Title != "windhawk-mods" && f.Title != "playnite" && f.Title != "retroarch" {
			failed = append(failed, f.Title)
		}
	}
	if strings.Join(failed, ",") != "wallpaper-engine" {
		t.Fatalf("only wallpaper-engine should fail without Steam: %+v", res)
	}
	ran := ranLines(fr)
	for _, want := range []string{
		"winget install --id Valve.Steam --exact --silent",
		"winget install --id Opera.OperaGX --exact",
		"powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command $ErrorActionPreference = 'Stop'; $env:ENVSETUP_ROOT = '/repo'; ",
	} {
		if !strings.Contains(ran, want) {
			t.Errorf("missing command %q in:\n%s", want, ran)
		}
	}
	if strings.Contains(ran, "sudo") || strings.Contains(ran, "sh -c") {
		t.Errorf("no sudo and no sh on Windows:\n%s", ran)
	}

	// With Steam installed the install dialog is requested...
	must(t, os.WriteFile(filepath.Join(steam, "steam.exe"), nil, 0o644))
	fr.ran = nil
	p.Execute(context.Background(), io.Discard, func(Event) {})
	if !strings.Contains(ranLines(fr), "cmd /c start  steam://install/431960") {
		t.Errorf("steam install not requested:\n%s", ranLines(fr))
	}
	// ...and an app found in a library on another drive counts as installed.
	lib := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(steam, "steamapps"), 0o755))
	must(t, os.MkdirAll(filepath.Join(lib, "steamapps"), 0o755))
	vdf := "\"libraryfolders\"\n{\n\t\"1\"\n\t{\n\t\t\"path\"\t\t\"" + strings.ReplaceAll(lib, `\`, `\\`) + "\"\n\t}\n}\n"
	must(t, os.WriteFile(filepath.Join(steam, "steamapps", "libraryfolders.vdf"), []byte(vdf), 0o644))
	must(t, os.WriteFile(filepath.Join(lib, "steamapps", "appmanifest_431960.acf"), nil, 0o644))
	if !env.steamInstalled("431960") || env.steamInstalled("1") {
		t.Error("steam library lookup is wrong")
	}
}

func TestWindowsIgnoresWildcardEntries(t *testing.T) {
	cfg := realConfig(t)
	for _, id := range []string{"curl", "zed", "wezterm"} {
		if _, err := Resolve(id, cfg.Apps[id], win); err == nil {
			t.Errorf("%s has no windows entry and must not resolve there", id)
		}
	}
	if r, err := Resolve("discord", cfg.Apps["discord"], win); err != nil || r.Name != "Discord.Discord" || r.Method != config.MethodNative {
		t.Errorf("discord on windows = %+v, %v", r, err)
	}
	if _, err := Resolve("wallpaper-engine", cfg.Apps["wallpaper-engine"], cachy); err == nil {
		t.Error("the steam method must be Windows only")
	}
}

func TestAptRepoIsAddedBeforeInstall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "-----BEGIN PGP PUBLIC KEY BLOCK-----\n")
	}))
	defer srv.Close()
	cfg := &config.Config{
		Apps: map[string]config.App{"tool": {
			Names: map[string]string{"debian": "tool-ce tool-cli"},
			Repo:  map[string]config.Repo{"ubuntu": {Key: srv.URL + "/gpg", Source: "https://example.com/linux/ubuntu {codename} stable"}},
		}},
		Profiles: map[string]config.Profile{"t": {Apps: []string{"tool"}}},
	}
	fr := &fakeRunner{}
	apt := t.TempDir()
	env := &Env{Info: ubuntu, Root: t.TempDir(), Home: t.TempDir(), Runner: fr, AptDir: apt}
	p, err := BuildPlan(cfg, "t", env, Options{})
	if err != nil || len(p.Skipped) != 0 {
		t.Fatalf("plan: %v, skipped %+v", err, p.Skipped)
	}
	if _, detail := p.Steps[0].Pending(context.Background()); !strings.Contains(detail, "adds its apt repository") {
		t.Errorf("detail = %q", detail)
	}
	var log strings.Builder
	if res := p.Execute(context.Background(), &log, func(Event) {}); res.Failed != 0 {
		t.Fatalf("failed: %+v", res.Failures)
	}
	ran := ranLines(fr)
	keyring := filepath.Join(apt, "keyrings", "envsetup-tool.asc")
	list := filepath.Join(apt, "sources.list.d", "envsetup-tool.list")
	iKey, iList := strings.Index(ran, keyring), strings.Index(ran, list)
	iUpdate, iInstall := strings.Index(ran, "apt-get update"), strings.Index(ran, "apt-get install -y tool-ce tool-cli")
	if iKey < 0 || iList < iKey || iUpdate < iList || iInstall < iUpdate {
		t.Errorf("wrong commands or order:\n%s", ran)
	}
	want := "deb [arch=amd64 signed-by=" + keyring + "] https://example.com/linux/ubuntu noble stable"
	if !strings.Contains(log.String(), want) {
		t.Errorf("source line: want %q in %q", want, log.String())
	}

	// Without a codename the repository cannot be written, so the app is skipped.
	noCode := ubuntu
	noCode.Codename = ""
	p, _ = BuildPlan(cfg, "t", &Env{Info: noCode, Runner: fr}, Options{})
	if len(p.Skipped) != 1 {
		t.Errorf("an unknown codename must skip the app: %+v", p.Skipped)
	}
}

func TestVarsReachScripts(t *testing.T) {
	env := &Env{Info: cachy, Root: "/repo", Vars: map[string]string{"git.name": "Ana d'Ávila", "git.email": "a@b.c"}}
	cmd := env.script("echo hi")
	if cmd[0] != "sh" || !strings.Contains(cmd[2], `export ENVSETUP_GIT_EMAIL='a@b.c'; export ENVSETUP_GIT_NAME='Ana d'\''Ávila'; echo hi`) {
		t.Errorf("sh script = %q", cmd)
	}
	env.Info = win
	cmd = env.script("echo hi")
	if cmd[0] != "powershell" || !strings.Contains(cmd[len(cmd)-1], `$env:ENVSETUP_GIT_NAME = 'Ana d''Ávila'; echo hi`) {
		t.Errorf("powershell script = %q", cmd)
	}

	cfg := &config.Config{Profiles: map[string]config.Profile{"t": {}}}
	if _, err := BuildPlan(cfg, "t", &Env{Info: cachy}, Options{Vars: map[string]string{"bad key;": "x"}}); err == nil {
		t.Error("a --set key with odd characters must be rejected")
	}
}

func TestIdentityStep(t *testing.T) {
	fr := &fakeRunner{answers: map[string]string{"git config --global user.name": "Ana\n"}}
	env := &Env{Info: cachy, Root: "/repo", Home: "/home/u", Runner: fr}
	steps, err := Wizard(context.Background(), realConfig(t), "dev-cachyos", env)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, s := range steps {
		if s.ID != "identity" {
			continue
		}
		found = true
		if len(s.Inputs) != 2 || s.Inputs[0].ID != "git.name" || s.Inputs[0].Value != "Ana" || s.Inputs[1].Value != "" {
			t.Errorf("inputs = %+v", s.Inputs)
		}
	}
	if !found {
		t.Error("dev-cachyos has no identity step")
	}
	_, got := planTitles(t, "dev-ubuntu", ubuntu, Options{})
	for _, want := range []string{",git-identity,", ",ssh-key,", ",gh-auth,", ",webkit2gtk,", ",docker,", ",nodejs,"} {
		if !strings.Contains(got, want) {
			t.Errorf("ubuntu plan misses %q", want)
		}
	}
}
