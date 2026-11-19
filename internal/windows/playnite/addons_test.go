package playnite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]string{"10.64.0.0": "10.64", " 10.7.1.0\r\n": "10.7", "1": "", "a.b": ""} {
		if got := ParseVersion(in); got != want {
			t.Errorf("ParseVersion(%q) = %q", in, got)
		}
	}
}

func TestPickPackage(t *testing.T) {
	m := []byte("\xEF\xBB\xBFAddonId: x\nPackages:\n  - Version: 3.9\n    PackageUrl: u39\n  - Version: 3.10\n    PackageUrl: u310\n  - Version: 3.7.1\n    PackageUrl: u371\n")
	if url, v, _ := pickPackage(m, "3.7.1"); url != "u371" || v != "3.7.1" {
		t.Errorf("same version: %s %s", url, v)
	}
	if url, v, _ := pickPackage(m, "9.9"); url != "u310" || v != "3.10" {
		t.Errorf("newest: %s %s", url, v)
	}
	// Theme Options repeats the key
	dup := []byte("AddonId: x\nPackages:\n  - Version: 1.0\n    PackageUrl: a\nName: x\nPackages:\n  - Version: 1.1\n    PackageUrl: b\n")
	if url, v, err := pickPackage(dup, "0"); err != nil || url != "b" || v != "1.1" {
		t.Errorf("repeated Packages: %s %s %v", url, v, err)
	}
	if _, _, err := pickPackage([]byte("Packages: []\n"), "1"); err == nil {
		t.Error("no package must fail")
	}
}

func TestScanAddons(t *testing.T) {
	dir := t.TempDir()
	write := func(rel string, body []byte) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, body, 0o644)
	}
	write("Extensions/playnite-successstory-plugin/extension.yaml", []byte("\xEF\xBB\xBFId: playnite-successstory-plugin\r\nName: SuccessStory\r\nVersion: 3.7.1\r\n"))
	write("Extensions/SteamLibrary_Builtin/extension.yaml", []byte("Id: SteamLibrary\nName: Steam\n"))
	// UTF-16 LE with BOM, as some add-ons ship it
	u := []byte{0xFF, 0xFE}
	for _, r := range "Id: Utf16Ext\nName: Wide\nVersion: 1.0\n" {
		u = append(u, byte(r), 0)
	}
	write("Extensions/Wide/extension.yaml", u)
	write("Themes/Desktop/Harmony_d49/theme.yaml", []byte("Id: Harmony_d49\nName: Harmony\nVersion: 2.74\n"))
	write("Themes/Fullscreen/Default/theme.yaml", []byte("Id: Default\nName: Default\n"))

	got := ScanAddons(dir)
	if len(got) != 5 {
		t.Fatalf("addons: %+v", got)
	}
	by := map[string]Addon{}
	for _, a := range got {
		by[a.ID] = a
	}
	if a := by["playnite-successstory-plugin"]; a.Name != "SuccessStory" || a.Kind != "extension" || a.Folder != "Extensions/playnite-successstory-plugin" || a.Builtin {
		t.Errorf("successstory: %+v", a)
	}
	if !by["SteamLibrary"].Builtin || !by["Default"].Builtin || by["Harmony_d49"].Kind != "desktop" {
		t.Errorf("kinds: %+v", got)
	}
	if by["Utf16Ext"].Name != "Wide" {
		t.Errorf("utf-16 manifest not read: %+v", got)
	}

	file := filepath.Join(t.TempDir(), "addons.yaml")
	if err := WriteAddons(file, AddonList{Playnite: "10.64", Addons: got}); err != nil {
		t.Fatal(err)
	}
	back, err := ReadAddons(file)
	if err != nil || back.Playnite != "10.64" || len(back.Addons) != 5 {
		t.Errorf("round trip: %+v %v", back, err)
	}
}
