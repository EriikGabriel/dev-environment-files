package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func appendTo(t *testing.T, body string, f File) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "p.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendFile(p, f); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	return string(b)
}

func TestAppendFileChangesOnlyOneLine(t *testing.T) {
	in := "name: x\n\n# note\nfiles:\n  - { src: a, dest: ~/.a }\n\n# trailing comment\nother: 1\n"
	out := appendTo(t, in, File{Src: "dotfiles/b", Dest: "~/.b", Mode: "copy"})
	want := "name: x\n\n# note\nfiles:\n  - { src: a, dest: ~/.a }\n  - { src: dotfiles/b, dest: ~/.b, mode: copy }\n\n# trailing comment\nother: 1\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestAppendFileIntoEmptyList(t *testing.T) {
	in := "name: x\n# c\nsteps: []\n\n# extras\nfiles: []\n"
	out := appendTo(t, in, File{Src: "dotfiles/b", Dest: "~/.b"})
	want := "name: x\n# c\nsteps: []\n\n# extras\nfiles:\n  - { src: dotfiles/b, dest: ~/.b }\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	out = appendTo(t, "name: x\n", File{Src: "s", Dest: "~/d"})
	if out != "name: x\n\nfiles:\n  - { src: s, dest: ~/d }\n" {
		t.Errorf("missing key: got %q", out)
	}
}

func TestAppendFileFallbacks(t *testing.T) {
	// no files key at all
	out := appendTo(t, "name: x\n", File{Src: "s", Dest: "~/d"})
	if !strings.Contains(out, "files:") || !strings.Contains(out, "s") {
		t.Errorf("files key not created:\n%s", out)
	}
	// block-style entry: cannot be edited as one line, must still produce valid YAML
	out = appendTo(t, "files:\n  - src: a\n    dest: ~/.a\n", File{Src: "b", Dest: "~/.b"})
	p := filepath.Join(t.TempDir(), "p.yaml")
	_ = os.WriteFile(p, []byte(out), 0o644)
	var prof Profile
	if err := readYAML(p, &prof); err != nil || len(prof.Files) != 2 || prof.Files[1].Src != "b" {
		t.Errorf("block fallback produced %+v, err %v\n%s", prof.Files, err, out)
	}
	// a value that needs quoting survives a round trip
	out = appendTo(t, "files:\n  - { src: a, dest: ~/.a }\n", File{Src: "dotfiles/x: y", Dest: "~/# odd"})
	_ = os.WriteFile(p, []byte(out), 0o644)
	prof = Profile{}
	if err := readYAML(p, &prof); err != nil || prof.Files[1].Src != "dotfiles/x: y" || prof.Files[1].Dest != "~/# odd" {
		t.Errorf("quoting broke: %+v, err %v\n%s", prof.Files, err, out)
	}
}

func TestSetDirsRewritesOneLine(t *testing.T) {
	in := `name: x
steps:
  - id: a
    dirs: [~/one]
  - id: workspace
    # keep
    options:
      - id: www
        title: t
        dirs: [~/www/projects, ~/www/games]
        files: []
`
	p := filepath.Join(t.TempDir(), "p.yaml")
	if err := os.WriteFile(p, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetDirs(p, "workspace", "www", []string{"~/www/projects", "~/www/my notes/", "~/code"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	want := strings.Replace(in, "dirs: [~/www/projects, ~/www/games]", "dirs: [~/www/projects, ~/www/my notes, ~/code]", 1)
	if string(b) != want {
		t.Errorf("got:\n%s", b)
	}
	var prof Profile
	if err := readYAML(p, &prof); err != nil || prof.Steps[1].Options[0].Dirs[1] != "~/www/my notes" {
		t.Errorf("round trip failed: %v %+v", err, prof.Steps)
	}
	if err := SetDirs(p, "a", "", nil); err != nil {
		t.Fatal(err)
	}
	if b, _ = os.ReadFile(p); !strings.Contains(string(b), "  - id: a\n    dirs: []\n") {
		t.Errorf("step-level dirs not emptied:\n%s", b)
	}
	for _, bad := range [][]string{{"/etc"}, {"~/a/../b"}, {"www"}, {"~/"}} {
		if err := SetDirs(p, "workspace", "www", bad); err == nil {
			t.Errorf("%v should be rejected", bad)
		}
	}
	if err := SetDirs(p, "nope", "", nil); err == nil {
		t.Error("unknown step should fail")
	}
}

func TestSetDockPinned(t *testing.T) {
	in := "name: x\ndesktop:\n  kind: gnome\n  dock:\n    position: bottom\n    pinned: [a, b]  \nsteps: []\n"
	p := filepath.Join(t.TempDir(), "p.yaml")
	if err := os.WriteFile(p, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetDockPinned(p, []string{"dev.zed.Zed", "GodotHub", "my app"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if want := "    pinned: [dev.zed.Zed, GodotHub, my app]"; !strings.Contains(string(b), want+"\n") {
		t.Errorf("got:\n%s", b)
	}
	var prof Profile
	if err := readYAML(p, &prof); err != nil || len(prof.Desktop.Dock.Pinned) != 3 {
		t.Errorf("round trip: %v %+v", err, prof.Desktop.Dock)
	}
	for _, bad := range [][]string{{"a", "a"}, {"x.desktop"}, {""}} {
		if err := SetDockPinned(p, bad); err == nil {
			t.Errorf("%v should be rejected", bad)
		}
	}
}

func TestStepAppsAreEditedInPlace(t *testing.T) {
	in := "name: x\nsteps:\n  # first\n  - id: base\n    title: Base\n    apps: [a, b]\n\n  - id: other\n    title: Other # keep\n    description: d\n    optional: [z]\nfiles: []\n"
	p := filepath.Join(t.TempDir(), "p.yaml")
	if err := os.WriteFile(p, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	read := func() string { b, _ := os.ReadFile(p); return string(b) }
	step := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	step(AddStepApp(p, "base", "c", false))
	step(AddStepApp(p, "base", "o", true)) // no optional list yet: a new line
	step(AddStepApp(p, "other", "n", false))
	want := "name: x\nsteps:\n  # first\n  - id: base\n    title: Base\n    apps: [a, b, c]\n    optional: [o]\n\n  - id: other\n    title: Other # keep\n    description: d\n    apps: [n]\n    optional: [z]\nfiles: []\n"
	if got := read(); got != want {
		t.Fatalf("after adding, got:\n%s\nwant:\n%s", got, want)
	}
	if err := AddStepApp(p, "base", "o", false); err == nil {
		t.Error("adding an app twice must fail")
	}
	if err := AddStepApp(p, "nope", "q", false); err == nil {
		t.Error("unknown step must fail")
	}

	step(RemoveStepApp(p, "base", "a"))
	step(RemoveStepApp(p, "base", "o"))
	if err := RemoveStepApp(p, "base", "zzz"); err == nil {
		t.Error("removing an app that is not there must fail")
	}
	if got := read(); !strings.Contains(got, "    apps: [b, c]\n    optional: []\n") {
		t.Errorf("after removing, got:\n%s", got)
	}
	var prof Profile
	if err := readYAML(p, &prof); err != nil || len(prof.Steps) != 2 || len(prof.Steps[1].Apps) != 1 {
		t.Errorf("the result must still load: %+v, %v", prof.Steps, err)
	}
}

func TestAppendApp(t *testing.T) {
	p := filepath.Join(t.TempDir(), "apps.yaml")
	in := "# header\napps:\n  git: { names: { \"*\": git } }\n"
	if err := os.WriteFile(p, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendApp(p, "heroic", "windows", "", "HeroicGamesLauncher.HeroicGamesLauncher", "Epic, GOG: one launcher"); err != nil {
		t.Fatal(err)
	}
	if err := AppendApp(p, "hades", "windows", MethodSteam, "1145360", ""); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	want := in + "  heroic: { description: \"Epic, GOG: one launcher\", names: { windows: HeroicGamesLauncher.HeroicGamesLauncher } }\n" +
		"  hades: { method: { windows: steam }, names: { windows: \"1145360\" } }\n"
	if string(b) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", b, want)
	}
	var cat struct {
		Apps map[string]App `yaml:"apps"`
	}
	if err := readYAML(p, &cat); err != nil || cat.Apps["hades"].Names["windows"] != "1145360" ||
		cat.Apps["heroic"].Description != "Epic, GOG: one launcher" {
		t.Errorf("the catalogue must still load: %+v, %v", cat.Apps, err)
	}
	for _, bad := range [][2]string{{"git", "x"}, {"Bad Id", "x"}, {"ok", ""}} {
		if err := AppendApp(p, bad[0], "windows", "", bad[1], ""); err == nil {
			t.Errorf("AppendApp(%q, %q) must fail", bad[0], bad[1])
		}
	}
}
