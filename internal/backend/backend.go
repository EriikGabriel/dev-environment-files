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
	}
	return nil, fmt.Errorf("no native package manager for %q (family %q)", info.ID, info.Family)
}

// ---- apt ----

type Apt struct{}

func (Apt) Name() string { return "apt" }

func (Apt) Refresh(ctx context.Context, r Runner, out io.Writer) error {
	return r.Run(ctx, out, Priv("apt-get", "update")...)
}

func (Apt) Installed(ctx context.Context, r Runner, pkg string) bool {
	s, err := r.Output(ctx, "dpkg-query", "-W", "-f=${Status}", pkg)
	return err == nil && strings.Contains(s, "install ok installed")
}

func (Apt) Install(ctx context.Context, r Runner, out io.Writer, pkg string) error {
	return r.Run(ctx, out, Priv("env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y", pkg)...)
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
	_, err := r.Output(ctx, "pacman", "-Qi", pkg)
	return err == nil
}

func (Pacman) Install(ctx context.Context, r Runner, out io.Writer, pkg string) error {
	return r.Run(ctx, out, Priv("pacman", "-S", "--needed", "--noconfirm", pkg)...)
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
