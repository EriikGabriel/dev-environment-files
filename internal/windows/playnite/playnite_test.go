package playnite

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakePlaynite makes a small Playnite folder in dir.
func fakePlaynite(t *testing.T, dir string) {
	write(t, filepath.Join(dir, Exe), "exe")
	write(t, filepath.Join(dir, "config.json"), "\ufeff"+`{"Theme":"Fusion_1111","DisabledPlugins":["Off_2222"],"DatabasePath":"{PlayniteDir}\\library"}`)
	write(t, filepath.Join(dir, "fullscreenConfig.json"), `{"Theme":"Solaris_3333"}`)
	write(t, filepath.Join(dir, "Themes", "Desktop", "Fusion_1111", "theme.yaml"), "\ufeffId: Fusion_1111\nName: Fusion\nAuthor: a\nVersion: 2.0\n")
	write(t, filepath.Join(dir, "Themes", "Desktop", "Default", "theme.yaml"), "Id: Default\nName: Default\n")
	write(t, filepath.Join(dir, "Themes", "Fullscreen", "Solaris_3333", "theme.yaml"), "Id: Solaris_3333\nName: Solaris\n")
	write(t, filepath.Join(dir, "Extensions", "Meta_9c960604-b8bc-4407-a4e4-e291c6097c7d", "extension.yaml"), "Id: Meta_9c960604-b8bc-4407-a4e4-e291c6097c7d\nName: Meta\nVersion: 1\nType: MetadataProvider\n")
	write(t, filepath.Join(dir, "Extensions", "Off", "extension.yaml"), "Id: Off_2222\nName: Off\nModule: Off.dll\n")
	// the GUID of Off's data folder is only inside its DLL, as UTF-16
	g := "aaaaaaaa-1111-2222-3333-444444444444"
	dll := []byte("MZ")
	for _, c := range []byte(g) {
		dll = append(dll, c, 0)
	}
	write(t, filepath.Join(dir, "Extensions", "Off", "Off.dll"), string(dll))
	write(t, filepath.Join(dir, "ExtensionsData", g, "config.json"), `{"Tool":{"FfmpegPath":"`+jsonPath(filepath.Join(dir, "ffmpeg.exe"))+`","NormalizePath":"D:\\Python\\Scripts\\ffmpeg-normalize.exe"}}`)
	write(t, filepath.Join(dir, "ExtensionsData", "9c960604-b8bc-4407-a4e4-e291c6097c7d", "config.json"), `{"Video":"C:\\Playnite\\yt-dlp.exe","Other":"C:\\PlayniteOld\\x"}`)
	write(t, filepath.Join(dir, "library", "games.db"), "0123456789")
	write(t, filepath.Join(dir, "cache", "big.bin"), strings.Repeat("x", 100))
	write(t, filepath.Join(dir, "playnite.log"), "log")
}

func jsonPath(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }

func TestLoadDefaultAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "playnite.yaml")
	cfg, exists, err := Load(path)
	if err != nil || exists {
		t.Fatalf("missing file: %v %v", exists, err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("default = %+v", cfg)
	}
	cfg.Store = `E:\Backups`
	cfg.Exclude = []string{"cache"}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Load(path)
	if err != nil || !exists || !reflect.DeepEqual(got, cfg) {
		t.Fatalf("round trip = %+v %v %v", got, exists, err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(raw), "# The portable Playnite") {
		t.Errorf("the header is missing")
	}
}

func TestValidate(t *testing.T) {
	for _, c := range []struct {
		store string
		ok    bool
	}{
		{"", true},
		{`E:\Backups`, true},
		{`C:\Playnite`, false},
		{`c:\playnite\`, false},
		{`C:\Playnite\snap`, false},
		{`C:\`, true}, // the copy goes to C:\Playnite... which is the folder itself
	} {
		err := Config{Path: `C:\Playnite`, Store: c.store}.Validate()
		if c.store == `C:\` {
			if err == nil {
				t.Errorf("store C:\\ would mirror onto the folder itself")
			}
			continue
		}
		if (err == nil) != c.ok {
			t.Errorf("store %q: err = %v", c.store, err)
		}
	}
	if (Config{Path: `C:\Playnite`, Exclude: []string{`..\x`}}).Validate() == nil {
		t.Errorf("an exclude that leaves the folder was accepted")
	}
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	fakePlaynite(t, dir)
	in := Read(Config{Path: dir, Exclude: []string{"cache", "*.log"}}, true)
	if !in.Found {
		t.Fatal("not found")
	}
	var active []string
	for _, th := range in.Themes {
		if th.Active {
			active = append(active, th.Mode+":"+th.Name)
		}
	}
	if !reflect.DeepEqual(active, []string{"desktop:Fusion", "fullscreen:Solaris"}) {
		t.Errorf("active themes = %v", active)
	}
	if len(in.Extensions) != 2 {
		t.Fatalf("extensions = %+v", in.Extensions)
	}
	meta, off := in.Extensions[0], in.Extensions[1]
	if meta.Name != "Meta" || !meta.Enabled || meta.Data != "9c960604-b8bc-4407-a4e4-e291c6097c7d" {
		t.Errorf("meta = %+v", meta)
	}
	if off.Enabled || off.Data != "aaaaaaaa-1111-2222-3333-444444444444" {
		t.Errorf("off = %+v (the data folder comes from the DLL)", off)
	}
	for _, f := range in.Folders {
		if f.Name == "cache" && (!f.Excluded || f.Bytes != 100) {
			t.Errorf("cache = %+v", f)
		}
	}
	if in.Bytes == 0 || in.Bytes > 1000 {
		t.Errorf("bytes = %d: the cache and the log must not count", in.Bytes)
	}
	if Read(Config{Path: t.TempDir()}, true).Found {
		t.Errorf("an empty folder was taken for Playnite")
	}
}

func TestRobocopyArgs(t *testing.T) {
	got := RobocopyArgs(`C:\Playnite`, `E:\b\Playnite`, []string{"cache", "ExtensionsData/x/Cache", "*.log"}, true)
	want := []string{`C:\Playnite`, `E:\b\Playnite`, "/MIR", "/COPY:DAT", "/DCOPY:DAT", "/R:1", "/W:1", "/MT:16", "/NP", "/NFL", "/NDL", "/NJH", "/XJ",
		"/XD", filepath.Join(`C:\Playnite`, "cache"), filepath.Join(`C:\Playnite`, `ExtensionsData\x\Cache`), "/XF", "*.log"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if got := RobocopyArgs("a", "b", nil, false); got[2] != "/E" {
		t.Errorf("a restore without mirror must not purge: %q", got)
	}
}

func TestRewritePaths(t *testing.T) {
	dir := t.TempDir()
	fakePlaynite(t, dir)
	changed, err := RewritePaths(dir, `C:\Playnite`, `D:\Games\Playnite`)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 {
		t.Fatalf("changed = %v", changed)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "ExtensionsData", "9c960604-b8bc-4407-a4e4-e291c6097c7d", "config.json"))
	want := `{"Video":"D:\\Games\\Playnite\\yt-dlp.exe","Other":"C:\\PlayniteOld\\x"}`
	if string(raw) != want {
		t.Errorf("got %s\nwant %s (only whole folder names change)", raw, want)
	}
	if c, _ := RewritePaths(dir, `C:\Playnite`, `c:\playnite\`); c != nil {
		t.Errorf("the same folder must not rewrite anything: %v", c)
	}
}

func TestExternalsAndRequirements(t *testing.T) {
	dir := t.TempDir()
	fakePlaynite(t, dir)
	cfg := Config{Path: dir, StartMenu: true}
	in := Read(cfg, false)
	ext := Externals(cfg, in.Extensions)
	got := map[string]string{}
	for _, x := range ext {
		got[x.Key] = x.Extension + " " + x.Path
	}
	want := map[string]string{
		"NormalizePath": `Off D:\Python\Scripts\ffmpeg-normalize.exe`,
	}
	// "Video" does not end in Path, and FfmpegPath is inside the folder
	// (only on Windows, where the temp folder has a drive letter).
	if !reflect.DeepEqual(got, want) {
		t.Errorf("externals = %v", got)
	}

	win := t.TempDir()
	write(t, filepath.Join(win, "System32", "wmp.dll"), "")
	reqs := Requirements(cfg, in, Host{Windir: win, AppData: t.TempDir()})
	ids := map[string]bool{}
	for _, r := range reqs {
		ids[r.ID] = r.OK
	}
	if !ids[ReqMediaPlayer] {
		t.Errorf("wmp.dll is there: %+v", reqs)
	}
	if ok, found := ids[ReqStartMenu]; !found || ok {
		t.Errorf("the start menu link is missing: %+v", reqs)
	}
	if ok, found := ids[ReqFfmpegNormalize]; !found || ok {
		t.Errorf("ffmpeg-normalize is missing: %+v", reqs)
	}

	if _, err := SetExternal(cfg, `D:\Python\Scripts\ffmpeg-normalize.exe`, `C:\Py\Scripts\ffmpeg-normalize.exe`); err != nil {
		t.Fatal(err)
	}
	for _, x := range Externals(cfg, in.Extensions) {
		if x.Key == "NormalizePath" && x.Path != `C:\Py\Scripts\ffmpeg-normalize.exe` {
			t.Errorf("SetExternal left %s", x.Path)
		}
	}
}

func TestManifest(t *testing.T) {
	store := t.TempDir()
	if m, err := ReadManifest(store); m != nil || err != nil {
		t.Fatalf("empty store: %v %v", m, err)
	}
	m := Manifest{SavedAt: time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC), Source: `C:\Playnite`, Bytes: 5, Extensions: []string{"a 1"}}
	if err := WriteManifest(store, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(store)
	if err != nil || !reflect.DeepEqual(*got, m) {
		t.Errorf("round trip = %+v %v", got, err)
	}
}

func TestMatchModulesIsUnambiguous(t *testing.T) {
	dir := t.TempDir()
	g1, g2, g3 := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"
	dll := func(name string, guids ...string) string {
		b := []byte("MZ")
		for _, g := range guids {
			for _, c := range []byte(g) {
				b = append(b, c, 0)
			}
		}
		p := filepath.Join(dir, name)
		write(t, p, string(b))
		return p
	}
	for _, g := range []string{g1, g2, g3} {
		write(t, filepath.Join(dir, "data", g, "config.json"), "{}")
	}
	data, _ := os.ReadDir(filepath.Join(dir, "data"))
	for run := 0; run < 20; run++ { // map order must not matter
		exts := make([]Extension, 4)
		matchModules(exts, map[int]string{
			0: dll("shared.dll", g1, g2), // names its own folder and another plugin's
			1: dll("b.dll", g2),          // only its own
			2: dll("c.dll", g3),          // c and d both name g3: nobody gets it
			3: dll("d.dll", g3),
		}, data)
		got := []string{exts[0].Data, exts[1].Data, exts[2].Data, exts[3].Data}
		if !reflect.DeepEqual(got, []string{g1, g2, "", ""}) {
			t.Fatalf("run %d: %v", run, got)
		}
	}
}
