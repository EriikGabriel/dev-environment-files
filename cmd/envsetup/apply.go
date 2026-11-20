package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EriikGabriel/dev-environment-files/internal/backend"
	"github.com/EriikGabriel/dev-environment-files/internal/engine"
)

// applyCommands show and run the plan of a profile.
func applyCommands(f *flags) []*cobra.Command {
	plan := &cobra.Command{
		Use:   "plan",
		Short: "Show what apply would do, without changing anything",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, env, err := buildPlan(*f)
			if err != nil {
				return err
			}
			env.CheckCache = engine.OpenCheckCache(engine.StateDir())
			defer env.CheckCache.Save()
			if f.json {
				return printJSON(os.Stdout, planJSON{
					Platform: toPlatformJSON(env.Info), Root: env.Root, Profile: p.Profile, Choices: p.Choices,
					Steps: nonNil(p.Describe(cmd.Context())), Skipped: nonNilSkipped(p.Skipped),
				})
			}
			printPlan(p, cmd.Context())
			return nil
		},
	}
	addPlanFlags(plan, f)
	plan.Flags().BoolVar(&f.json, "json", false, "machine-readable output")

	apply := &cobra.Command{
		Use:   "apply",
		Short: "Install the profile's apps and deploy its config files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Geteuid() == 0 && os.Getenv("SUDO_USER") != "" {
				return fmt.Errorf("run envsetup as your normal user, not through sudo: it calls sudo itself where needed, and per-user files must land in your home")
			}
			// apply checks for real and changes what is installed: the
			// answers kept for the Configurar screen are dropped, before and after
			engine.ClearCheckCache(engine.StateDir())
			defer engine.ClearCheckCache(engine.StateDir())
			if f.json {
				return applyJSON(cmd.Context(), *f)
			}
			p, _, err := buildPlan(*f)
			if err != nil {
				return err
			}
			printPlan(p, cmd.Context())
			if !f.yes && !confirm("Apply?") {
				return fmt.Errorf("aborted")
			}
			res := p.Execute(cmd.Context(), os.Stdout, func(ev engine.Event) {
				switch ev.Status {
				case engine.Running:
					fmt.Printf("\n==> [%d/%d] %s: %s\n", ev.Index, ev.Total, ev.Title, ev.Detail)
				case engine.Done:
					fmt.Printf("    ok\n")
				case engine.UpToDate:
					fmt.Printf("[%d/%d] %s: %s\n", ev.Index, ev.Total, ev.Title, ev.Detail)
				case engine.Failed:
					fmt.Printf("    FAILED: %v\n", ev.Err)
				}
			})
			fmt.Printf("\n%d done, %d up to date, %d failed\n", res.Done, res.UpToDate, res.Failed)
			for _, fl := range res.Failures {
				fmt.Printf("  - %s: %v\n", fl.Title, fl.Err)
			}
			if res.Failed > 0 {
				return fmt.Errorf("%d steps failed", res.Failed)
			}
			return nil
		},
	}
	addPlanFlags(apply, f)
	apply.Flags().BoolVarP(&f.yes, "yes", "y", false, "do not ask for confirmation")
	apply.Flags().BoolVar(&f.json, "json", false, "stream progress as JSON lines (implies --yes)")
	apply.Flags().BoolVar(&f.sudoPW, "sudo-password-stdin", false, "read the sudo password from the first line of stdin")

	return []*cobra.Command{plan, apply}
}

func buildPlan(f flags) (*engine.Plan, *engine.Env, error) {
	cfg, env, err := load(f)
	if err != nil {
		return nil, nil, err
	}
	prof, err := pickProfile(cfg, env, f.profile)
	if err != nil {
		return nil, nil, err
	}
	apps := f.apps
	if f.noApps {
		apps = []string{} // non-nil and empty: select nothing, not everything
	}
	vars := map[string]string{}
	for _, kv := range f.set {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, nil, fmt.Errorf("--set wants key=value, got %q", kv)
		}
		vars[k] = v
	}
	p, err := engine.BuildPlan(cfg, prof, env, engine.Options{Apps: apps, NoFiles: f.noFiles, Choices: f.choose, Vars: vars})
	return p, env, err
}

func printPlan(p *engine.Plan, ctx context.Context) {
	for _, l := range p.Describe(ctx) {
		mark := "="
		if l.Pending {
			mark = "+"
		}
		fmt.Printf("%s %-28s %s\n", mark, l.Title, l.Detail)
	}
	for _, s := range p.Skipped {
		fmt.Printf("- %-28s skipped: %s\n", s.ID, s.Reason)
	}
}

func confirm(q string) bool {
	fmt.Printf("%s [y/N] ", q)
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	ans = strings.ToLower(strings.TrimSpace(ans))
	return ans == "y" || ans == "yes"
}

func nonNil(l []engine.Line) []engine.Line {
	if l == nil {
		return []engine.Line{}
	}
	return l
}

func nonNilSkipped(l []engine.Skipped) []engine.Skipped {
	if l == nil {
		return []engine.Skipped{}
	}
	return l
}

// applyJSON runs apply for the desktop app: no prompts, progress as JSON lines.
func applyJSON(ctx context.Context, f flags) error {
	if f.sudoPW {
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		cleanup, err := backend.InstallSudoShim(strings.TrimRight(line, "\r\n"))
		if err != nil {
			return err
		}
		defer cleanup()
		if err := backend.CheckSudo(); err != nil {
			return err
		}
	}
	backend.NoStdin = true
	p, _, err := buildPlan(f)
	if err != nil {
		return err
	}
	out := newNDJSON(os.Stdout)
	logFile, logPath := openRunLog()
	if logFile != nil {
		defer logFile.Close()
	}
	lines := &logLines{out: out}
	if logFile != nil {
		lines.file = logFile
	}
	note := func(format string, a ...any) {
		if logFile != nil {
			fmt.Fprintf(logFile, "%s  "+format+"\n", append([]any{time.Now().Format("15:04:05")}, a...)...)
		}
	}
	note("apply started: profile %s, %d steps", p.Profile, len(p.Steps))
	res := p.Execute(ctx, lines, func(ev engine.Event) {
		lines.index = ev.Index
		e := streamEvent{Type: "step", Index: ev.Index, Total: ev.Total, Title: ev.Title,
			Status: statusName(ev.Status), Detail: ev.Detail, Group: ev.Group, DurationMs: ev.Elapsed.Milliseconds()}
		if ev.Err != nil {
			e.Error = ev.Err.Error()
		}
		out.emit(e)
		switch ev.Status {
		case engine.Running:
			note("[%d/%d] START   %s: %s", ev.Index, ev.Total, ev.Title, ev.Detail)
		case engine.Done:
			note("[%d/%d] CHANGED %s (%s)", ev.Index, ev.Total, ev.Title, ev.Elapsed.Round(time.Millisecond))
		case engine.UpToDate:
			note("[%d/%d] SAME    %s: %s", ev.Index, ev.Total, ev.Title, ev.Detail)
		case engine.Failed:
			note("[%d/%d] FAILED  %s: %v", ev.Index, ev.Total, ev.Title, ev.Err)
		}
	})
	if ctx.Err() != nil {
		note("apply interrupted")
	}
	note("finished: %d changed, %d already up to date, %d failed", res.Done, res.UpToDate, res.Failed)
	out.emit(streamEvent{Type: "done", Done: &res.Done, UpToDate: &res.UpToDate, Failed: &res.Failed, LogFile: logPath})
	if res.Failed > 0 {
		return fmt.Errorf("%d steps failed", res.Failed)
	}
	return nil
}

// openRunLog creates the text log of an apply run under the user's state
// directory. A failure to create it only disables the file log.
func openRunLog() (*os.File, string) {
	dir := engine.StateDir()
	if dir == "" {
		return nil, ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, ""
	}
	path := filepath.Join(dir, "apply-"+time.Now().Format("20060102-150405")+".log")
	f, err := os.Create(path)
	if err != nil {
		return nil, ""
	}
	return f, path
}
