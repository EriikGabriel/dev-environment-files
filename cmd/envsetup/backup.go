package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/config"

	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// The backup commands put together what the app's Backup tab shows before
// and after formatting: the state of Playnite and RetroArch (saved? the
// store reachable?) and of the git clone that carries their settings.
// The JSON shapes are part of the contract with app/src/types/backup.ts.

// gitPaths are what a backup commits: the settings of the programs and the
// config files that say where everything is.
var gitPaths = []string{"config", "windows"}

type gitJSON struct {
	// Repo is false when the data folder is not a git clone (the copy the
	// installer seeds): nothing can be pushed or pulled from it.
	Repo     bool   `json:"repo"`
	Branch   string `json:"branch"`
	Upstream string `json:"upstream"`
	// Dirty are the changed files under config/ and windows/ (git status).
	Dirty []string `json:"dirty"`
	// Ahead and Behind count the commits against the upstream, as of the
	// last fetch.
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
	Error  string `json:"error,omitempty"`
}

// storeGit is <store>\envsetup-git.json: where the last backup was pushed,
// so a formatted machine knows what to clone before it has the repo.
type storeGit struct {
	URL      string    `json:"url"`
	Branch   string    `json:"branch"`
	PushedAt time.Time `json:"pushed_at"`
}

const storeGitFile = "envsetup-git.json"

type backupJSON struct {
	Root    string  `json:"root"`
	Windows bool    `json:"windows"`
	Git     gitJSON `json:"git"`
	// StoreGit is what the store says about the last push; nil when unknown.
	StoreGit  *storeGit     `json:"store_git"`
	Playnite  playniteJSON  `json:"playnite"`
	RetroArch retroarchJSON `json:"retroarch"`
	// Owner is false when the configuration is someone else's: there is no
	// GitHub backup for them, only their own store.
	Owner bool `json:"owner"`
}

// gitExe is git, also right after winget installed it (PATH is not updated
// in this process yet).
func gitExe() string {
	if p, err := exec.LookPath("git"); err == nil {
		return p
	}
	for _, p := range []string{os.Getenv("ProgramFiles") + `\Git\cmd\git.exe`, os.Getenv("LOCALAPPDATA") + `\Programs\Git\cmd\git.exe`} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// git runs git in root and returns its trimmed output.
func git(root string, args ...string) (string, error) {
	c := exec.Command(gitExe(), append([]string{"-C", root}, args...)...)
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	if err := c.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimSpace(out.String()), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

func gitStatus(root string, fetch bool) gitJSON {
	g := gitJSON{Dirty: []string{}}
	if gitExe() == "" {
		g.Error = "o git não está instalado"
		return g
	}
	if v, err := git(root, "rev-parse", "--is-inside-work-tree"); err != nil || v != "true" {
		return g
	}
	g.Repo = true
	g.Branch, _ = git(root, "rev-parse", "--abbrev-ref", "HEAD")
	g.Upstream, _ = git(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if fetch && g.Upstream != "" {
		if _, err := git(root, "fetch", "--quiet"); err != nil {
			g.Error = err.Error()
		}
	}
	if out, err := git(root, append([]string{"status", "--porcelain", "--untracked-files=all", "--"}, gitPaths...)...); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if len(line) > 3 {
				g.Dirty = append(g.Dirty, strings.Trim(line[3:], `"`))
			}
		}
	}
	if g.Upstream != "" {
		if out, err := git(root, "rev-list", "--left-right", "--count", "@{u}...HEAD"); err == nil {
			if f := strings.Fields(out); len(f) == 2 {
				g.Behind, _ = strconv.Atoi(f[0])
				g.Ahead, _ = strconv.Atoi(f[1])
			}
		}
	}
	return g
}

func backupCommands(f *flags) []*cobra.Command {
	jsonFlag := withJSON(f)

	var fetch bool
	status := jsonFlag(&cobra.Command{
		Use:   "backup",
		Short: "Show what a format needs: Playnite and RetroArch saved, the store reachable, git committed and pushed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			b := backupJSON{Root: root}
			b.Owner, _ = config.IsOwner(root)
			// git, Playnite and RetroArch are read at the same time: each
			// one waits on its own processes and folders
			var wg sync.WaitGroup
			var perr, rerr error
			wg.Add(3)
			go func() { defer wg.Done(); b.Git = gitStatus(root, fetch) }()
			go func() { defer wg.Done(); b.Playnite, perr = describePlaynite(cmd, *f, false) }()
			go func() { defer wg.Done(); b.RetroArch, rerr = describeRetroArch(cmd, *f, false) }()
			wg.Wait()
			if perr != nil {
				return perr
			}
			if rerr != nil {
				return rerr
			}
			b.Windows = b.RetroArch.Windows
			for _, store := range []string{b.RetroArch.Config.Store, b.Playnite.Config.Store} {
				var sg storeGit
				if found, err := snapshot.ReadJSON(filepath.Join(store, storeGitFile), &sg); store != "" && found && err == nil {
					b.StoreGit = &sg
					break
				}
			}
			if f.json {
				return printJSON(os.Stdout, b)
			}
			fmt.Printf("git: repo=%v branch=%s upstream=%s changed=%d ahead=%d behind=%d\n", b.Git.Repo, b.Git.Branch, b.Git.Upstream, len(b.Git.Dirty), b.Git.Ahead, b.Git.Behind)
			for _, d := range b.Git.Dirty {
				fmt.Printf("  %s\n", d)
			}
			if m := b.Playnite.Snapshot; m != nil && m.Manifest != nil {
				fmt.Printf("Playnite: saved %s\n", m.Manifest.SavedAt.Local().Format("2006-01-02 15:04"))
			}
			if m := b.RetroArch.Snapshot; m != nil && m.Manifest != nil {
				fmt.Printf("RetroArch: saved %s\n", m.Manifest.SavedAt.Local().Format("2006-01-02 15:04"))
			}
			return nil
		},
	})
	status.Flags().BoolVar(&fetch, "fetch", false, "ask the remote first, so ahead and behind are current")

	push := jsonFlag(&cobra.Command{
		Use:   "backup-push",
		Short: "Commit the changes in config/ and windows/ and push them",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			g := gitStatus(root, false)
			if !g.Repo {
				return fmt.Errorf("%s is not a git clone: choose a clone of the repo as the data folder to send the backup to GitHub", root)
			}
			committed := false
			if len(g.Dirty) > 0 {
				if _, err := git(root, append([]string{"add", "-A", "--"}, gitPaths...)...); err != nil {
					return err
				}
				host, _ := os.Hostname()
				msg := fmt.Sprintf("backup: settings of %s, %s", host, time.Now().Format("2006-01-02 15:04"))
				if _, err := git(root, "commit", "-m", msg, "--", "config", "windows"); err != nil {
					return err
				}
				committed = true
			}
			var out string
			if g.Upstream == "" {
				out, err = git(root, "push", "-u", "origin", "HEAD")
			} else {
				out, err = git(root, "push")
			}
			if err != nil {
				return err
			}
			after := gitStatus(root, false)
			// tell the store where the backup went, for a formatted machine
			url, _ := git(root, "remote", "get-url", "origin")
			if _, env, err := load(*f); err == nil {
				for _, store := range storesOf(env) {
					snapshot.WriteJSON(filepath.Join(store, storeGitFile), storeGit{URL: url, Branch: after.Branch, PushedAt: time.Now().UTC().Truncate(time.Second)})
				}
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"committed": committed, "files": len(g.Dirty), "pushed": true, "branch": after.Branch, "upstream": after.Upstream})
			}
			fmt.Printf("%d files committed, pushed to %s %s\n", len(g.Dirty), after.Upstream, out)
			return nil
		},
	})

	pull := jsonFlag(&cobra.Command{
		Use:   "backup-pull",
		Short: "Bring the latest backup from GitHub (git pull --ff-only)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := findRoot(f.root)
			if err != nil {
				return err
			}
			g := gitStatus(root, false)
			if !g.Repo {
				return fmt.Errorf("%s is not a git clone: clone the repo and choose it as the data folder", root)
			}
			if g.Upstream == "" {
				return fmt.Errorf("the branch %s has no upstream on GitHub", g.Branch)
			}
			out, err := git(root, "pull", "--ff-only")
			if err != nil {
				return err
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"pulled": true, "output": out})
			}
			fmt.Println(out)
			return nil
		},
	})
	var cloneDir, cloneURL, cloneBranch string
	clone := jsonFlag(&cobra.Command{
		Use:   "backup-clone",
		Short: "Clone the repo of the backup (installing Git with winget when missing), for a formatted machine",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cloneURL == "" {
				return fmt.Errorf("--url is required")
			}
			out := os.Stdout
			if f.json {
				out = os.Stderr
			}
			if gitExe() == "" {
				fmt.Fprintln(out, "installing Git with winget")
				c := exec.CommandContext(cmd.Context(), "winget", "install", "--id", "Git.Git", "-e", "--silent", "--accept-package-agreements", "--accept-source-agreements")
				c.Stdout, c.Stderr = out, out
				if err := c.Run(); err != nil && gitExe() == "" {
					return fmt.Errorf("install Git: %w", err)
				}
				if gitExe() == "" {
					return fmt.Errorf("Git was installed but git.exe was not found; open the app again")
				}
			}
			if _, err := os.Stat(filepath.Join(cloneDir, "config", "apps.yaml")); err == nil {
				fmt.Fprintf(out, "%s already has the repo\n", cloneDir)
			} else {
				args := []string{"clone"}
				if cloneBranch != "" {
					args = append(args, "--branch", cloneBranch)
				}
				args = append(args, cloneURL, cloneDir)
				c := exec.CommandContext(cmd.Context(), gitExe(), args...)
				c.Stdout, c.Stderr = out, out
				if err := c.Run(); err != nil {
					return fmt.Errorf("git clone %s: %w", cloneURL, err)
				}
			}
			if f.json {
				return printJSON(os.Stdout, map[string]any{"path": cloneDir})
			}
			fmt.Println(cloneDir)
			return nil
		},
	})
	clone.Flags().StringVar(&cloneDir, "dir", "", "where to clone")
	clone.Flags().StringVar(&cloneURL, "url", "", "the repository")
	clone.Flags().StringVar(&cloneBranch, "branch", "", "the branch of the backup")
	clone.MarkFlagRequired("dir")

	return []*cobra.Command{status, push, pull, clone}
}

// storesOf lists the stores of Playnite and RetroArch (once each).
func storesOf(env *engine.Env) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s != "" && !seen[snapshot.Clean(s)] {
			if _, err := os.Stat(s); err == nil {
				seen[snapshot.Clean(s)] = true
				out = append(out, s)
			}
		}
	}
	if c, _, err := env.RetroArchConfig(); err == nil {
		add(c.Store)
	}
	if c, _, err := env.PlayniteConfig(); err == nil {
		add(c.Store)
	}
	return out
}
