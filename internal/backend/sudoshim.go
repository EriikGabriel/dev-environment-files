package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NoStdin stops commands from inheriting this process's stdin. The TUI and
// the GUI both need it: stdin belongs to the UI, not to the child.
var NoStdin bool

// InstallSudoShim lets a process without a terminal (the desktop app) use
// sudo. It puts a `sudo` wrapper first on PATH that feeds the password to the
// real sudo through -S, so every child, including `sh -c` scripts and AUR
// helpers that call sudo themselves, gets authenticated. The password lives in
// this process's environment, readable only by the same user, and is removed
// by the returned cleanup.
func InstallSudoShim(password string) (cleanup func(), err error) {
	real, err := exec.LookPath("sudo")
	if err != nil {
		return nil, fmt.Errorf("sudo not found: %w", err)
	}
	dir, err := os.MkdirTemp("", "envsetup-sudo-")
	if err != nil {
		return nil, err
	}
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$ENVSETUP_SUDO_PW\" | exec %s -S -p '' \"$@\"\n", shellQuote(real))
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	oldPath := os.Getenv("PATH")
	os.Setenv("ENVSETUP_SUDO_PW", password)
	os.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	NoStdin = true
	return func() {
		os.Unsetenv("ENVSETUP_SUDO_PW")
		os.Setenv("PATH", oldPath)
		os.RemoveAll(dir)
	}, nil
}

// CheckSudo verifies the password works, so the GUI can report a wrong one
// before starting a long run.
func CheckSudo() error {
	out, err := exec.Command("sudo", "-v").CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo rejected the password: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
