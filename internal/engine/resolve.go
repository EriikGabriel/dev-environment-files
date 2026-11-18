package engine

import (
	"fmt"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
	"github.com/EriikGabriel/dev-environment-files/internal/windows/ninite"
)

// Unavailable means an app has no definition for the current distro.
type Unavailable struct{ Reason string }

func (u Unavailable) Error() string { return u.Reason }

// Resolved is an App flattened for one specific host.
type Resolved struct {
	ID          string
	Description string
	Method      string
	Name        string // package name, or flatpak ID
	Source      string // download URL or script text
	Check       string // shell check for script apps
	Post        []string
	// Repo is the apt repository to register first; nil when none.
	Repo *config.Repo
}

// Resolve picks the per-distro values of app for the given host.
func Resolve(id string, app config.App, info platform.Info) (Resolved, error) {
	cands := info.Candidates()

	for _, k := range cands {
		for _, u := range app.Unavailable {
			if u == k {
				return Resolved{}, Unavailable{fmt.Sprintf("not available on %s", k)}
			}
		}
	}

	windows := info.Family == "windows"
	r := Resolved{ID: id, Description: app.Description, Method: config.MethodNative}
	if m, ok := pick(app.Method, cands, windows); ok {
		r.Method = m
	}
	r.Name, _ = pick(app.Names, cands, windows)
	r.Source, _ = pick(app.Source, cands, windows)
	r.Check, _ = pick(app.Check, cands, windows)
	r.Post, _ = pick(app.Post, cands, windows)
	r.Source = expand(r.Source, info)
	if repo, ok := pick(app.Repo, cands, windows); ok && r.Method == config.MethodNative && info.Family == "debian" {
		repo.Source = expand(repo.Source, info)
		if strings.Contains(repo.Source, "{codename}") {
			return Resolved{}, Unavailable{"the repository needs the release codename, which os-release does not give"}
		}
		r.Repo = &repo
	}

	switch r.Method {
	case config.MethodNative, config.MethodAUR, config.MethodFlatpak, config.MethodWinget, config.MethodScoop:
		if r.Name == "" {
			return Resolved{}, Unavailable{fmt.Sprintf("no package name defined for %s", info.ID)}
		}
	case config.MethodSteam:
		if r.Name == "" || strings.Trim(r.Name, "0123456789") != "" {
			return Resolved{}, Unavailable{"steam needs the numeric AppID in names"}
		}
	case config.MethodNinite:
		slugs, err := ninite.Parse([]string{r.Name})
		if err != nil {
			return Resolved{}, Unavailable{err.Error()}
		}
		if len(slugs) == 0 {
			return Resolved{}, Unavailable{"no Ninite apps selected"}
		}
		r.Name = strings.Join(slugs, " ")
	case config.MethodDebURL:
		if r.Name == "" || r.Source == "" {
			return Resolved{}, Unavailable{fmt.Sprintf("deb-url needs names and source for %s", info.ID)}
		}
	case config.MethodScript:
		if r.Source == "" {
			return Resolved{}, Unavailable{fmt.Sprintf("no script defined for %s", info.ID)}
		}
	}

	switch {
	case r.Method == config.MethodAUR && info.Family != "arch":
		return Resolved{}, Unavailable{"aur method only works on Arch-based distros"}
	case r.Method == config.MethodDebURL && info.Family != "debian":
		return Resolved{}, Unavailable{"deb-url method only works on Debian-based distros"}
	case r.Method == config.MethodFlatpak && windows:
		return Resolved{}, Unavailable{"flatpak does not exist on Windows"}
	case (r.Method == config.MethodWinget || r.Method == config.MethodScoop || r.Method == config.MethodSteam || r.Method == config.MethodNinite || r.Method == config.MethodPolicies || r.Method == config.MethodPlaynite || r.Method == config.MethodWindhawk) && !windows:
		return Resolved{}, Unavailable{r.Method + " method only works on Windows"}
	}
	return r, nil
}

// pick is config.Pick, except that on Windows the "*" fallback is ignored:
// the wildcard entries of the catalogue are Linux package names and sh
// scripts, so Windows values must be given under the `windows` key.
func pick[T any](m map[string]T, cands []string, windows bool) (T, bool) {
	if !windows {
		return config.Pick(m, cands)
	}
	for _, k := range cands {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// expand substitutes {arch} (amd64|arm64), {x64} (x64|arm64) and, when it is
// known, {codename} (the release codename: noble, bookworm).
func expand(s string, info platform.Info) string {
	arch, x64 := info.Arch, info.Arch
	if info.Arch == "amd64" {
		x64 = "x64"
	}
	pairs := []string{"{arch}", arch, "{x64}", x64}
	if info.Codename != "" {
		pairs = append(pairs, "{codename}", info.Codename)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}
