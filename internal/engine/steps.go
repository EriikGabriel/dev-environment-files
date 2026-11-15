package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EriikGabriel/dev-environment-files/internal/backend"
	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

// Step is one unit of work. Pending is evaluated right before Run, so a step
// sees what earlier steps in the same run have already done.
type Step interface {
	Title() string
	// Pending reports whether Run has work to do, with a short description.
	Pending(ctx context.Context) (bool, string)
	Run(ctx context.Context, out io.Writer) error
}

// ---------------------------------------------------------------- app step

type appStep struct {
	env *Env
	res Resolved
}

func (s *appStep) Title() string { return s.res.ID }

func (s *appStep) backend() (backend.Backend, error) {
	switch s.res.Method {
	case config.MethodAUR:
		return backend.AUR{}, nil
	case config.MethodFlatpak:
		return backend.Flatpak{}, nil
	case config.MethodScript:
		return nil, nil
	default: // native and deb-url both end up in the distro manager
		return backend.Native(s.env.Info)
	}
}

func (s *appStep) installed(ctx context.Context) bool {
	if s.res.Method == config.MethodScript {
		if s.res.Check == "" {
			return false
		}
		_, err := s.env.Runner.Output(ctx, "sh", "-c", s.res.Check)
		return err == nil
	}
	b, err := s.backend()
	if err != nil {
		return false
	}
	return b.Installed(ctx, s.env.Runner, s.res.Name)
}

func (s *appStep) Pending(ctx context.Context) (bool, string) {
	if !s.installed(ctx) {
		return true, s.describeInstall()
	}
	if len(s.res.Post) > 0 {
		return true, "installed, running post-install"
	}
	return false, "already installed"
}

func (s *appStep) describeInstall() string {
	switch s.res.Method {
	case config.MethodScript:
		return "run install script"
	case config.MethodDebURL:
		return fmt.Sprintf("install %s from .deb", s.res.Name)
	default:
		b, err := s.backend()
		if err != nil {
			return "install " + s.res.Name
		}
		return fmt.Sprintf("install %s via %s", s.res.Name, b.Name())
	}
}

func (s *appStep) Run(ctx context.Context, out io.Writer) error {
	if !s.installed(ctx) {
		if err := s.install(ctx, out); err != nil {
			return err
		}
	}
	for _, cmd := range s.res.Post {
		fmt.Fprintf(out, "$ %s\n", cmd)
		if err := s.env.Runner.Run(ctx, out, "sh", "-c", withSudo(cmd, s.env.Root)); err != nil {
			return fmt.Errorf("post-install %q: %w", cmd, err)
		}
	}
	return nil
}

func (s *appStep) install(ctx context.Context, out io.Writer) error {
	r := s.env.Runner
	if s.res.Method == config.MethodScript {
		return r.Run(ctx, out, "sh", "-c", withSudo(s.res.Source, s.env.Root))
	}
	b, err := s.backend()
	if err != nil {
		return err
	}
	if err := s.env.refresh(ctx, b, out); err != nil {
		return fmt.Errorf("refresh %s: %w", b.Name(), err)
	}
	if s.res.Method == config.MethodDebURL {
		path, err := download(ctx, s.res.Source)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		return backend.Apt{}.InstallLocalDeb(ctx, r, out, path)
	}
	return b.Install(ctx, r, out, s.res.Name)
}

// withSudo prepares config-provided shell: $SUDO is "sudo" (empty as root)
// and $ENVSETUP_ROOT is the repo root, for scripts that need bundled files.
func withSudo(script, root string) string {
	quoted := "'" + strings.ReplaceAll(root, "'", `'\''`) + "'"
	return fmt.Sprintf("SUDO=%q; export ENVSETUP_ROOT=%s; ", strings.Join(backend.SudoPrefix(), " "), quoted) + script
}

func download(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", url, resp.Status)
	}
	// apt only treats an argument as a local file when it is a path, and
	// refuses files it cannot read as _apt, so keep it in a world-readable dir.
	f, err := os.CreateTemp("", "envsetup-*.deb")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// ---------------------------------------------------------------- file step

type fileStep struct {
	env      *Env
	src, dst string
	mode     string // link or copy
}

func (s *fileStep) Title() string { return s.dst }

func (s *fileStep) upToDate() bool {
	fi, err := os.Lstat(s.dst)
	if err != nil {
		return false
	}
	if s.mode == "link" {
		t, err := os.Readlink(s.dst)
		return err == nil && fi.Mode()&os.ModeSymlink != 0 && t == s.src
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	a, errA := os.ReadFile(s.src)
	b, errB := os.ReadFile(s.dst)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func (s *fileStep) Pending(context.Context) (bool, string) {
	if _, err := os.Stat(s.src); err != nil {
		return true, "source missing: " + s.src
	}
	if s.upToDate() {
		return false, "up to date"
	}
	return true, fmt.Sprintf("%s from %s", s.mode, s.src)
}

func (s *fileStep) Run(_ context.Context, out io.Writer) error {
	si, err := os.Stat(s.src)
	if err != nil {
		return err
	}
	if s.mode == "copy" && si.IsDir() {
		return fmt.Errorf("copy mode supports files only, use link for %s", s.src)
	}
	if err := os.MkdirAll(filepath.Dir(s.dst), 0o755); err != nil {
		return err
	}
	if _, err := os.Lstat(s.dst); err == nil {
		bak := fmt.Sprintf("%s.envsetup-%d.bak", s.dst, time.Now().Unix())
		fmt.Fprintf(out, "backing up %s to %s\n", s.dst, bak)
		if err := os.Rename(s.dst, bak); err != nil {
			return err
		}
	}
	if s.mode == "link" {
		return os.Symlink(s.src, s.dst)
	}
	data, err := os.ReadFile(s.src)
	if err != nil {
		return err
	}
	return os.WriteFile(s.dst, data, si.Mode().Perm())
}

// ---------------------------------------------------------------- dir step

type dirStep struct{ path string }

func (s *dirStep) Title() string { return s.path }

func (s *dirStep) Pending(context.Context) (bool, string) {
	if fi, err := os.Stat(s.path); err == nil && fi.IsDir() {
		return false, "exists"
	}
	return true, "create directory"
}

func (s *dirStep) Run(context.Context, io.Writer) error { return os.MkdirAll(s.path, 0o755) }

// ---------------------------------------------------------- vscode extensions

type vscodeStep struct {
	env  *Env
	path string
}

func (s *vscodeStep) Title() string { return "vscode extensions" }

// wanted reads the marketplace extension IDs, skipping the built-in vscode.*
// entries that every VS Code install already ships with.
func (s *vscodeStep) wanted() ([]string, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var list []struct {
		Identifier struct {
			ID string `json:"id"`
		} `json:"identifier"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	var ids []string
	for _, e := range list {
		if id := e.Identifier.ID; id != "" && !strings.HasPrefix(id, "vscode.") {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *vscodeStep) missing(ctx context.Context) ([]string, error) {
	want, err := s.wanted()
	if err != nil {
		return nil, err
	}
	out, err := s.env.Runner.Output(ctx, "code", "--list-extensions")
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, l := range strings.Fields(out) {
		have[strings.ToLower(l)] = true
	}
	var miss []string
	for _, id := range want {
		if !have[strings.ToLower(id)] {
			miss = append(miss, id)
		}
	}
	return miss, nil
}

func (s *vscodeStep) Pending(ctx context.Context) (bool, string) {
	if !s.env.Runner.Has("code") {
		return true, "waiting for the code binary"
	}
	miss, err := s.missing(ctx)
	if err != nil {
		return true, err.Error()
	}
	if len(miss) == 0 {
		return false, "all extensions installed"
	}
	return true, fmt.Sprintf("install %d extensions", len(miss))
}

func (s *vscodeStep) Run(ctx context.Context, out io.Writer) error {
	if !s.env.Runner.Has("code") {
		return fmt.Errorf("the code binary is not on PATH")
	}
	miss, err := s.missing(ctx)
	if err != nil {
		return err
	}
	var failed []string
	for _, id := range miss {
		if err := s.env.Runner.Run(ctx, out, "code", "--install-extension", id); err != nil {
			failed = append(failed, id)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d extensions failed: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}
