package engine

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

// AddOptions control Capture.
type AddOptions struct {
	Profile string
	// Group is the folder under dotfiles/ the file is stored in. Empty means
	// it is derived from the path (~/.config/<tool>/... uses <tool>).
	Group string
	// Link replaces the original with a symlink into the repo (default true
	// in the CLI). When false the original is left untouched.
	Link bool
}

// AddResult describes what Capture did.
type AddResult struct {
	Src  string `json:"src"`  // path in the repo, relative to the root
	Dest string `json:"dest"` // original location, with ~
	Mode string `json:"mode"`
}

// Capture moves a config file or directory from the host into dotfiles/ and
// registers it in the profile, so the next apply deploys it everywhere.
func Capture(root, home, path string, opt AddOptions) (AddResult, error) {
	abs, err := filepath.Abs(expandHome(path, home))
	if err != nil {
		return AddResult{}, err
	}
	rel, err := filepath.Rel(home, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return AddResult{}, fmt.Errorf("%s is not inside your home directory", abs)
	}

	profilePath := filepath.Join(root, "config", "profiles", opt.Profile+".yaml")
	if _, err := os.Stat(profilePath); err != nil {
		return AddResult{}, fmt.Errorf("unknown profile %q", opt.Profile)
	}
	dest := "~/" + filepath.ToSlash(rel)

	cfg, err := config.Load(root)
	if err != nil {
		return AddResult{}, err
	}
	for _, f := range cfg.Profiles[opt.Profile].Files {
		if f.Dest == dest {
			return AddResult{}, fmt.Errorf("%s is already managed by profile %q", dest, opt.Profile)
		}
	}

	fi, err := os.Lstat(abs)
	if err != nil {
		return AddResult{}, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return AddResult{}, fmt.Errorf("%s is already a symlink; resolve it first", abs)
	}

	group, inner := splitGroup(rel, opt.Group, fi.IsDir())
	srcRel := filepath.Join("dotfiles", group, inner)
	srcAbs := filepath.Join(root, srcRel)
	if _, err := os.Lstat(srcAbs); err == nil {
		return AddResult{}, fmt.Errorf("%s already exists in the repo", srcRel)
	}

	if err := os.MkdirAll(filepath.Dir(srcAbs), 0o755); err != nil {
		return AddResult{}, err
	}
	if err := copyTree(abs, srcAbs); err != nil {
		os.RemoveAll(srcAbs)
		return AddResult{}, fmt.Errorf("copy into repo: %w", err)
	}

	mode := "copy"
	if opt.Link {
		mode = "link"
	}
	entry := config.File{Src: filepath.ToSlash(srcRel), Dest: dest}
	if mode == "copy" {
		entry.Mode = "copy"
	}
	if err := config.AppendFile(profilePath, entry); err != nil {
		os.RemoveAll(srcAbs) // nothing registered, so leave no orphan behind
		return AddResult{}, err
	}

	if opt.Link {
		// The copy is verified on disk before the original is touched, and the
		// original is moved aside first so a failure here loses nothing.
		bak := fmt.Sprintf("%s.envsetup-capture.bak", abs)
		if err := os.Rename(abs, bak); err != nil {
			return AddResult{}, err
		}
		if err := os.Symlink(srcAbs, abs); err != nil {
			os.Rename(bak, abs)
			return AddResult{}, err
		}
		os.RemoveAll(bak)
	}
	return AddResult{Src: entry.Src, Dest: dest, Mode: mode}, nil
}

// splitGroup picks the dotfiles/<group>/ folder and the path inside it.
func splitGroup(rel, group string, isDir bool) (string, string) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if group != "" {
		if isDir {
			return group, ""
		}
		return group, parts[len(parts)-1]
	}
	// ~/.config/<tool>/... lives under dotfiles/<tool>/; the directory
	// ~/.config/<tool> itself becomes dotfiles/<tool>.
	if parts[0] == ".config" && (len(parts) > 2 || (len(parts) == 2 && isDir)) {
		return parts[1], filepath.Join(parts[2:]...)
	}
	name := parts[len(parts)-1]
	g := strings.TrimPrefix(name, ".")
	if ext := filepath.Ext(g); ext != "" {
		g = strings.TrimSuffix(g, ext)
	}
	if g == "" {
		g = "misc"
	}
	return g, name
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&os.ModeSymlink != 0:
			l, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(l, target)
		default:
			return copyFile(p, target, info.Mode().Perm())
		}
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
