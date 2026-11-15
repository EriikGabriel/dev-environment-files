// Package tui is the interactive front-end: pick apps, review, watch progress.
package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/EriikGabriel/dev-environment-files/internal/backend"
	"github.com/EriikGabriel/dev-environment-files/internal/config"
	"github.com/EriikGabriel/dev-environment-files/internal/engine"
)

var (
	title   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	good    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	bad     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warn    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	cursorS = lipgloss.NewStyle().Bold(true)
)

type stage int

const (
	stageSelect stage = iota
	stageRun
	stageDone
)

type row struct {
	id      string
	detail  string
	checked bool
}

type stepView struct {
	title  string
	status engine.Status
	detail string
	err    error
}

type (
	sudoDoneMsg struct{ err error }
	eventMsg    engine.Event
	logMsg      string
	finishedMsg engine.Result
)

type model struct {
	cfg     *config.Config
	env     *engine.Env
	profile string

	stage   stage
	rows    []row // one per app, then a final row for dotfiles
	skipped []engine.Skipped
	cursor  int

	steps  []stepView
	log    []string
	result engine.Result
	err    error

	ch     chan tea.Msg
	cancel context.CancelFunc
	width  int
	height int
}

// Run starts the TUI for a profile.
func Run(cfg *config.Config, env *engine.Env, profile string) error {
	backend.NonInteractive = true
	m := &model{cfg: cfg, env: env, profile: profile, width: 80, height: 24}
	if err := m.loadRows(); err != nil {
		return err
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m *model) loadRows() error {
	p, err := engine.BuildPlan(m.cfg, m.profile, m.env, engine.Options{NoFiles: true})
	if err != nil {
		return err
	}
	m.skipped = p.Skipped
	for _, l := range p.Describe(context.Background()) {
		m.rows = append(m.rows, row{id: l.Title, detail: l.Detail, checked: l.Pending})
	}
	m.rows = append(m.rows, row{id: "dotfiles + vscode extensions", detail: "deploy config files", checked: true})
	return nil
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		switch m.stage {
		case stageSelect:
			return m.updateSelect(msg)
		case stageDone:
			if msg.String() == "q" || msg.String() == "enter" {
				return m, tea.Quit
			}
		}
	case sudoDoneMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("sudo authentication failed: %w", msg.err)
			m.stage = stageDone
			return m, nil
		}
		return m, m.start()
	case eventMsg:
		m.applyEvent(engine.Event(msg))
		return m, m.wait()
	case logMsg:
		m.log = append(m.log, string(msg))
		if len(m.log) > 200 {
			m.log = m.log[len(m.log)-200:]
		}
		return m, m.wait()
	case finishedMsg:
		m.result = engine.Result(msg)
		m.stage = stageDone
	}
	return m, nil
}

func (m *model) updateSelect(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case " ", "x":
		m.rows[m.cursor].checked = !m.rows[m.cursor].checked
	case "a": // toggle all
		all := true
		for _, r := range m.rows {
			all = all && r.checked
		}
		for i := range m.rows {
			m.rows[i].checked = !all
		}
	case "enter":
		m.stage = stageRun
		if len(backend.SudoPrefix()) > 0 {
			// Hand the terminal to sudo once so the password prompt works,
			// then everything else can run with the cached credentials.
			return m, tea.ExecProcess(exec.Command("sudo", "-v"), func(err error) tea.Msg {
				return sudoDoneMsg{err}
			})
		}
		return m, m.start()
	}
	return m, nil
}

// start builds the plan from the selection and runs it in the background.
func (m *model) start() tea.Cmd {
	var apps []string
	for _, r := range m.rows[:len(m.rows)-1] {
		if r.checked {
			apps = append(apps, r.id)
		}
	}
	files := m.rows[len(m.rows)-1].checked
	if apps == nil {
		apps = []string{} // non-nil empty: select nothing, not everything
	}
	plan, err := engine.BuildPlan(m.cfg, m.profile, m.env, engine.Options{Apps: apps, NoFiles: !files})
	if err != nil {
		m.err, m.stage = err, stageDone
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.ch = make(chan tea.Msg, 256)
	go func() {
		res := plan.Execute(ctx, &logWriter{ch: m.ch}, func(ev engine.Event) { m.ch <- eventMsg(ev) })
		m.ch <- finishedMsg(res)
	}()
	return m.wait()
}

func (m *model) wait() tea.Cmd {
	ch := m.ch
	return func() tea.Msg { return <-ch }
}

func (m *model) applyEvent(ev engine.Event) {
	v := stepView{title: ev.Title, status: ev.Status, detail: ev.Detail, err: ev.Err}
	if n := len(m.steps); n > 0 && ev.Status != engine.Running && m.steps[n-1].title == ev.Title &&
		m.steps[n-1].status == engine.Running {
		if v.detail == "" {
			v.detail = m.steps[n-1].detail
		}
		m.steps[n-1] = v
		return
	}
	m.steps = append(m.steps, v)
}

// logWriter turns command output into log lines for the view.
type logWriter struct {
	ch  chan tea.Msg
	buf string
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.buf += string(p)
	for {
		i := strings.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.buf[:i]
		w.buf = w.buf[i+1:]
		if j := strings.LastIndexByte(line, '\r'); j >= 0 { // progress bars
			line = line[j+1:]
		}
		if line = strings.TrimSpace(line); line != "" {
			w.ch <- logMsg(line)
		}
	}
	return len(p), nil
}

func (m *model) View() string {
	var b strings.Builder
	b.WriteString(title.Render("envsetup") + dim.Render(fmt.Sprintf("  %s · %s · profile %q",
		m.env.Info.Name, m.env.Info.Arch, m.profile)) + "\n\n")

	switch m.stage {
	case stageSelect:
		m.viewSelect(&b)
	case stageRun:
		m.viewRun(&b)
	case stageDone:
		m.viewRun(&b)
		m.viewSummary(&b)
	}
	return b.String()
}

func (m *model) viewSelect(b *strings.Builder) {
	room := max(m.height-8, 5)
	start := 0
	if m.cursor >= room {
		start = m.cursor - room + 1
	}
	for i := start; i < len(m.rows) && i < start+room; i++ {
		r := m.rows[i]
		box := "[ ]"
		if r.checked {
			box = good.Render("[x]")
		}
		line := fmt.Sprintf("%s %-24s %s", box, r.id, dim.Render(r.detail))
		if i == m.cursor {
			line = cursorS.Render("> ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(truncate(line, m.width) + "\n")
	}
	if len(m.skipped) > 0 {
		b.WriteString("\n" + warn.Render(fmt.Sprintf("%d apps unavailable here: ", len(m.skipped))))
		var ids []string
		for _, s := range m.skipped {
			ids = append(ids, s.ID)
		}
		b.WriteString(dim.Render(strings.Join(ids, ", ")) + "\n")
	}
	b.WriteString("\n" + dim.Render("space toggle · a all · enter apply · q quit") + "\n")
}

func (m *model) viewRun(b *strings.Builder) {
	room := max(m.height-14, 4)
	steps := m.steps
	if len(steps) > room {
		steps = steps[len(steps)-room:]
	}
	for _, s := range steps {
		var mark string
		switch s.status {
		case engine.Running:
			mark = warn.Render("…")
		case engine.Done:
			mark = good.Render("✓")
		case engine.UpToDate:
			mark = dim.Render("=")
		case engine.Failed:
			mark = bad.Render("✗")
		}
		line := fmt.Sprintf("%s %-24s %s", mark, s.title, dim.Render(s.detail))
		if s.err != nil {
			line = fmt.Sprintf("%s %-24s %s", mark, s.title, bad.Render(s.err.Error()))
		}
		b.WriteString(truncate(line, m.width) + "\n")
	}
	if m.stage == stageRun {
		b.WriteString("\n")
		tail := m.log
		if len(tail) > 5 {
			tail = tail[len(tail)-5:]
		}
		for _, l := range tail {
			b.WriteString(dim.Render(truncate("  "+l, m.width)) + "\n")
		}
		b.WriteString("\n" + dim.Render("ctrl+c to abort") + "\n")
	}
}

func (m *model) viewSummary(b *strings.Builder) {
	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(bad.Render("error: "+m.err.Error()) + "\n")
	} else {
		fmt.Fprintf(b, "%s  %s  %s\n",
			good.Render(fmt.Sprintf("%d done", m.result.Done)),
			dim.Render(fmt.Sprintf("%d up to date", m.result.UpToDate)),
			bad.Render(fmt.Sprintf("%d failed", m.result.Failed)))
	}
	b.WriteString(dim.Render("q to quit") + "\n")
}

func truncate(s string, w int) string {
	if w <= 0 || lipgloss.Width(s) <= w {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}
