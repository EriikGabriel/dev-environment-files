package engine

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestParallelRunsEveryJob(t *testing.T) {
	var n atomic.Int32
	jobs := make([]func(), 50)
	for i := range jobs {
		jobs[i] = func() { n.Add(1) }
	}
	parallel(checkWorkers, jobs)
	if n.Load() != 50 {
		t.Fatalf("ran %d of 50", n.Load())
	}
}

// The checks run in parallel; what the Configurar screen and the plan show
// must be the same as checking one at a time.
func TestParallelChecksGiveTheSameAnswer(t *testing.T) {
	cfg := realConfig(t)
	run := func(workers int) ([]WizardStep, []Line) {
		old := checkWorkers
		checkWorkers = workers
		defer func() { checkWorkers = old }()
		fr := &fakeRunner{has: map[string]bool{"winget": true}, installed: map[string]bool{"Valve.Steam": true, "Discord.Discord": true, "Brave.Brave": true}}
		env := &Env{Info: win, Root: t.TempDir(), Home: t.TempDir(), Runner: fr, SteamDir: t.TempDir()}
		w, err := Wizard(context.Background(), cfg, "gaming-windows", env)
		must(t, err)
		p, err := BuildPlan(cfg, "gaming-windows", env, Options{NoFiles: true})
		must(t, err)
		return w, p.Describe(context.Background())
	}
	w1, d1 := run(1)
	w8, d8 := run(8)
	if !reflect.DeepEqual(w1, w8) {
		t.Error("the wizard differs when the checks run in parallel")
	}
	if !reflect.DeepEqual(d1, d8) {
		t.Errorf("the plan differs when the checks run in parallel:\n%+v\n%+v", d1, d8)
	}
	installed := 0
	for _, s := range w8 {
		for _, a := range s.Apps {
			if a.Installed {
				installed++
			}
		}
	}
	if installed < 3 {
		t.Errorf("the installed apps must be seen: %d", installed)
	}
}
