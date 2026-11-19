package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchProfile(t *testing.T) {
	pats := []string{"retroarch.cfg", "saves", "ExtensionsData/*/config.json"}
	cases := map[string]bool{
		"retroarch.cfg":                    true,
		"RetroArch.CFG":                    true,
		"saves/Snes9x/Mario.srm":           true,
		"savesx/a":                         false,
		"ExtensionsData/abc/config.json":   true,
		"ExtensionsData/abc/other.json":    false,
		"ExtensionsData/abc/d/config.json": false,
		"retroarch.exe":                    false,
	}
	for rel, want := range cases {
		if got := MatchProfile(pats, rel); got != want {
			t.Errorf("MatchProfile(%q) = %v", rel, got)
		}
	}
	if got := TopLevel(pats); len(got) != 2 || got[0] != "retroarch.cfg" || got[1] != "saves" {
		t.Errorf("TopLevel = %v", got)
	}
}

func TestIsSecretKey(t *testing.T) {
	for k, want := range map[string]bool{
		"cheevos_password":               true,
		"cheevos_token":                  true,
		"cheevos_username":               false,
		"ApiKey":                         true,
		"ProviderApiKeys":                true,
		"SgdbApiKey":                     true,
		"AccessToken":                    true,
		"RefreshToken":                   true,
		"IgdbClientSecret":               true,
		"TokenType":                      false,
		"RefreshTokenExpiryUtc":          false,
		"ViewAchievementsHotkey":         false,
		"LanguageKey":                    false,
		"YoutubeCookiesPath":             false,
		"content_show_settings_password": true,
		"kiosk_mode_password":            true,
		"netplay_show_passworded":        false,
	} {
		if got := IsSecretKey(k); got != want {
			t.Errorf("IsSecretKey(%q) = %v", k, got)
		}
	}
}

func TestScrubJSONRoundTrip(t *testing.T) {
	raw := "\xEF\xBB\xBF" + `{
  "ApiKey": "abc\"123",
  "Name": "keep",
  "Nested": { "ApiKey": "", "RefreshToken": "r1", "TokenExpiryUtc": "2026-01-01" },
  "ProviderApiKeys": { "Groq": "g1", "Other": "" },
  "List": [ { "ApiKey": "l1" } ],
  "Count": 3
}`
	clean, secrets := Scrub("ExtensionsData/x/config.json", []byte(raw))
	s := string(clean)
	for _, gone := range []string{`abc\"123`, "r1", "g1", "l1"} {
		if strings.Contains(s, gone) {
			t.Errorf("%s left in:\n%s", gone, s)
		}
	}
	for _, kept := range []string{`"Name": "keep"`, `"TokenExpiryUtc": "2026-01-01"`, `"Count": 3`, "\xEF\xBB\xBF{"} {
		if !strings.Contains(s, kept) {
			t.Errorf("%s lost in:\n%s", kept, s)
		}
	}
	if len(secrets) != 4 {
		t.Fatalf("secrets: %+v", secrets)
	}
	back, n := Unscrub("ExtensionsData/x/config.json", clean, secrets)
	if n != 4 || string(back) != raw {
		t.Errorf("n=%d, round trip:\n%s", n, back)
	}
	// a value set again by hand is not overwritten
	edited := strings.Replace(s, `"RefreshToken": ""`, `"RefreshToken": "new"`, 1)
	back, n = Unscrub("x.json", []byte(edited), secrets)
	if n != 3 || !strings.Contains(string(back), `"RefreshToken": "new"`) {
		t.Errorf("n=%d:\n%s", n, back)
	}
}

func TestScrubCfg(t *testing.T) {
	raw := "video_driver = \"vulkan\"\r\ncheevos_password = \"hunter2\"\r\ncheevos_username = \"me\"\r\nnetplay_password = \"\"\r\n"
	clean, secrets := Scrub("retroarch.cfg", []byte(raw))
	if strings.Contains(string(clean), "hunter2") || !strings.Contains(string(clean), "cheevos_password = \"\"\r\n") || !strings.Contains(string(clean), `cheevos_username = "me"`) {
		t.Errorf("scrubbed:\n%s", clean)
	}
	if len(secrets) != 1 || secrets[0].Key != "cheevos_password" {
		t.Fatalf("secrets: %+v", secrets)
	}
	if back, n := Unscrub("retroarch.cfg", clean, secrets); n != 1 || string(back) != raw {
		t.Errorf("round trip (%d):\n%s", n, back)
	}
}

func TestExportImport(t *testing.T) {
	src, git, dst := t.TempDir(), t.TempDir(), t.TempDir()
	put := func(dir, rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	pats := []string{"retroarch.cfg", "saves"}
	put(src, "retroarch.cfg", "cheevos_password = \"pw\"\n")
	put(src, "saves/a.srm", "save")
	put(src, "retroarch.exe", "MZ")
	put(git, "saves/old/gone.srm", "old")

	res, err := Export(src, git, pats)
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != 2 || len(res.Changed) != 2 || len(res.Removed) != 1 || len(res.Secrets) != 1 {
		t.Errorf("export: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(git, "saves", "old")); err == nil {
		t.Error("the empty folder was left behind")
	}
	if _, err := os.Stat(filepath.Join(git, "retroarch.exe")); err == nil {
		t.Error("a file outside the patterns was copied")
	}
	if raw, _ := os.ReadFile(filepath.Join(git, "retroarch.cfg")); strings.Contains(string(raw), "pw") {
		t.Errorf("secret in git: %s", raw)
	}
	again, _ := Export(src, git, pats)
	if len(again.Changed) != 0 {
		t.Errorf("unchanged files written again: %+v", again)
	}

	file := SecretsFile(t.TempDir(), "retroarch")
	if err := WriteJSON(file, res.Secrets); err != nil {
		t.Fatal(err)
	}
	secrets, err := ReadSecrets(file)
	if err != nil {
		t.Fatal(err)
	}
	imp, err := Import(git, dst, pats, secrets)
	if err != nil {
		t.Fatal(err)
	}
	if imp.Files != 2 || imp.Secrets != 1 {
		t.Errorf("import: %+v", imp)
	}
	if raw, _ := os.ReadFile(filepath.Join(dst, "retroarch.cfg")); string(raw) != "cheevos_password = \"pw\"\n" {
		t.Errorf("restored cfg: %q", raw)
	}
}
