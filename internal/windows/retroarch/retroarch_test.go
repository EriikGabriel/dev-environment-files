package retroarch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestValidate(t *testing.T) {
	ok := Config{Path: `C:\Emu\RetroArch`, Games: `C:\Emu\Games`, Store: `E:\Backup`}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
	bad := map[string]Config{
		"games inside":   {Path: `C:\Emu\RetroArch`, Games: `C:\Emu\RetroArch\roms`},
		"store inside":   {Path: `C:\Emu\RetroArch`, Store: `c:/emu/retroarch/backup`},
		"store in games": {Path: `C:\Emu\RetroArch`, Games: `C:\Emu\Games`, Store: `C:\Emu\Games\bk`},
		"games in store": {Path: `C:\Emu\RetroArch`, Games: `E:\Backup\Games`, Store: `E:\Backup`},
		"empty path":     {Path: " "},
		"exclude ..":     {Path: `C:\Emu\RetroArch`, Exclude: []string{`..\x`}},
	}
	for name, c := range bad {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config", "retroarch.yaml")
	if _, exists, err := Load(p); exists || err != nil {
		t.Fatalf("missing file: exists=%v err=%v", exists, err)
	}
	c := Config{Path: `D:\RA`, Games: "", Store: `E:\bk`, Exclude: []string{"logs"}, StartMenu: false}
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Load(p)
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	if got.Path != c.Path || got.Games != "" || got.Store != c.Store || got.StartMenu || len(got.Exclude) != 1 {
		t.Errorf("round trip: %+v", got)
	}
	raw, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(raw), "# The portable RetroArch") {
		t.Errorf("the header is missing:\n%s", raw)
	}
}

// fixture makes a RetroArch folder and a games folder whose playlists point
// to an old place.
func fixture(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	c := Config{Path: filepath.Join(dir, "RetroArch"), Games: filepath.Join(dir, "Games")}
	write(t, filepath.Join(c.Path, Exe), "MZ")
	write(t, filepath.Join(c.Path, "retroarch.cfg"), "video_driver = \"vulkan\"\n# comment\nvideo_fullscreen = \"true\"\nlibretro_directory = \":\\cores\"\n")
	write(t, filepath.Join(c.Path, "cores", "snes9x_libretro.dll"), "")
	write(t, filepath.Join(c.Path, "info", "snes9x_libretro.info"), "display_name = \"Nintendo - SNES / SFC (Snes9x)\"\nsystemname = \"Super Nintendo Entertainment System\"\n")
	write(t, filepath.Join(c.Path, "saves", "a.srm"), "x")
	write(t, filepath.Join(c.Path, "system", "bios.bin"), "x")
	write(t, filepath.Join(c.Games, "SNES", "Mario.sfc"), "rom")
	write(t, filepath.Join(c.Games, "SNES", "Zelda.zip"), "zip")
	write(t, filepath.Join(c.Games, "PS1", "Game (USA)", "Game (USA).cue"), "cue")
	write(t, filepath.Join(c.Games, "PS1", "Game (USA)", "Game (USA).bin"), "bin")
	// the same name twice: ambiguous, left alone
	write(t, filepath.Join(c.Games, "A", "Twin.gba"), "1")
	write(t, filepath.Join(c.Games, "B", "Twin.gba"), "2")
	write(t, filepath.Join(c.Path, "playlists", "SNES.lpl"), `{
  "version": "1.5",
  "default_core_path": "C:\\Old\\cores\\snes9x_libretro.dll",
  "items": [
    {
      "path": "C:\\Old Games\\SNES\\Mario.sfc",
      "label": "Mario \"World\"",
      "core_path": "DETECT"
    },
    {
      "path": "C:\\Old Games\\SNES\\Zelda.zip#Zelda.sfc",
      "core_path": "C:\\Old\\cores\\snes9x_libretro.dll"
    },
    {
      "path": "C:\\Old Games\\Twin.gba",
      "core_path": "DETECT"
    },
    {
      "path": "C:\\Old Games\\Gone.sfc",
      "core_path": "DETECT"
    }
  ]
}
`)
	write(t, filepath.Join(c.Path, "playlists", "builtin", "content_history.lpl"), `{"items": [{"path": "C:\\Old Games\\PS1\\Game (USA)\\Game (USA).cue", "core_path": "DETECT"}]}`)
	return c
}

func TestRead(t *testing.T) {
	c := fixture(t)
	in := Read(c, true)
	if !in.Found {
		t.Fatal("retroarch.exe not found")
	}
	if len(in.Cores) != 1 || in.Cores[0].Name != "Nintendo - SNES / SFC (Snes9x)" || in.Cores[0].System == "" {
		t.Errorf("cores: %+v", in.Cores)
	}
	if in.Saves != 1 || in.Bios != 1 || in.States != 0 {
		t.Errorf("saves=%d bios=%d states=%d", in.Saves, in.Bios, in.States)
	}
	// a .cue + .bin disc counts once
	want := map[string]int{"SNES": 2, "PS1": 1, "A": 1, "B": 1}
	for _, s := range in.Systems {
		if want[s.Name] != s.Games {
			t.Errorf("system %s: %d games, want %d", s.Name, s.Games, want[s.Name])
		}
	}
	if in.GameCount != 5 {
		t.Errorf("game count = %d", in.GameCount)
	}
	var snes *Playlist
	for i := range in.Playlists {
		if in.Playlists[i].Name == "SNES" {
			snes = &in.Playlists[i]
		}
	}
	if snes == nil || snes.Items != 4 || snes.Missing != 4 || snes.Fixable != 2 {
		t.Errorf("SNES playlist: %+v", snes)
	}
	if len(in.Settings) != 2 || in.Settings[0].Key != "video_driver" || in.Settings[0].Value != "vulkan" {
		t.Errorf("settings: %+v", in.Settings)
	}
	if in.Files == 0 || len(in.Folders) == 0 {
		t.Errorf("the folder was not measured: %+v", in.Folders)
	}
}

func TestFixPlaylists(t *testing.T) {
	c := fixture(t)
	fixes, err := FixPlaylists(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 2 {
		t.Fatalf("fixes: %+v", fixes)
	}
	raw, _ := os.ReadFile(filepath.Join(c.Path, "playlists", "SNES.lpl"))
	s := string(raw)
	esc := func(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }
	for _, want := range []string{
		esc(filepath.Join(c.Games, "SNES", "Mario.sfc")),
		esc(filepath.Join(c.Games, "SNES", "Zelda.zip")) + "#Zelda.sfc",
		esc(filepath.Join(c.Path, "cores", "snes9x_libretro.dll")),
		`"label": "Mario \"World\""`, // untouched
		`C:\\Old Games\\Twin.gba`,    // ambiguous: kept
		`C:\\Old Games\\Gone.sfc`,    // not found: kept
	} {
		if !strings.Contains(s, want) {
			t.Errorf("playlist lacks %s:\n%s", want, s)
		}
	}
	if strings.Contains(s, `C:\\Old\\cores`) {
		t.Errorf("the core paths were not fixed:\n%s", s)
	}
	for _, f := range fixes {
		if f.Name == "SNES" && (f.Games != 2 || f.Cores != 1 || len(f.Missing) != 2) {
			t.Errorf("SNES fix: %+v", f)
		}
	}
	hist, _ := os.ReadFile(filepath.Join(c.Path, "playlists", "builtin", "content_history.lpl"))
	if !strings.Contains(string(hist), esc(filepath.Join(c.Games, "PS1", "Game (USA)", "Game (USA).cue"))) {
		t.Errorf("history not fixed: %s", hist)
	}

	// a second run has nothing left to change
	again, err := FixPlaylists(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range again {
		if f.Games != 0 || f.Cores != 0 {
			t.Errorf("second run changed something: %+v", f)
		}
	}
}

func TestRequirements(t *testing.T) {
	c := fixture(t)
	r := Requirements(c, Read(c, false))
	if r[0].ID != ReqPlaylists || r[0].OK || !r[0].Fixable {
		t.Errorf("playlists: %+v", r[0])
	}
	if _, err := FixPlaylists(c); err != nil {
		t.Fatal(err)
	}
	// the ambiguous and the gone game stay missing, and cannot be fixed
	r = Requirements(c, Read(c, false))
	if r[0].OK || r[0].Fixable {
		t.Errorf("after the fix: %+v", r[0])
	}
}
