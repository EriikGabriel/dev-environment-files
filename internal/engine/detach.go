package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

// Detached is one destination that was a symlink into the data folder and is
// now a real copy of what it pointed to.
type Detached struct {
	Dest string `json:"dest"`
	Src  string `json:"src"`
	Dir  bool   `json:"dir"`
}

// Detach replaces every deployed symlink that points into the data folder
// with a copy of its target, so the data folder can be deleted (or the app
// uninstalled) without breaking the home. It looks at the files of every
// profile, not only the current one. Destinations that are not symlinks, or
// that point elsewhere, are left alone. A later apply puts the links back.
func Detach(cfg *config.Config, env *Env, out io.Writer) ([]Detached, error) {
	root, err := filepath.Abs(env.Root)
	if err != nil {
		return nil, err
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	dests := map[string]bool{}
	add := func(files []config.File) {
		for _, f := range files {
			dests[expandHome(f.Dest, env.Home)] = true
		}
	}
	for _, p := range cfg.Profiles {
		add(p.Files)
		for _, s := range p.Steps {
			add(s.Files)
			for _, o := range s.Options {
				add(o.Files)
			}
		}
	}
	list := make([]string, 0, len(dests))
	for d := range dests {
		list = append(list, d)
	}
	sort.Strings(list)

	done := []Detached{}
	var failed []string
	for _, dst := range list {
		fi, err := os.Lstat(dst)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := filepath.EvalSymlinks(dst)
		if err != nil || !inside(root, target) {
			continue // broken, or not ours
		}
		d, err := detachOne(dst, target)
		if err != nil {
			fmt.Fprintf(out, "FAILED %s: %v\n", dst, err)
			failed = append(failed, dst)
			continue
		}
		fmt.Fprintf(out, "%s is now a copy of %s\n", dst, target)
		done = append(done, d)
	}
	if len(failed) > 0 {
		return done, fmt.Errorf("%d files are still links into %s: %s", len(failed), root, strings.Join(failed, ", "))
	}
	return done, nil
}

func inside(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// detachOne copies target next to the link first and only then swaps it in,
// so a failure never leaves the destination missing.
func detachOne(dst, target string) (Detached, error) {
	ti, err := os.Stat(target)
	if err != nil {
		return Detached{}, err
	}
	tmp := fmt.Sprintf("%s.envsetup-detach-%d", dst, os.Getpid())
	_ = os.RemoveAll(tmp)
	if ti.IsDir() {
		err = copyTree(target, tmp)
	} else {
		err = copyFile(target, tmp, ti.Mode().Perm())
	}
	if err != nil {
		_ = os.RemoveAll(tmp)
		return Detached{}, err
	}
	if err := os.Remove(dst); err != nil {
		_ = os.RemoveAll(tmp)
		return Detached{}, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		// put the link back rather than leave nothing
		_ = os.Symlink(target, dst)
		_ = os.RemoveAll(tmp)
		return Detached{}, err
	}
	return Detached{Dest: dst, Src: target, Dir: ti.IsDir()}, nil
}
