package playnite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Requirement is something Playnite needs that its folder does not carry.
type Requirement struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	About  string `json:"about"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
	// Fixable tells whether `playnite-fix <id>` can take care of it.
	Fixable bool `json:"fixable"`
	// Elevated tells whether the fix asks for elevation (UAC).
	Elevated bool `json:"elevated"`
}

// The requirements that can be fixed.
const (
	ReqMediaPlayer     = "media-player"
	ReqStartMenu       = "start-menu"
	ReqFfmpegNormalize = "ffmpeg-normalize"
)

// Host are the folders of this machine the checks look at.
type Host struct {
	Windir  string // C:\Windows
	AppData string // the user's Roaming folder
}

// ThisHost reads Host from the environment.
func ThisHost() Host {
	w := os.Getenv("SystemRoot")
	if w == "" {
		w = `C:\Windows`
	}
	return Host{Windir: w, AppData: os.Getenv("APPDATA")}
}

// StartMenuLink is the shortcut register-portable-app.ps1 makes.
func (h Host) StartMenuLink() string {
	return filepath.Join(h.AppData, "Microsoft", "Windows", "Start Menu", "Programs", "Playnite.lnk")
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Requirements checks what the Playnite of c needs outside its folder.
func Requirements(c Config, in Install, h Host) []Requirement {
	out := []Requirement{{
		ID:    ReqMediaPlayer,
		Title: "Windows Media Player (trailers)",
		About: "O Playnite toca os trailers e vídeos de fundo com o player do WPF, que usa o wmp.dll do Windows Media Player. " +
			"Ele é um recurso opcional do Windows (Media.WindowsMediaPlayer): sem ele os vídeos não tocam. " +
			"Corrigir roda Add-WindowsCapability, que baixa o recurso do Windows Update.",
		OK:       exists(filepath.Join(h.Windir, "System32", "wmp.dll")),
		Fixable:  true,
		Elevated: true,
	}}
	if c.StartMenu {
		out = append(out, Requirement{
			ID:      ReqStartMenu,
			Title:   "Atalho no menu Iniciar",
			About:   "O Playnite portable não tem instalador: um atalho no menu Iniciar e uma entrada em Configurações › Apps fazem a busca do Windows (e o Raycast) acharem ele. Remover a entrada só tira o registro, a pasta fica.",
			OK:      exists(h.StartMenuLink()),
			Detail:  h.StartMenuLink(),
			Fixable: in.Found,
		})
	}
	for _, x := range Externals(c, in.Extensions) {
		r := Requirement{
			ID:     "path:" + x.Key,
			Title:  fmt.Sprintf("%s: %s", x.Extension, x.Key),
			About:  "Um caminho fora da pasta do Playnite nas configurações da extensão; o snapshot não leva esse programa.",
			OK:     exists(x.Path),
			Detail: x.Path,
		}
		if strings.EqualFold(filepath.Base(x.Path), "ffmpeg-normalize.exe") {
			r.ID, r.Fixable = ReqFfmpegNormalize, true
			r.About = "O ffmpeg-normalize (Python) normaliza o volume das músicas de tema. Corrigir instala com o pip e aponta a extensão para onde ele ficou."
		}
		out = append(out, r)
	}
	return out
}

// MediaPlayerScript installs the Windows Media Player capability. It runs
// elevated.
const MediaPlayerScript = `$ErrorActionPreference = 'Stop'
$c = Get-WindowsCapability -Online -Name 'Media.WindowsMediaPlayer*' | Select-Object -First 1
if (-not $c) { throw 'this Windows does not offer the Media.WindowsMediaPlayer capability' }
if ($c.State -ne 'Installed') { Add-WindowsCapability -Online -Name $c.Name | Out-Null }
`

// FfmpegNormalizeScript installs ffmpeg-normalize with pip and prints where
// its program is, on the last line.
const FfmpegNormalizeScript = `$ErrorActionPreference = 'Stop'
$py = Get-Command py, python -ErrorAction SilentlyContinue | Where-Object { $_.Source -notlike '*WindowsApps*' } | Select-Object -First 1
if (-not $py) { throw 'Python was not found: install it first (winget install Python.Python.3.13)' }
& $py.Source -m pip install --upgrade --disable-pip-version-check ffmpeg-normalize | Out-Host
if ($LASTEXITCODE) { throw "pip failed ($LASTEXITCODE)" }
$dir = & $py.Source -c "import sysconfig; print(sysconfig.get_path('scripts'))"
Join-Path $dir.Trim() 'ffmpeg-normalize.exe'
`

// SetExternal points every setting that names old at new, in the settings of
// the extensions. It returns the files changed.
func SetExternal(c Config, old, new string) ([]string, error) {
	q := func(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }
	var changed []string
	files, _ := filepath.Glob(filepath.Join(c.Path, "ExtensionsData", "*", "config.json"))
	for _, p := range files {
		raw, err := os.ReadFile(p)
		if err != nil {
			return changed, err
		}
		s := string(raw)
		if !strings.Contains(s, `"`+q(old)+`"`) {
			continue
		}
		s = strings.ReplaceAll(s, `"`+q(old)+`"`, `"`+q(new)+`"`)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			return changed, err
		}
		changed = append(changed, p)
	}
	return changed, nil
}
