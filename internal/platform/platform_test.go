package platform

import (
	"strings"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	tests := []struct {
		name       string
		osRelease  string
		wantID     string
		wantFamily string
		wantCand   []string
	}{
		{
			name: "ubuntu",
			osRelease: `NAME="Ubuntu"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 24.04 LTS"`,
			wantID: "ubuntu", wantFamily: "debian", wantCand: []string{"ubuntu", "debian"},
		},
		{
			name: "arch",
			osRelease: `NAME="Arch Linux"
ID=arch
PRETTY_NAME="Arch Linux"`,
			wantID: "arch", wantFamily: "arch", wantCand: []string{"arch"},
		},
		{
			name: "cachyos derives from arch",
			osRelease: `NAME="CachyOS Linux"
ID=cachyos
ID_LIKE=arch`,
			wantID: "cachyos", wantFamily: "arch", wantCand: []string{"cachyos", "arch"},
		},
		{
			name: "unknown id falls back to ID_LIKE",
			osRelease: `ID=mydistro
ID_LIKE="ubuntu debian"`,
			wantID: "mydistro", wantFamily: "debian", wantCand: []string{"mydistro", "ubuntu", "debian"},
		},
		{
			name:       "unrecognized",
			osRelease:  `ID=nixos`,
			wantID:     "nixos",
			wantFamily: "unknown",
			wantCand:   []string{"nixos"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseOSRelease(strings.NewReader(tt.osRelease), "linux", "amd64")
			if got.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantID)
			}
			if got.Family != tt.wantFamily {
				t.Errorf("Family = %q, want %q", got.Family, tt.wantFamily)
			}
			if strings.Join(got.Candidates(), ",") != strings.Join(tt.wantCand, ",") {
				t.Errorf("Candidates = %v, want %v", got.Candidates(), tt.wantCand)
			}
		})
	}
}

func TestWindowsFamily(t *testing.T) {
	got := ParseOSRelease(strings.NewReader(""), "windows", "amd64")
	if got.Family != "windows" {
		t.Errorf("Family = %q, want windows", got.Family)
	}
}

func TestCodename(t *testing.T) {
	for _, tt := range []struct{ name, osRelease, want string }{
		{"ubuntu", "ID=ubuntu\nVERSION_CODENAME=noble\nUBUNTU_CODENAME=noble\n", "noble"},
		{"mint uses the ubuntu base", "ID=linuxmint\nID_LIKE=\"ubuntu debian\"\nVERSION_CODENAME=wilma\nUBUNTU_CODENAME=noble\n", "noble"},
		{"debian", "ID=debian\nVERSION_CODENAME=bookworm\n", "bookworm"},
		{"arch has none", "ID=arch\n", ""},
	} {
		if got := ParseOSRelease(strings.NewReader(tt.osRelease), "linux", "amd64").Codename; got != tt.want {
			t.Errorf("%s: codename = %q, want %q", tt.name, got, tt.want)
		}
	}
}
