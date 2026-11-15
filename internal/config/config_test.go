package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRepoConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		cands []string
		want  string
	}{
		{[]string{"cachyos", "arch"}, "dev-cachyos"},
		{[]string{"arch"}, "dev-cachyos"},
		{[]string{"ubuntu", "debian"}, "dev-ubuntu"},
		{[]string{"linuxmint", "ubuntu", "debian"}, "dev-ubuntu"},
		{[]string{"windows"}, "gaming-windows"},
	}
	for _, tt := range tests {
		if got, _ := cfg.MatchProfile(tt.cands); got != tt.want {
			t.Errorf("MatchProfile(%v) = %q, want %q", tt.cands, got, tt.want)
		}
	}
	if _, ok := cfg.MatchProfile([]string{"nixos"}); ok {
		t.Error("nixos should match no profile")
	}
}

func TestPickOrder(t *testing.T) {
	m := map[string]string{"ubuntu": "u", "debian": "d", "*": "any"}
	tests := []struct {
		cands []string
		want  string
	}{
		{[]string{"ubuntu", "debian"}, "u"}, // exact ID wins
		{[]string{"mint", "debian"}, "d"},   // ID_LIKE next
		{[]string{"arch"}, "any"},           // wildcard last
	}
	for _, tt := range tests {
		if got, _ := Pick(m, tt.cands); got != tt.want {
			t.Errorf("Pick(%v) = %q, want %q", tt.cands, got, tt.want)
		}
	}
	if _, ok := Pick(map[string]string{"arch": "a"}, []string{"ubuntu"}); ok {
		t.Error("no match and no wildcard should report not found")
	}
}

func TestValidateErrors(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("config/apps.yaml", "apps:\n  a:\n    names: {\"*\": a}\n")
	write("config/profiles/p.yaml", "apps: [missing]\n")
	if _, err := Load(dir); err == nil {
		t.Error("profile referencing an unknown app should fail")
	}

	write("config/profiles/p.yaml", "apps: [a]\n")
	write("config/apps.yaml", "apps:\n  a:\n    method: {\"*\": pip}\n")
	if _, err := Load(dir); err == nil {
		t.Error("unknown method should fail")
	}

	write("config/apps.yaml", "apps:\n  a:\n    nmaes: {\"*\": a}\n") // typo
	if _, err := Load(dir); err == nil {
		t.Error("unknown key should fail instead of being ignored")
	}
}
