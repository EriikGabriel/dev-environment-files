package windhawk

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = "\xef\xbb\xbfMac Island & Dock backup\n" +
	"iislandBar=0\n" +
	"iislandAutoHide=1\n" +
	`sislandOrder=6m-09\\-gU` + "\n" +
	`smonitorPlaces=500\t0\t\\\\.\\DISPLAY1\n` + "\n" +
	`sweatherPlace=Some City\t-1.5\t-2.5\n` + "\n" +
	"snotes=buy milk\n" +
	`sprofile1=iislandBar=1\nsweatherPlace=Some City\nsislandOrder=x` + "\n" +
	"strayOrder=a|1\n" +
	"strayOrder=a|1\n"

func TestIslandRoundTripAndEscapes(t *testing.T) {
	is, err := ParseIsland([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(is.Entries) != 9 {
		t.Fatalf("entries = %d", len(is.Entries))
	}
	if e := is.Entries[2]; e.Name != "islandOrder" || e.Str != `6m-09\-gU` {
		t.Errorf(`\\ is one backslash: %+v`, e)
	}
	if e := is.Entries[3]; e.Str != "500\t0\t\\\\.\\DISPLAY1\n" {
		t.Errorf("tabs, newlines and backslashes: %q", e.Str)
	}
	if !bytes.Equal(is.Bytes(), []byte(sample)) {
		t.Errorf("the file must come back as the mod wrote it:\n%q\n%q", is.Bytes(), sample)
	}
	for _, bad := range []string{"", "something else\niislandBar=0\n", "Mac Island & Dock backup\nxfoo=1\n", "Mac Island & Dock backup\niislandBar=yes\n"} {
		if _, err := ParseIsland([]byte(bad)); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}

func TestIslandKeepsThePersonalValuesOut(t *testing.T) {
	is, _ := ParseIsland([]byte(sample))
	if n := is.Scrub(); n != 3 {
		t.Errorf("weatherPlace, notes and the place inside the profile: %d", n)
	}
	out := string(is.Bytes())
	if strings.Contains(out, "Some City") || strings.Contains(out, "milk") {
		t.Errorf("personal values left in:\n%s", out)
	}
	if !strings.Contains(out, `sprofile1=iislandBar=1\nsweatherPlace=\nsislandOrder=x`) {
		t.Errorf("the rest of the profile must stay:\n%s", out)
	}

	// an import writes one value per name and leaves the machine's own
	// personal values alone
	vals := is.Values()
	names := map[string]int{}
	for _, v := range vals {
		names[v.Name]++
	}
	if names["trayOrder"] != 1 || names["weatherPlace"] != 0 || names["notes"] != 0 || names["islandBar"] != 1 {
		t.Errorf("values = %+v", vals)
	}

	// saving from a machine takes its values, never its personal ones
	machine := map[string]IslandValue{
		"islandBar":            {Kind: "dword", Int: 1},
		"weatherPlace":         {Kind: "string", Str: "Home Town"},
		"notes":                {Kind: "string", Str: "secret"},
		"setupDone_S-1-5-21-1": {Kind: "string", Str: "3.2.0"},
	}
	if n := is.Update(machine); n != 1 {
		t.Errorf("only islandBar changed: %d", n)
	}
	out = string(is.Bytes())
	if strings.Contains(out, "Home Town") || strings.Contains(out, "secret") || strings.Contains(out, "setupDone") {
		t.Errorf("personal values or per-account state taken from the machine:\n%s", out)
	}
	if !strings.Contains(out, "iislandBar=1\n") {
		t.Errorf("islandBar must be the machine's: %s", out)
	}
}

func TestIslandDiff(t *testing.T) {
	is, _ := ParseIsland([]byte("Mac Island & Dock backup\niislandBar=0\niislandAutoHide=1\nsislandOrder=ab\n"))
	differ, missing := is.Diff(map[string]IslandValue{
		"islandBar":   {Kind: "dword", Int: 0},
		"islandOrder": {Kind: "string", Str: "ba"},
	})
	if differ != 1 || missing != 1 {
		t.Errorf("islandOrder differs and islandAutoHide is missing: %d %d", differ, missing)
	}
}

func TestIslandScripts(t *testing.T) {
	w := IslandWriteScript(`C:\t\values.json`)
	for _, want := range []string{`Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Windhawk\Engine\ModsWritable\mac-island-dock\LocalStorage`, "-PropertyType DWord", "-PropertyType String", `'C:\t\values.json'`} {
		if !strings.Contains(w, want) {
			t.Errorf("write script misses %s", want)
		}
	}
	if r := IslandReadScript(`C:\t\s.json`); !strings.Contains(r, "GetValueKind") || !strings.Contains(r, "UTF8Encoding $false") {
		t.Errorf("read script:\n%s", r)
	}
	s := ElevatedSteps(`C:\WH\windhawk-cli.exe`, `C:\t\cli.log`,
		Step{Args: ToggleDockArgs(false)}, Step{PS: "Write-Error 'x'"}, Step{Args: ToggleDockArgs(true)})
	off, ps, on := strings.Index(s, "'disable'"), strings.Index(s, "Write-Error 'x'"), strings.Index(s, "'enable'")
	if off < 0 || ps < off || on < ps {
		t.Errorf("the mod is turned off, its storage written, then turned on:\n%s", s)
	}
	if !strings.Contains(s, `Set-Content -LiteralPath 'C:\t\cli.log.1.err'`) {
		t.Errorf("a failing PowerShell step must leave its error where ReadLogs looks:\n%s", s)
	}
}

// The repo's backup is the mod's own file, without where the user lives.
func TestRepoIslandBackup(t *testing.T) {
	raw, err := os.ReadFile(IslandPath(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Skip("no island backup in the repo")
	}
	is, err := ParseIsland(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n := is.Scrub(); n != 0 {
		t.Errorf("windows/windhawk/%s.txt has %d personal values: run windhawk-island on it again", DockMod, n)
	}
	if len(is.Values()) < 100 {
		t.Errorf("only %d options", len(is.Values()))
	}
}
