package windhawk

import (
	"path/filepath"
	"strings"
	"testing"
)

func backup(t *testing.T, s string) *Backup {
	t.Helper()
	b, err := Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParse(t *testing.T) {
	if _, err := Parse([]byte(`{"format":"other","mods":[]}`)); err == nil {
		t.Error("another format was accepted")
	}
	if _, err := Parse([]byte(`not json`)); err == nil {
		t.Error("garbage was accepted")
	}
	b := backup(t, "\ufeff"+`{"format":"windhawk-user-data-v1","mods":[{"modId":"a","version":"1","config":{"disabled":true}}]}`)
	if len(b.Mods) != 1 || !b.Mods[0].Disabled() {
		t.Errorf("mods = %+v", b.Mods)
	}
}

func TestTheRepoBackupParses(t *testing.T) {
	b, err := Load(BackupPath(filepath.Join("..", "..")))
	if err != nil || b == nil {
		t.Fatalf("the export of the repo: %v %v", b, err)
	}
	if len(b.Mods) == 0 {
		t.Error("the export has no mods")
	}
	if m, _ := Load(filepath.Join(t.TempDir(), "none.json")); m != nil {
		t.Error("a missing file is not an export")
	}
}

func TestCompare(t *testing.T) {
	want := backup(t, `{"format":"windhawk-user-data-v1","appSettings":{"theme":"dark"},"mods":[
		{"modId":"same","version":"1","settings":{"n":1,"s":"x"}},
		{"modId":"ver","version":"2"},
		{"modId":"set","version":"1","settings":{"n":1}},
		{"modId":"off","version":"1","config":{"disabled":true}},
		{"modId":"gone","version":"1"}]}`)
	have := backup(t, `{"format":"windhawk-user-data-v1","appSettings":{"theme":"dark"},"mods":[
		{"modId":"same","version":"1","settings":{"s":"x","n":1.0}},
		{"modId":"ver","version":"1"},
		{"modId":"set","version":"1","settings":{"n":2}},
		{"modId":"off","version":"1"},
		{"modId":"zzz","version":"9"}]}`)
	mods, app := Compare(want, have)
	if app {
		t.Error("the app settings are the same")
	}
	got := map[string]ModStatus{}
	var order []string
	for _, m := range mods {
		got[m.ID] = m
		order = append(order, m.ID)
	}
	if strings.Join(order, ",") != "same,ver,set,off,gone,zzz" {
		t.Errorf("order = %v (the repo's, then the extra ones)", order)
	}
	for id, state := range map[string]string{"same": StateSame, "ver": StateChanged, "set": StateChanged, "off": StateChanged, "gone": StateMissing, "zzz": StateExtra} {
		if got[id].State != state {
			t.Errorf("%s: state %s, want %s (%v)", id, got[id].State, state, got[id].Changes)
		}
	}
	if c := got["set"].Changes; len(c) != 1 || !strings.Contains(c[0], "n") {
		t.Errorf("set changes = %v", c)
	}
	if c := got["off"].Changes; len(c) != 1 || !strings.Contains(c[0], "ativado") {
		t.Errorf("off changes = %v", c)
	}
	if got["off"].Enabled {
		t.Error("the repo keeps off disabled")
	}
	if !Pending(mods) {
		t.Error("missing and changed mods are pending")
	}
	if Pending([]ModStatus{{State: StateSame}, {State: StateExtra}}) {
		t.Error("extra mods do not make an import pending")
	}

	// the machine not read: no states
	mods, _ = Compare(want, nil)
	if len(mods) != 5 || mods[0].State != "" {
		t.Errorf("unread = %+v", mods)
	}
}

func TestCLIError(t *testing.T) {
	out := "warning: something\n" + `{"schemaVersion":1,"success":false,"error":{"code":"x","message":"mod not found"}}`
	if got := CLIError(out); got != "mod not found" {
		t.Errorf("got %q", got)
	}
	if got := CLIError(`{"success":false,"error":"boom"}`); got != "boom" {
		t.Errorf("got %q", got)
	}
	if got := CLIError("plain text\n"); got != "plain text" {
		t.Errorf("got %q", got)
	}
}

func TestElevatedScript(t *testing.T) {
	s := ElevatedScript(`C:\Program Files\Windhawk\windhawk-cli.exe`, `C:\t\cli.log`,
		ImportArgs(`C:\it's here\backup.json`), InstallDockArgs(), SetDockArgs(DockOn))
	for _, want := range []string{
		`-FilePath 'C:\Program Files\Windhawk\windhawk-cli.exe'`,
		// a path with a space reaches the CLI as one argument
		`'import', ('"' + 'C:\it''s here\backup.json' + '"'), '--confirm-app-restart'`,
		`-RedirectStandardOutput 'C:\t\cli.log.0' -RedirectStandardError 'C:\t\cli.log.0.err'`,
		`'mod', 'install', 'mac-island-dock'`,
		`'mod', 'settings', 'set', 'mac-island-dock', 'taskbarMode=dock'`,
		`-RedirectStandardOutput 'C:\t\cli.log.2'`,
		"if ($p.ExitCode) { exit $p.ExitCode }",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("script misses %s:\n%s", want, s)
		}
	}
	if strings.Count(s, "Start-Process") != 3 {
		t.Errorf("one call per command:\n%s", s)
	}
}

func TestParseSetting(t *testing.T) {
	v, err := ParseSetting(`{"schemaVersion":1,"success":true,"data":{"id":"mac-island-dock","key":"taskbarMode","value":"dock"}}`, DockSetting)
	if err != nil || v != "dock" {
		t.Errorf("one key: %v %v", v, err)
	}
	v, err = ParseSetting(`{"success":true,"data":{"settings":{"taskbarMode":"windows"}}}`, DockSetting)
	if err != nil || v != "windows" {
		t.Errorf("all keys: %v %v", v, err)
	}
	if _, err := ParseSetting(`{"success":false,"error":{"code":"MOD_NOT_INSTALLED","message":"Mod not installed: x"}}`, DockSetting); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error: %v", err)
	}
}
