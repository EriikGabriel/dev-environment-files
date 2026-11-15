package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EriikGabriel/dev-environment-files/internal/backend"
	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
)

// Env is everything steps need from the outside world.
type Env struct {
	Info   platform.Info
	Root   string // repo root: where dotfiles/ lives
	Home   string
	Runner backend.Runner

	refreshed map[string]bool
}

// refresh updates a backend's metadata at most once per run.
func (e *Env) refresh(ctx context.Context, b backend.Backend, out io.Writer) error {
	if e.refreshed == nil {
		e.refreshed = map[string]bool{}
	}
	if e.refreshed[b.Name()] {
		return nil
	}
	if err := b.Refresh(ctx, e.Runner, out); err != nil {
		return err
	}
	e.refreshed[b.Name()] = true
	return nil
}

// Skipped is an app left out of the plan, with the reason.
type Skipped struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// Plan is an ordered list of steps for one profile on one host.
type Plan struct {
	Profile string
	Steps   []Step
	Skipped []Skipped
	// Choices is the option in effect for every choice step.
	Choices map[string]string
	// groups[i] is the id of the profile step that Steps[i] came from.
	groups []string
}

// Options narrow what BuildPlan includes.
type Options struct {
	// Apps is the exact set of toggleable apps to include. nil means the
	// defaults: every step's `apps`, none of its `optional`.
	Apps []string
	// Choices maps a step id to the chosen option id ("none" skips it).
	// Steps left out use their default.
	Choices map[string]string
	// NoFiles skips dotfile and extension steps.
	NoFiles bool
}

// BuildPlan resolves a profile for the host described by env. All apps come
// first, in step order, then files, then VS Code extensions, so dotfiles are
// deployed after installers that would otherwise overwrite them.
func BuildPlan(cfg *config.Config, profileID string, env *Env, opt Options) (*Plan, error) {
	prof, ok := cfg.Profiles[profileID]
	if !ok {
		return nil, fmt.Errorf("unknown profile %q (available: %s)", profileID, strings.Join(cfg.ProfileNames(), ", "))
	}
	if env.Info.Family != "debian" && env.Info.Family != "arch" {
		return nil, fmt.Errorf("unsupported platform: %s/%s (%q); Linux Debian and Arch families only for now",
			env.Info.OS, env.Info.Arch, env.Info.ID)
	}
	for id := range opt.Choices {
		found := false
		for _, st := range prof.Steps {
			found = found || st.ID == id
		}
		if !found {
			return nil, fmt.Errorf("unknown step %q in --choose", id)
		}
	}

	var selected map[string]bool
	if opt.Apps != nil {
		selected = map[string]bool{}
		for _, a := range opt.Apps {
			selected[a] = true
		}
	}

	type fileIn struct {
		f     config.File
		group string
	}
	p := &Plan{Profile: profileID, Choices: map[string]string{}}
	var files []fileIn
	var exts [][2]string // path, group
	var dirs [][2]string // path, group
	addDirs := func(list []string, group string) {
		for _, d := range list {
			dirs = append(dirs, [2]string{d, group})
		}
	}
	included := map[string]bool{}

	addApp := func(id, group string) {
		if included[id] {
			return
		}
		res, err := Resolve(id, cfg.Apps[id], env.Info)
		if err != nil {
			p.Skipped = append(p.Skipped, Skipped{ID: id, Reason: err.Error()})
			return
		}
		included[id] = true
		p.Steps = append(p.Steps, &appStep{env: env, res: res})
		p.groups = append(p.groups, group)
	}

	for _, id := range prof.Apps {
		if selected == nil || selected[id] {
			addApp(id, "")
		}
	}
	for _, st := range prof.Steps {
		for _, id := range st.Apps {
			if selected == nil || selected[id] {
				addApp(id, st.ID)
			}
		}
		for _, id := range st.Optional {
			if selected != nil && selected[id] {
				addApp(id, st.ID)
			}
		}
		for _, f := range st.Files {
			files = append(files, fileIn{f, st.ID})
		}
		addDirs(st.Dirs, st.ID)
		if len(st.Options) == 0 {
			continue
		}
		choice, given := opt.Choices[st.ID]
		if !given {
			choice = st.Default
		}
		if choice == "" || choice == "none" {
			p.Choices[st.ID] = "none"
			continue
		}
		o, ok := st.Option(choice)
		if !ok {
			return nil, fmt.Errorf("step %q has no option %q", st.ID, choice)
		}
		p.Choices[st.ID] = choice
		for _, id := range o.Apps {
			addApp(id, st.ID)
		}
		for _, f := range o.Files {
			files = append(files, fileIn{f, st.ID})
		}
		addDirs(o.Dirs, st.ID)
		if o.VSCodeExtensions != "" {
			exts = append(exts, [2]string{o.VSCodeExtensions, st.ID})
		}
	}
	for _, f := range prof.Files {
		files = append(files, fileIn{f, ""})
	}

	if opt.NoFiles {
		return p, nil
	}
	// Directories come before files, which may be deployed inside them.
	for _, d := range dirs {
		p.Steps = append(p.Steps, &dirStep{path: expandHome(d[0], env.Home)})
		p.groups = append(p.groups, d[1])
	}
	for _, in := range files {
		f := in.f
		if !appliesTo(f.Only, env.Info) || (f.App != "" && !included[f.App]) {
			continue
		}
		mode := f.Mode
		if mode == "" {
			mode = "link"
		}
		p.Steps = append(p.Steps, &fileStep{
			env:  env,
			src:  filepath.Join(env.Root, f.Src),
			dst:  expandHome(f.Dest, env.Home),
			mode: mode,
		})
		p.groups = append(p.groups, in.group)
	}
	for _, e := range exts {
		p.Steps = append(p.Steps, &vscodeStep{env: env, path: filepath.Join(env.Root, e[0])})
		p.groups = append(p.groups, e[1])
	}
	// The dock comes last: it pins apps and config installed above.
	if d := prof.Desktop; d.Kind != "" && len(d.Dock.Pinned) > 0 && (d.App == "" || included[d.App]) {
		p.Steps = append(p.Steps, &dockStep{env: env, kind: d.Kind, pinned: d.Dock.Pinned})
		p.groups = append(p.groups, "desktop")
	}
	return p, nil
}

func appliesTo(only []string, info platform.Info) bool {
	if len(only) == 0 {
		return true
	}
	for _, o := range only {
		for _, c := range info.Candidates() {
			if o == c {
				return true
			}
		}
	}
	return false
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// Line is one row of a dry-run report.
type Line struct {
	Kind        string `json:"kind"` // app, file, dir, dock or vscode
	Step        string `json:"step"` // profile step id; "" for extras
	ID          string `json:"id"`   // app id, or destination for files
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Pending     bool   `json:"pending"`
	Detail      string `json:"detail"`
	Src         string `json:"src,omitempty"` // files only
	Mode        string `json:"mode,omitempty"`
}

// Describe reports the current state of every step without changing anything.
func (p *Plan) Describe(ctx context.Context) []Line {
	lines := make([]Line, len(p.Steps))
	for i, s := range p.Steps {
		pending, detail := s.Pending(ctx)
		l := Line{Title: s.Title(), ID: s.Title(), Pending: pending, Detail: detail}
		if i < len(p.groups) {
			l.Step = p.groups[i]
		}
		switch st := s.(type) {
		case *appStep:
			l.Kind, l.ID, l.Description = "app", st.res.ID, st.res.Description
		case *fileStep:
			l.Kind, l.Src, l.Mode = "file", st.src, st.mode
		case *vscodeStep:
			l.Kind = "vscode"
		case *dirStep:
			l.Kind = "dir"
		case *dockStep:
			l.Kind = "dock"
		}
		lines[i] = l
	}
	return lines
}

// Status of a step during Execute.
type Status int

const (
	Running Status = iota
	Done
	UpToDate
	Failed
)

// Event is emitted when a step starts and when it finishes.
type Event struct {
	Index, Total int
	Group        string        // profile step id the item belongs to
	Elapsed      time.Duration // set when the item finishes
	Title        string
	Status       Status
	Detail       string
	Err          error
}

// Result summarizes an Execute run.
type Result struct {
	Done, UpToDate, Failed int
	Failures               []Event
}

// Execute runs every step in order. A failing step is recorded and the run
// continues, so one broken package does not block the rest of the setup.
func (p *Plan) Execute(ctx context.Context, out io.Writer, emit func(Event)) Result {
	var res Result
	total := len(p.Steps)
	for i, s := range p.Steps {
		if ctx.Err() != nil {
			break
		}
		ev := Event{Index: i + 1, Total: total, Title: s.Title()}
		if i < len(p.groups) {
			ev.Group = p.groups[i]
		}
		pending, detail := s.Pending(ctx)
		if !pending {
			ev.Status, ev.Detail = UpToDate, detail
			res.UpToDate++
			emit(ev)
			continue
		}
		ev.Status, ev.Detail = Running, detail
		emit(ev)
		start := time.Now()
		err := s.Run(ctx, out)
		ev.Elapsed = time.Since(start)
		if err != nil {
			ev.Status, ev.Err = Failed, err
			res.Failed++
			res.Failures = append(res.Failures, ev)
		} else {
			ev.Status = Done
			res.Done++
		}
		emit(ev)
	}
	return res
}

// DefaultHome is the home directory steps deploy into.
func DefaultHome() (string, error) { return os.UserHomeDir() }
