package retroarch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Requirement is something the RetroArch setup needs that a plain copy of
// the folders does not give.
type Requirement struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	About   string `json:"about"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Fixable bool   `json:"fixable"`
}

// The requirements that `retroarch-fix` takes care of.
const (
	ReqPlaylists = "playlists"
	ReqStartMenu = "start-menu"
)

// StartMenuLink is the shortcut register-portable-app.ps1 makes.
func StartMenuLink() string {
	return filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "RetroArch.lnk")
}

// Requirements checks the playlists and the Start menu entry.
func Requirements(c Config, in Install) []Requirement {
	missing, fixable := 0, 0
	for _, p := range in.Playlists {
		// the history keeps games that were deleted since: those are not a
		// problem, only the ones that can be pointed at the games again
		if strings.HasPrefix(p.Name, "builtin/") {
			missing += p.Fixable
		} else {
			missing += p.Missing
		}
		fixable += p.Fixable
	}
	pl := Requirement{
		ID:    ReqPlaylists,
		Title: "Playlists apontando para os jogos",
		About: "As playlists guardam o caminho completo de cada jogo e core. Quando as pastas mudam de lugar (outra máquina, " +
			"outro disco), o RetroArch mostra o jogo mas não o abre. Corrigir procura cada arquivo pelo nome na pasta de jogos " +
			"e na de cores e troca só os caminhos que não existem.",
		OK:      missing == 0,
		Fixable: fixable > 0,
	}
	switch {
	case missing == 0:
		pl.Detail = "todos os jogos das playlists foram encontrados"
	case fixable == missing:
		pl.Detail = fmt.Sprintf("%d jogos com caminho antigo; todos podem ser corrigidos", missing)
	default:
		pl.Detail = fmt.Sprintf("%d jogos com caminho antigo; %d podem ser corrigidos, os outros não estão na pasta de jogos", missing, fixable)
	}
	sm := Requirement{
		ID:    ReqStartMenu,
		Title: "Atalho no menu Iniciar",
		About: "O RetroArch portable não tem instalador: o atalho no menu Iniciar e a entrada em Aplicativos instalados são " +
			"criados pelo envsetup, sem mover a pasta.",
		OK:      exists(StartMenuLink()),
		Fixable: in.Found,
	}
	if !sm.OK {
		sm.Detail = "sem atalho no menu Iniciar"
	}
	return []Requirement{pl, sm}
}
