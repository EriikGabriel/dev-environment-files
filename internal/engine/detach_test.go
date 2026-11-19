package engine

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

func TestDetachSurvivesDeletingTheDataFolder(t *testing.T) {
	mustHaveSymlink(t)
	root, home, elsewhere := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(p, s string) {
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, []byte(s), 0o644))
	}
	write(filepath.Join(root, "dotfiles", "zshrc"), "zsh")
	write(filepath.Join(root, "dotfiles", "niri", "config.kdl"), "niri")
	write(filepath.Join(root, "dotfiles", "niri", "cfg", "input.kdl"), "input")
	write(filepath.Join(elsewhere, "other"), "other")
	write(filepath.Join(home, ".plain"), "plain")

	link := func(src, dst string) {
		must(t, os.MkdirAll(filepath.Dir(dst), 0o755))
		must(t, os.Symlink(src, dst))
	}
	link(filepath.Join(root, "dotfiles", "zshrc"), filepath.Join(home, ".zshrc"))
	link(filepath.Join(root, "dotfiles", "niri"), filepath.Join(home, ".config", "niri"))
	link(filepath.Join(elsewhere, "other"), filepath.Join(home, ".other"))

	cfg := &config.Config{Profiles: map[string]config.Profile{
		"a": {Files: []config.File{{Src: "dotfiles/zshrc", Dest: "~/.zshrc"}, {Src: "x", Dest: "~/.plain"}}},
		"b": {Steps: []config.Step{{Options: []config.Option{{Files: []config.File{
			{Src: "dotfiles/niri", Dest: "~/.config/niri"},
			{Src: "x", Dest: "~/.other"},
			{Src: "x", Dest: "~/.missing"},
		}}}}}},
	}}
	got, err := Detach(cfg, &Env{Root: root, Home: home}, io.Discard)
	must(t, err)
	if len(got) != 2 {
		t.Fatalf("detached %v, want .zshrc and .config/niri", got)
	}

	must(t, os.RemoveAll(root))

	for p, want := range map[string]string{
		filepath.Join(home, ".zshrc"):                              "zsh",
		filepath.Join(home, ".config", "niri", "config.kdl"):       "niri",
		filepath.Join(home, ".config", "niri", "cfg", "input.kdl"): "input",
		filepath.Join(home, ".plain"):                              "plain",
	} {
		b, err := os.ReadFile(p)
		if err != nil || string(b) != want {
			t.Errorf("%s = %q, %v; want %q", p, b, err, want)
		}
		if fi, _ := os.Lstat(p); fi != nil && fi.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s is still a symlink", p)
		}
	}
	if fi, _ := os.Lstat(filepath.Join(home, ".other")); fi == nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("a link that does not point into the data folder must be left alone")
	}

	// running it again changes nothing
	again, err := Detach(cfg, &Env{Root: root, Home: home}, io.Discard)
	if err != nil || len(again) != 0 {
		t.Errorf("second run = %v, %v", again, err)
	}
}
