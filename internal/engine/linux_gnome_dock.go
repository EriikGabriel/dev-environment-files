package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// dockStep makes the real dock match desktop.dock.pinned from the profile:
// Noctalia gets its config.toml rewritten, GNOME gets favorite-apps.
type dockStep struct {
	env    *Env
	kind   string
	pinned []string
}

func (s *dockStep) Title() string { return "dock (ordem dos apps)" }

var quoted = regexp.MustCompile(`"([^"]*)"`)

func (s *dockStep) noctaliaPath() string {
	return filepath.Join(s.env.Home, ".config", "noctalia", "config.toml")
}

func (s *dockStep) Pending(ctx context.Context) (bool, string) {
	want, current, err := s.state(ctx)
	if err != nil {
		return true, err.Error()
	}
	if strings.Join(want, "\x00") == strings.Join(current, "\x00") {
		return false, fmt.Sprintf("%d apps in order", len(want))
	}
	return true, fmt.Sprintf("set %d apps in order", len(want))
}

// state returns the order the profile wants and the one the system has.
func (s *dockStep) state(ctx context.Context) (want, current []string, err error) {
	switch s.kind {
	case "niri-noctalia":
		b, err := os.ReadFile(s.noctaliaPath())
		if err != nil {
			return s.pinned, nil, fmt.Errorf("noctalia config not deployed yet (%v)", err)
		}
		_, _, _, cur := findPinned(string(b))
		return s.pinned, cur, nil
	case "gnome":
		out, err := s.env.Runner.Output(ctx, "gsettings", "get", "org.gnome.shell", "favorite-apps")
		if err != nil {
			return nil, nil, fmt.Errorf("gsettings unavailable: not running inside a GNOME session")
		}
		for _, m := range regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(out, -1) {
			current = append(current, strings.TrimSuffix(m[1], ".desktop"))
		}
		// like the old bash scripts, only pin apps that are installed
		for _, id := range s.pinned {
			if e, ok := findDesktop(s.env.Home, id); ok {
				want = append(want, e.ID)
			}
		}
		return want, current, nil
	}
	return nil, nil, fmt.Errorf("unknown desktop %q", s.kind)
}

// findPinned locates the `pinned = [...]` assignment inside the [dock] table
// of a TOML file. It returns the line range [start,end) it occupies (or the
// insertion point when absent), whether it exists, and the current values.
func findPinned(text string) (start, end int, found bool, values []string) {
	lines := strings.Split(text, "\n")
	inDock, dockLine := false, -1
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "[") {
			inDock = t == "[dock]"
			if inDock {
				dockLine = i
			}
			continue
		}
		if inDock && regexp.MustCompile(`^pinned\s*=`).MatchString(t) {
			j := i
			for !strings.Contains(lines[j], "]") && j+1 < len(lines) {
				j++
			}
			for _, m := range quoted.FindAllStringSubmatch(strings.Join(lines[i:j+1], "\n"), -1) {
				values = append(values, m[1])
			}
			return i, j + 1, true, values
		}
	}
	if dockLine >= 0 {
		return dockLine + 1, dockLine + 1, false, nil
	}
	return len(lines), len(lines), false, nil
}

func (s *dockStep) Run(ctx context.Context, out io.Writer) error {
	want, _, err := s.state(ctx)
	if err != nil && s.kind != "niri-noctalia" {
		return err
	}
	switch s.kind {
	case "niri-noctalia":
		// Writes through the symlink, so the repo's copy of the file changes.
		target, err := filepath.EvalSymlinks(s.noctaliaPath())
		if err != nil {
			return fmt.Errorf("noctalia config not found: %w", err)
		}
		b, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		text := string(b)
		items := make([]string, len(s.pinned))
		for i, id := range s.pinned {
			items[i] = `"` + strings.ReplaceAll(id, `"`, `\"`) + `"`
		}
		newLine := "pinned = [" + strings.Join(items, ", ") + "]"
		start, end, found, _ := findPinned(text)
		lines := strings.Split(text, "\n")
		if found {
			lines = append(lines[:start], append([]string{newLine}, lines[end:]...)...)
		} else if start == len(lines) {
			lines = append(lines, "[dock]", newLine)
		} else {
			lines = append(lines[:start], append([]string{newLine}, lines[start:]...)...)
		}
		fmt.Fprintf(out, "writing the dock order to %s\n", target)
		tmp := target + ".tmp"
		if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, target)
	case "gnome":
		list := make([]string, len(want))
		for i, id := range want {
			list[i] = "'" + id + ".desktop'"
		}
		value := "[" + strings.Join(list, ", ") + "]"
		fmt.Fprintf(out, "gsettings set org.gnome.shell favorite-apps %s\n", value)
		return s.env.Runner.Run(ctx, out, "gsettings", "set", "org.gnome.shell", "favorite-apps", value)
	}
	return fmt.Errorf("unknown desktop %q", s.kind)
}
