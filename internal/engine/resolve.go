// Package engine turns a profile plus the detected platform into an ordered
// list of idempotent steps, and executes them.
package engine

import (
	"fmt"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
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

	r := Resolved{ID: id, Description: app.Description, Method: config.MethodNative}
	if m, ok := config.Pick(app.Method, cands); ok {
		r.Method = m
	}
	r.Name, _ = config.Pick(app.Names, cands)
	r.Source, _ = config.Pick(app.Source, cands)
	r.Check, _ = config.Pick(app.Check, cands)
	r.Post, _ = config.Pick(app.Post, cands)
	r.Source = expand(r.Source, info)

	switch r.Method {
	case config.MethodNative, config.MethodAUR, config.MethodFlatpak:
		if r.Name == "" {
			return Resolved{}, Unavailable{fmt.Sprintf("no package name defined for %s", info.ID)}
		}
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
	}
	return r, nil
}

// expand substitutes {arch} (amd64|arm64) and {x64} (x64|arm64).
func expand(s string, info platform.Info) string {
	arch, x64 := info.Arch, info.Arch
	if info.Arch == "amd64" {
		x64 = "x64"
	}
	return strings.NewReplacer("{arch}", arch, "{x64}", x64).Replace(s)
}
