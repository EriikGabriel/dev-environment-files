package backend

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/platform"
)

// Backend installs packages through one package manager.
type Backend interface {
	Name() string
	// Refresh updates package metadata. The engine calls it once, and only
	// when something actually needs installing.
	Refresh(ctx context.Context, r Runner, out io.Writer) error
	Installed(ctx context.Context, r Runner, pkg string) bool
	Install(ctx context.Context, r Runner, out io.Writer, pkg string) error
}

// Native returns the distro package manager for the host family.
func Native(info platform.Info) (Backend, error) {
	switch info.Family {
	case "debian":
		return Apt{}, nil
	case "arch":
		return Pacman{}, nil
	case "windows":
		return Winget{}, nil
	}
	return nil, fmt.Errorf("no native package manager for %q (family %q)", info.ID, info.Family)
}

// ---- apt ----

type Apt struct{}

func (Apt) Name() string { return "apt" }

func (Apt) Refresh(ctx context.Context, r Runner, out io.Writer) error {
	return r.Run(ctx, out, Priv("apt-get", "update")...)
}

// Installed accepts several packages separated by spaces: all must be there.
func (Apt) Installed(ctx context.Context, r Runner, pkg string) bool {
	names := strings.Fields(pkg)
	for _, n := range names {
		s, err := r.Output(ctx, "dpkg-query", "-W", "-f=${Status}", n)
		if err != nil || !strings.Contains(s, "install ok installed") {
			return false
		}
	}
	return len(names) > 0
}

func (Apt) Install(ctx context.Context, r Runner, out io.Writer, pkg string) error {
	cmd := append([]string{"env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y"}, strings.Fields(pkg)...)
	return r.Run(ctx, out, Priv(cmd...)...)
}

// InstallLocalDeb installs a downloaded .deb, resolving its dependencies.
func (Apt) InstallLocalDeb(ctx context.Context, r Runner, out io.Writer, path string) error {
	return r.Run(ctx, out, Priv("env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y", path)...)
}

// ---- pacman ----

type Pacman struct{}

func (Pacman) Name() string { return "pacman" }

// Refresh does a full upgrade: on Arch a bare -Sy followed by an install is a
// partial upgrade, which is unsupported and can break the system.
func (Pacman) Refresh(ctx context.Context, r Runner, out io.Writer) error {
	return r.Run(ctx, out, Priv("pacman", "-Syu", "--noconfirm")...)
}

func (Pacman) Installed(ctx context.Context, r Runner, pkg string) bool {
	names := strings.Fields(pkg)
	for _, n := range names {
		if _, err := r.Output(ctx, "pacman", "-Qi", n); err != nil {
			return false
		}
	}
	return len(names) > 0
}

func (Pacman) Install(ctx context.Context, r Runner, out io.Writer, pkg string) error {
	cmd := append([]string{"pacman", "-S", "--needed", "--noconfirm"}, strings.Fields(pkg)...)
	return r.Run(ctx, out, Priv(cmd...)...)
}

// ---- AUR (yay / paru) ----

// AUR builds packages from the Arch User Repository. makepkg refuses to run
// as root, so this backend must run as a regular user.
type AUR struct{}

func (AUR) Name() string { return "aur" }

func (AUR) helper(r Runner) (string, error) {
	for _, h := range []string{"yay", "paru"} {
		if r.Has(h) {
			return h, nil
		}
	}
	return "", fmt.Errorf("no AUR helper found (install yay or paru first)")
}

func (AUR) Refresh(context.Context, Runner, io.Writer) error { return nil }

func (AUR) Installed(ctx context.Context, r Runner, pkg string) bool {
	return Pacman{}.Installed(ctx, r, pkg)
}

func (a AUR) Install(ctx context.Context, r Runner, out io.Writer, pkg string) error {
	h, err := a.helper(r)
	if err != nil {
		return err
	}
	return r.Run(ctx, out, h, "-S", "--needed", "--noconfirm", pkg)
}

// ---- flatpak ----

type Flatpak struct{}

func (Flatpak) Name() string { return "flatpak" }

func (Flatpak) Refresh(context.Context, Runner, io.Writer) error { return nil }

func (Flatpak) Installed(ctx context.Context, r Runner, id string) bool {
	_, err := r.Output(ctx, "flatpak", "info", id)
	return err == nil
}

func (Flatpak) Install(ctx context.Context, r Runner, out io.Writer, id string) error {
	if !r.Has("flatpak") {
		return fmt.Errorf("flatpak is not installed")
	}
	return r.Run(ctx, out, Priv("flatpak", "install", "--system", "-y", "--noninteractive", "flathub", id)...)
}

// ---- winget (Windows) ----

// Winget is the Windows Package Manager. Packages are addressed by their
// exact ID (Valve.Steam). Installers that need elevation raise a UAC prompt.
type Winget struct{}

func (Winget) Name() string { return "winget" }

func (Winget) Refresh(context.Context, Runner, io.Writer) error { return nil }

// Installed relies on the exit code: `winget list` fails when nothing matches.
func (Winget) Installed(ctx context.Context, r Runner, id string) bool {
	_, err := r.Output(ctx, "winget", "list", "--accept-source-agreements", "--disable-interactivity", "--exact", "--id", id)
	return err == nil
}

func (Winget) Install(ctx context.Context, r Runner, out io.Writer, id string) error {
	if !r.Has("winget") {
		return fmt.Errorf("winget is not installed (it ships with App Installer from the Microsoft Store)")
	}
	return r.Run(ctx, out, "winget", "install", "--id", id, "--exact", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity")
}

// ---- scoop (Windows) ----

type Scoop struct{}

func (Scoop) Name() string { return "scoop" }

func (Scoop) Refresh(context.Context, Runner, io.Writer) error { return nil }

func (Scoop) Installed(ctx context.Context, r Runner, pkg string) bool {
	_, err := r.Output(ctx, "scoop", "prefix", pkg)
	return err == nil
}

func (Scoop) Install(ctx context.Context, r Runner, out io.Writer, pkg string) error {
	if !r.Has("scoop") {
		return fmt.Errorf("scoop is not installed (enable the scoop item first)")
	}
	return r.Run(ctx, out, "scoop", "install", pkg)
}
