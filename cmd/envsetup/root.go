package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/backend"
	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
)

func load(f flags) (*config.Config, *engine.Env, error) {
	root, err := findRoot(f.root)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, nil, err
	}
	home, err := engine.DefaultHome()
	if err != nil {
		return nil, nil, err
	}
	owner, _ := config.IsOwner(root)
	return cfg, &engine.Env{Info: platform.Detect(), Root: root, Home: home, Runner: backend.ExecRunner{}, Guest: !owner}, nil
}

// findRoot locates the repo root: the --root flag, else the nearest parent of
// the working directory (or of the executable) that has config/apps.yaml.
// findRoot finds the data folder (config/, dotfiles/, windows/): --root,
// then $ENVSETUP_ROOT, then a parent of the working directory or of this
// program (a git clone), then the folder the desktop app copies its data to
// (Documents\envsetup on Windows, ~/envsetup elsewhere).
func findRoot(flag string) (string, error) {
	has := func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, "config", "apps.yaml"))
		return err == nil
	}
	if flag != "" {
		if !has(flag) {
			return "", fmt.Errorf("%s has no config/apps.yaml", flag)
		}
		return filepath.Abs(flag)
	}
	if env := os.Getenv("ENVSETUP_ROOT"); env != "" && has(env) {
		return filepath.Abs(env)
	}
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, d := range starts {
		for ; ; d = filepath.Dir(d) {
			if has(d) {
				return d, nil
			}
			if d == filepath.Dir(d) {
				break
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		// best effort: a Documents folder moved to OneDrive is not found here
		for _, d := range []string{filepath.Join(home, "Documents", "envsetup"), filepath.Join(home, "envsetup")} {
			if has(d) {
				return d, nil
			}
		}
	}
	return "", fmt.Errorf("could not find config/apps.yaml; run inside the repo, set ENVSETUP_ROOT or pass --root")
}

// matchKeys are the keys profiles are matched against for a host.
func matchKeys(i platform.Info) []string {
	if i.OS != "linux" {
		return []string{i.OS}
	}
	return i.Candidates()
}

// pickProfile returns the requested profile, or the one matching the host.
func pickProfile(cfg *config.Config, env *engine.Env, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	if id, ok := cfg.MatchProfile(matchKeys(env.Info)); ok {
		return id, nil
	}
	return "", fmt.Errorf("no profile matches this machine (%s); pass --profile (available: %s)",
		env.Info.ID, strings.Join(cfg.ProfileNames(), ", "))
}

// repoRelative turns a path inside the repo into one relative to it, with
// forward slashes; other paths stay as they are.
func repoRelative(root, p string) string {
	if p == "" || !filepath.IsAbs(p) {
		return p
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return p
	}
	return filepath.ToSlash(rel)
}
