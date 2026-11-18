package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

func setupRepo(t *testing.T) (root, home string) {
	t.Helper()
	root, home = t.TempDir(), t.TempDir()
	must(t, os.MkdirAll(filepath.Join(root, "config", "profiles"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "config", "apps.yaml"), []byte("apps:\n  git: { names: { \"*\": git } }\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "config", "profiles", "dev.yaml"), []byte(`name: dev
apps: [git]
# keep this comment
files:
  - { src: dotfiles/a, dest: ~/.a }
`), 0o644))
	return root, home
}

func TestCaptureDirectoryLinksAndRegisters(t *testing.T) {
	mustHaveSymlink(t)
	root, home := setupRepo(t)
	dir := filepath.Join(home, ".config", "foo")
	must(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "sub", "x.conf"), []byte("hello"), 0o600))

	res, err := Capture(root, home, "~/.config/foo", AddOptions{Profile: "dev", Link: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Src != "dotfiles/foo" || res.Dest != "~/.config/foo" || res.Mode != "link" {
		t.Errorf("result = %+v", res)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "dotfiles", "foo", "sub", "x.conf")); string(b) != "hello" {
		t.Errorf("repo copy content = %q", b)
	}
	if target, _ := os.Readlink(dir); target != filepath.Join(root, "dotfiles", "foo") {
		t.Errorf("original is not linked into the repo: %q", target)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "sub", "x.conf")); string(b) != "hello" {
		t.Error("file must stay readable through the symlink")
	}
	if m, _ := filepath.Glob(dir + "*.bak"); len(m) != 0 {
		t.Errorf("backup left behind: %v", m)
	}

	// The profile keeps its comment and now has the entry; apply sees it as up to date.
	raw, _ := os.ReadFile(filepath.Join(root, "config", "profiles", "dev.yaml"))
	if !strings.Contains(string(raw), "# keep this comment") {
		t.Errorf("comment lost:\n%s", raw)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	files := cfg.Profiles["dev"].Files
	if len(files) != 2 || files[1].Dest != "~/.config/foo" {
		t.Fatalf("files = %+v", files)
	}
	step := &fileStep{src: filepath.Join(root, files[1].Src), dst: filepath.Join(home, ".config", "foo"), mode: "link"}
	if pending, _ := step.Pending(context.Background()); pending {
		t.Error("captured file should already be up to date")
	}
}

func TestCaptureRejects(t *testing.T) {
	root, home := setupRepo(t)
	must(t, os.WriteFile(filepath.Join(home, ".a"), []byte("1"), 0o644))
	must(t, os.WriteFile(filepath.Join(home, ".b"), []byte("1"), 0o644))

	if _, err := Capture(root, home, "~/.a", AddOptions{Profile: "dev"}); err == nil {
		t.Error("already managed dest must be rejected")
	}
	if _, err := Capture(root, home, "/etc/hosts", AddOptions{Profile: "dev"}); err == nil {
		t.Error("path outside home must be rejected")
	}
	if _, err := Capture(root, home, "~/.b", AddOptions{Profile: "nope"}); err == nil {
		t.Error("unknown profile must be rejected")
	}
	// A failed capture must not touch the original.
	if b, _ := os.ReadFile(filepath.Join(home, ".b")); string(b) != "1" {
		t.Error("original modified by a rejected capture")
	}
}

func TestCaptureCopyModeLeavesOriginal(t *testing.T) {
	root, home := setupRepo(t)
	must(t, os.WriteFile(filepath.Join(home, ".zshrc"), []byte("z"), 0o644))
	res, err := Capture(root, home, "~/.zshrc", AddOptions{Profile: "dev", Group: "zsh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Src != "dotfiles/zsh/.zshrc" || res.Mode != "copy" {
		t.Errorf("result = %+v", res)
	}
	if fi, _ := os.Lstat(filepath.Join(home, ".zshrc")); fi.Mode()&os.ModeSymlink != 0 {
		t.Error("copy mode must not replace the original with a symlink")
	}
}

func TestViewStaysInsideRepo(t *testing.T) {
	mustHaveSymlink(t)
	root := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(root, "dotfiles", "d"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "dotfiles", "d", "a.conf"), []byte("x=1\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "dotfiles", "bin"), []byte{0, 1, 2}, 0o644))
	outside := filepath.Join(t.TempDir(), "secret")
	must(t, os.WriteFile(outside, []byte("s"), 0o644))
	must(t, os.Symlink(outside, filepath.Join(root, "dotfiles", "leak")))

	v, err := View(root, "dotfiles/d/a.conf")
	if err != nil || v.Kind != "text" || v.Content != "x=1\n" {
		t.Errorf("text view = %+v, %v", v, err)
	}
	v, err = View(root, "dotfiles")
	if err != nil || v.Kind != "dir" || len(v.Entries) != 3 || !v.Entries[0].Dir {
		t.Errorf("dir view = %+v, %v", v, err)
	}
	if v, _ = View(root, "dotfiles/bin"); v.Kind != "binary" {
		t.Errorf("binary kind = %q", v.Kind)
	}
	for _, bad := range []string{"../x", "dotfiles/../../etc/passwd", "/etc/passwd", "dotfiles/leak"} {
		if _, err := View(root, bad); err == nil {
			t.Errorf("View(%q) must be refused", bad)
		}
	}
}
