package winsetup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadUnattendShellAndInstalls(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "windows", "autounattend.xml"))
	if err != nil {
		t.Fatal(err)
	}
	ov, err := ReadUnattend(data)
	if err != nil {
		t.Fatal(err)
	}
	if !ov.Shell.StartPinsEmpty || ov.Shell.DesktopIcons != "Custom" || !ov.Shell.HideSearch || !ov.Shell.HideTaskView || !ov.Shell.NoWidgets || !ov.Shell.NoEdgeDesktopIcon {
		t.Errorf("shell settings of the answer file: %+v", ov.Shell)
	}
	got := map[string]string{}
	for _, i := range ov.Installs {
		got[i.ID] = i.Kind
	}
	if got["ninite"] != "ninite" || got["hydra"] != "github" || got["vc-redist"] != "runtime" || got["netfx3"] != "runtime" {
		t.Errorf("installs of the answer file: %v", got)
	}
}

func TestParseNinite(t *testing.T) {
	got, err := ParseNinite([]string{"7zip steam", "https://ninite.com/discord-7zip-.net8/ninite.exe", "paint.net,operaChromium"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"7zip", "steam", "discord", ".net8", "paint.net", "operaChromium"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if NinitePage(want[:2]) != "https://ninite.com/7zip-steam/" || NiniteInstaller(want[:2]) != "https://ninite.com/7zip-steam/ninite.exe" {
		t.Error("wrong ninite links")
	}
	if NinitePage(nil) != NiniteHome {
		t.Error("an empty selection is the home page")
	}
	for _, bad := range []string{"https://example.com/7zip/", "steam;calc", "a/b"} {
		if _, err := ParseNinite([]string{bad}); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
	if u := UnknownNinite([]string{"steam", "nope"}); !reflect.DeepEqual(u, []string{"nope"}) {
		t.Errorf("unknown = %v", u)
	}
}

// repoFile reads a file of the repo this package lives in.
func repoFile(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestReadUnattendOfTheRepo(t *testing.T) {
	ov, err := ReadUnattend(repoFile(t, "windows/autounattend.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ov.URL, GeneratorURL+"?LanguageMode=Unattended&") {
		t.Errorf("generator link not found: %.80s", ov.URL)
	}
	if !ov.NiniteHook {
		t.Error("the first-logon script that runs Ninite was not noticed")
	}
	if ov.HasPassword {
		t.Error("the account has no password")
	}
	if len(ov.Scripts) != 5 {
		t.Errorf("want 5 scripts (1 system, 4 first logon), got %d", len(ov.Scripts))
	}
	flat := map[string][]string{}
	for _, s := range ov.Sections {
		for _, it := range s.Items {
			flat[s.Title] = append(flat[s.Title], it.Label+"="+it.Value)
		}
	}
	has := func(section, item string) {
		t.Helper()
		for _, it := range flat[section] {
			if it == item {
				return
			}
		}
		t.Errorf("%q misses %q: %v", section, item, flat[section])
	}
	has("Idioma e região", "Idioma do Windows=pt-BR")
	has("Idioma e região", "Fuso horário=E. South America Standard Time")
	has("Instalação", "Edição=pro")
	has("Ajustes ativados", "Ignora os requisitos do Windows 11 (TPM, Secure Boot, RAM)=")
	has("Ajustes ativados", "Widgets desativados=")
	has("Apps e recursos removidos", "3D Viewer=")
	has("Apps e recursos removidos", "Office 365=")
	has("Outras opções", "Start Pins=Empty")
	if n := len(flat["Apps e recursos removidos"]); n != 42 {
		t.Errorf("want 42 removed apps, got %d", n)
	}
}

func TestReadUnattendRefusesOtherFiles(t *testing.T) {
	for _, bad := range []string{"", "not xml", "<html></html>", "<unattend><open></unattend>"} {
		if _, err := ReadUnattend([]byte(bad)); err == nil {
			t.Errorf("%q should not pass as an answer file", bad)
		}
	}
	ov, err := ReadUnattend([]byte(`<?xml version="1.0"?><unattend xmlns="urn:schemas-microsoft-com:unattend"></unattend>`))
	if err != nil || ov.URL != "" || len(ov.Sections) != 0 {
		t.Errorf("a hand-written answer file is valid, with nothing to show: %+v %v", ov, err)
	}
}

func TestWriteDrive(t *testing.T) {
	drive := t.TempDir()
	unattend := []byte("<unattend/>")
	ventoy := repoFile(t, "windows/ventoy.json")
	fetch := func(_ context.Context, url, dest string) error {
		if url != "https://ninite.com/7zip-steam/ninite.exe" {
			t.Errorf("wrong installer link %s", url)
		}
		return os.WriteFile(dest, []byte("MZ"), 0o644)
	}
	ctx := context.Background()

	if _, err := WriteDrive(ctx, drive, unattend, ventoy, nil, fetch); err == nil {
		t.Fatal("a drive without the ventoy folder must be refused")
	}
	must(t, os.MkdirAll(filepath.Join(drive, "ventoy"), 0o755))
	// the drive already has a ventoy.json with a setting of its own
	must(t, os.WriteFile(filepath.Join(drive, "ventoy", "ventoy.json"), []byte(`{"theme":{"file":"/t.txt"},"auto_install":[]}`), 0o644))

	st, err := Inspect(drive, unattend, ventoy)
	must(t, err)
	if st.Unattend != "missing" || st.Config != "outdated" || st.Ninite || len(st.Images) != 1 || st.Images[0].Present {
		t.Errorf("empty drive: %+v", st)
	}

	written, err := WriteDrive(ctx, drive, unattend, ventoy, []string{"7zip", "steam"}, fetch)
	must(t, err)
	if len(written) != 3 || written[1].Backup == "" {
		t.Errorf("want the answer file, ventoy.json (with a backup) and Ninite: %+v", written)
	}
	got, err := os.ReadFile(filepath.Join(drive, "ventoy", "script", "autounattend.xml"))
	must(t, err)
	if string(got) != string(unattend) {
		t.Error("answer file not where ventoy.json points")
	}
	var merged map[string]json.RawMessage
	b, _ := os.ReadFile(filepath.Join(drive, "ventoy", "ventoy.json"))
	must(t, json.Unmarshal(b, &merged))
	if merged["theme"] == nil || !strings.Contains(string(merged["auto_install"]), "/Windows11.iso") {
		t.Errorf("ventoy.json must keep the drive's keys and take the repo's: %s", b)
	}
	if _, err := os.Stat(filepath.Join(drive, NiniteFile)); err != nil {
		t.Error("Ninite installer must be at the drive root")
	}

	st, err = Inspect(drive, unattend, ventoy)
	must(t, err)
	if st.Unattend != "current" || st.Config != "current" || !st.Ninite {
		t.Errorf("after writing: %+v", st)
	}
	again, err := WriteDrive(ctx, drive, unattend, ventoy, nil, fetch)
	must(t, err)
	for _, w := range again {
		if w.Action != "unchanged" {
			t.Errorf("second run must change nothing: %+v", w)
		}
	}
	if st, _ := Inspect(drive, []byte("<unattend>new</unattend>"), ventoy); st.Unattend != "outdated" {
		t.Errorf("a different answer file in the repo makes the drive outdated: %+v", st)
	}
}

func TestTemplateCannotLeaveTheDrive(t *testing.T) {
	drive := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(drive, "ventoy"), 0o755))
	for _, tpl := range []string{"../../evil.xml", "/..", `C:\Windows\x.xml`} {
		cfg, _ := json.Marshal(Ventoy{AutoInstall: []AutoInstall{{Image: "/w.iso", Template: tpl}}})
		if _, err := WriteDrive(context.Background(), drive, []byte("<unattend/>"), cfg, nil, nil); err == nil {
			t.Errorf("template %q must be refused", tpl)
		}
	}
}

func TestWords(t *testing.T) {
	for in, want := range map[string]string{"3DViewer": "3D Viewer", "Office365": "Office 365", "ZuneVideo": "Zune Video", "StartPins": "Start Pins"} {
		if got := words(in); got != want {
			t.Errorf("words(%q) = %q, want %q", in, got, want)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
