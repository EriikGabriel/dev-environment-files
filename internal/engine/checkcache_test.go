package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckCacheReusesFreshAnswersOnly(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	c := OpenCheckCache(dir)
	c.now = func() time.Time { return now }
	calls := 0
	check := func() bool { calls++; return true }
	if !c.answer("k", check) || !c.answer("k", check) || calls != 1 {
		t.Fatalf("one check for two questions, got %d", calls)
	}
	must(t, c.Save())

	// another run of envsetup reads it back
	c2 := OpenCheckCache(dir)
	c2.now = func() time.Time { return now.Add(time.Minute) }
	if !c2.answer("k", func() bool { calls++; return false }) || calls != 1 {
		t.Errorf("a fresh answer must come from the file (calls %d)", calls)
	}
	// too old: checked again
	c2.now = func() time.Time { return now.Add(CheckCacheTTL + time.Second) }
	if c2.answer("k", func() bool { calls++; return false }) || calls != 2 {
		t.Errorf("an old answer must be checked again (calls %d)", calls)
	}

	ClearCheckCache(dir)
	if _, err := os.Stat(filepath.Join(dir, checkCacheFile)); !os.IsNotExist(err) {
		t.Error("apply must drop the cache")
	}
	var none *CheckCache
	if !none.answer("k", func() bool { return true }) || none.Save() != nil {
		t.Error("no cache: every check runs")
	}
}

// Apply (no cache) always checks for real; the read-only commands share it.
func TestPlanUsesTheCacheOnlyWhenGiven(t *testing.T) {
	cfg := realConfig(t)
	fr := &fakeRunner{has: map[string]bool{"winget": true}, installed: map[string]bool{"Valve.Steam": true}}
	env := &Env{Info: win, Root: t.TempDir(), Home: t.TempDir(), Runner: fr, SteamDir: t.TempDir(), CheckCache: OpenCheckCache(t.TempDir())}
	p, err := BuildPlan(cfg, "gaming-windows", env, Options{Apps: []string{"steam"}, NoFiles: true})
	must(t, err)
	if l := p.Describe(t.Context()); len(l) != 1 || l[0].Pending {
		t.Fatalf("steam is installed: %+v", l)
	}
	delete(fr.installed, "Valve.Steam")
	if l := p.Describe(t.Context()); l[0].Pending {
		t.Error("a fresh answer is reused by the read-only commands")
	}
	env.CheckCache = nil
	if l := p.Describe(t.Context()); !l[0].Pending {
		t.Error("without the cache the machine is asked again")
	}
}
