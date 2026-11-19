package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/testutil"
)

func repoPolicies(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testutil.RepoRoot(t), "config", "policies.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "policies.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func find(t *testing.T, f *File, id string) Policy {
	t.Helper()
	for _, p := range f.Policies {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no policy %q", id)
	return Policy{}
}

func TestRepoPoliciesLoad(t *testing.T) {
	f, err := Load(repoPolicies(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Policies) != 42 {
		t.Errorf("the file has %d policies, want 42", len(f.Policies))
	}
	owner := map[string]string{}
	for _, p := range f.Policies {
		if p.Choice != ChoiceVideo && p.Choice != ChoiceOff {
			t.Errorf("%s: unexpected repo choice %q", p.ID, p.Choice)
		}
		if p.Group == "" || p.Path == "" || p.Title == "" || p.Video == "" || p.About == "" {
			t.Errorf("%s: group, path, title, video and about are all required", p.ID)
		}
		for _, v := range p.Set {
			id := strings.ToLower(v.Key + `\` + v.Name)
			if other, dup := owner[id]; dup {
				t.Errorf("%s and %s write the same value %s", other, p.ID, id)
			}
			owner[id] = p.ID
		}
	}
}

func TestChoiceDecidesWhatIsWrittenAndRemoved(t *testing.T) {
	path := repoPolicies(t)
	f, _ := Load(path)
	wu := find(t, f, "windows-update-auto")
	set, remove := wu.Wanted()
	if len(set) != 5 || len(remove) != 0 {
		t.Fatalf("video: %d set, %d removed", len(set), len(remove))
	}

	if err := SetChoice(path, "windows-update-auto", "download"); err != nil {
		t.Fatal(err)
	}
	if err := SetChoice(path, "widgets", ChoiceOff); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	set, _ = find(t, f, "windows-update-auto").Wanted()
	if set[1].Name != "AUOptions" || set[1].Value != 3 {
		t.Errorf("option not in effect: %+v", set)
	}
	set, remove = find(t, f, "widgets").Wanted()
	if len(set) != 0 || len(remove) != 1 || remove[0].Name != "AllowNewsAndInterests" {
		t.Errorf("off must remove the value: %v %v", set, remove)
	}

	// only the two `choice:` lines changed
	before, _ := os.ReadFile(filepath.Join(testutil.RepoRoot(t), "config", "policies.yaml"))
	after, _ := os.ReadFile(path)
	a, b := strings.Split(string(before), "\n"), strings.Split(string(after), "\n")
	if len(a) != len(b) {
		t.Fatalf("line count changed: %d to %d", len(a), len(b))
	}
	changed := 0
	for i := range a {
		if a[i] != b[i] {
			changed++
		}
	}
	if changed != 2 {
		t.Errorf("want 2 changed lines, got %d", changed)
	}

	if err := SetChoice(path, "widgets", "nope"); err == nil {
		t.Error("an unknown choice must be refused")
	}
	if err := SetChoice(path, "nope", ChoiceOff); err == nil {
		t.Error("an unknown policy must be refused")
	}
}

func TestEveryPolicyCanBeSwitched(t *testing.T) {
	f, err := Load(repoPolicies(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range f.Policies {
		if len(p.Options) == 0 {
			t.Errorf("%s has no state besides the recommended one and not configured", p.ID)
		}
	}
	set := find(t, f, "widgets").Options[0].Set
	if len(set) != 1 || set[0].Value != 1 {
		t.Errorf("the opposite of a disabled policy writes the flipped value: %+v", set)
	}
}

func TestScripts(t *testing.T) {
	p := []Policy{
		{ID: "a", Choice: ChoiceVideo, Set: []Value{{`HKLM\SOFTWARE\Policies\Microsoft\Windows\Windows Search`, "AllowCortana", 0}}},
		{ID: "b", Choice: ChoiceOff, Set: []Value{{`HKLM\SOFTWARE\Policies\Microsoft\Dsh`, "AllowNewsAndInterests", 0}}},
	}
	state := StateScript(p)
	for _, want := range []string{
		`$r['a'] = [bool](((V 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Microsoft\Windows\Windows Search' 'AllowCortana') -eq 0))`,
		`$r['b'] = [bool](($null -eq (V 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Microsoft\Dsh' 'AllowNewsAndInterests')))`,
	} {
		if !strings.Contains(state, want) {
			t.Errorf("state script misses %s in:\n%s", want, state)
		}
	}
	apply := ApplyScript(p)
	for _, want := range []string{
		`Set-ItemProperty -LiteralPath 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Microsoft\Windows\Windows Search' -Name 'AllowCortana' -Value 0 -Type DWord`,
		`Remove-ItemProperty -LiteralPath 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Microsoft\Dsh' -Name 'AllowNewsAndInterests'`,
		`PolSet 'SOFTWARE\Policies\Microsoft\Windows\Windows Search' 'AllowCortana' 0`,
		`PolDel 'SOFTWARE\Policies\Microsoft\Dsh' 'AllowNewsAndInterests'`,
		"PolSave\n",
		`if ($c -eq 0x3B)`,
		`[Text.Encoding]::Unicode.GetBytes($e.Key)`,
		`[Text.Encoding]::Unicode.GetBytes($e.Name)`,
	} {
		if !strings.Contains(apply, want) {
			t.Errorf("apply script misses %s in:\n%s", want, apply)
		}
	}
	st, err := ParseState(`{"a":true,"b":false}` + "\r\n")
	if err != nil || !st["a"] || st["b"] {
		t.Errorf("state: %v %v", st, err)
	}
	if cmd := ElevatedCommand(`C:\Users\A B\x.ps1`); !strings.Contains(cmd, `'-File','"C:\Users\A B\x.ps1"'`) || !strings.Contains(cmd, "-Verb RunAs") {
		t.Errorf("a path with spaces must stay one argument: %s", cmd)
	}
}

func TestLoadRefusesKeysOutsidePolicies(t *testing.T) {
	for _, key := range []string{`HKLM\SYSTEM\CurrentControlSet\Services\x`, `HKCU\SOFTWARE\Policies\x`, `HKLM\SOFTWARE\Policies\x'; calc; '`} {
		path := filepath.Join(t.TempDir(), "p.yaml")
		doc := "policies:\n  - id: x\n    choice: video\n    set:\n      - { key: '" + strings.ReplaceAll(key, "'", "''") + "', name: A, value: 1 }\n"
		os.WriteFile(path, []byte(doc), 0o644)
		if _, err := Load(path); err == nil {
			t.Errorf("key %q must be refused", key)
		}
	}
}
