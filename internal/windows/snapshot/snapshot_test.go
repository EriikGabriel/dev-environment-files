package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{`C:\Emu\RetroArch`, `C:\Emu\Games`, false},
		{`C:\Emu\RetroArch`, `c:/emu/retroarch/`, true},
		{`C:\Emu`, `C:\Emu\Games`, true},
		{`C:\Emu\Games2`, `C:\Emu\Games`, false},
		{`C:\Emu`, ``, false},
	}
	for _, c := range cases {
		if got := Overlap(c.a, c.b); got != c.want {
			t.Errorf("Overlap(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestExcluded(t *testing.T) {
	pats := []string{"logs", `cache\thumbs`, "*.log"}
	cases := []struct {
		rel  string
		dir  bool
		want bool
	}{
		{"logs", true, true},
		{"LOGS", true, true},
		{`cache\thumbs`, true, true},
		{"cache", true, false},
		{`a\b\x.log`, false, true},
		{"logs", false, false},
	}
	for _, c := range cases {
		if got := Excluded(pats, c.rel, c.dir); got != c.want {
			t.Errorf("Excluded(%q, dir=%v) = %v", c.rel, c.dir, got)
		}
	}
}

func TestMeasure(t *testing.T) {
	dir := t.TempDir()
	for p, n := range map[string]int{"a.exe": 10, `cores\x.dll`: 100, `logs\l.txt`: 50, `saves\s.srm`: 5, `saves\old.log`: 7} {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, make([]byte, n), 0o644)
	}
	folders, total, files := Measure(dir, []string{"logs", "*.log"})
	if total != 115 || files != 3 {
		t.Errorf("total=%d files=%d", total, files)
	}
	if folders[0].Name != "cores" {
		t.Errorf("largest first: %+v", folders)
	}
	for _, f := range folders {
		if f.Name == "logs" && !f.Excluded {
			t.Errorf("logs should be marked excluded")
		}
	}
}
