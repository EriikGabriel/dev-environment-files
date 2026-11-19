// Package backend wraps the system package managers behind one interface.
package backend

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// Runner executes external commands. It exists so tests can fake the system.
type Runner interface {
	// Run executes cmd, streaming combined output to out.
	Run(ctx context.Context, out io.Writer, cmd ...string) error
	// Output executes cmd and returns stdout. A non-zero exit is an error.
	Output(ctx context.Context, cmd ...string) (string, error)
	// Has reports whether a binary is on PATH.
	Has(bin string) bool
}

// ExecRunner is the real Runner.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, out io.Writer, cmd ...string) error {
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Stdout, c.Stderr = out, out
	if !NonInteractive && !NoStdin {
		c.Stdin = os.Stdin // lets sudo prompt for a password when needed
	}
	return c.Run()
}

func (ExecRunner) Output(ctx context.Context, cmd ...string) (string, error) {
	b, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).Output()
	return string(b), err
}

func (ExecRunner) Has(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// NonInteractive is set by the TUI, which owns the terminal: commands must
// never wait for input, so sudo runs with -n and fails if it needs a password.
var NonInteractive bool

// SudoPrefix returns the command words that elevate privileges: none when
// already root, and none on Windows.
func SudoPrefix() []string {
	switch {
	case runtime.GOOS == "windows":
		return nil // installers ask for elevation themselves (UAC)
	case os.Geteuid() == 0:
		return nil
	case NonInteractive:
		return []string{"sudo", "-n"}
	}
	return []string{"sudo"}
}

// Priv prefixes cmd with sudo unless already running as root.
func Priv(cmd ...string) []string {
	return append(SudoPrefix(), cmd...)
}
