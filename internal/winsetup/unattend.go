package winsetup

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// GeneratorURL is the site that builds autounattend.xml files. A file made
// there carries, in a comment, a link that reopens the form with every
// setting it was generated from.
const GeneratorURL = "https://schneegans.de/windows/unattend-generator/"

// Item is one setting of the answer file.
type Item struct {
	Label string `json:"label"`
	Value string `json:"value,omitempty"`
}

// Section groups settings for display.
type Section struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}

// Script is a custom script embedded in the answer file.
type Script struct {
	Phase string `json:"phase"` // when it runs
	Type  string `json:"type"`  // Ps1, Cmd, Reg...
	Body  string `json:"body"`
}

// Shell is how the answer file sets up the desktop: what the layout preview
// shows before the user's own choices.
type Shell struct {
	TaskbarLeft    bool `json:"taskbar_left"`
	HideSearch     bool `json:"hide_search"`
	HideTaskView   bool `json:"hide_task_view"`
	NoWidgets      bool `json:"no_widgets"`
	StartPinsEmpty bool `json:"start_pins_empty"`
	// DesktopIcons is the generator's mode: Default or Custom. Custom keeps
	// only the system icons listed in DesktopSystemIcons.
	DesktopIcons       string   `json:"desktop_icons"`
	DesktopSystemIcons []string `json:"desktop_system_icons"`
	NoEdgeDesktopIcon  bool     `json:"no_edge_desktop_icon"`
}

// Install is something the answer file installs by script at first logon.
type Install struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Kind is ninite (the installer on the drive), github (the latest release
	// of a repository) or runtime (not an app with a window).
	Kind string `json:"kind"`
}

// Overview is what an autounattend.xml does, read from the generator link it
// carries.
type Overview struct {
	// URL reopens the generator with this file's settings; "" when the file
	// has no such link (it was not made by the generator, or was edited).
	URL      string    `json:"url"`
	Sections []Section `json:"sections"`
	Scripts  []Script  `json:"scripts"`
	// NiniteHook tells whether a script looks for a Ninite installer on the
	// drives at first logon.
	NiniteHook bool `json:"ninite_hook"`
	// HasPassword is set when an account has a password: it is readable in
	// the file and in the link, so the file must not go to a public repo.
	HasPassword bool      `json:"has_password"`
	Shell       Shell     `json:"shell"`
	Installs    []Install `json:"installs"`
}

var generatorLink = regexp.MustCompile(`<!--\s*(` + regexp.QuoteMeta(GeneratorURL) + `\?\S+?)\s*-->`)

// ReadUnattend checks that data is an unattend answer file and describes it.
func ReadUnattend(data []byte) (Overview, error) {
	ov := Overview{Sections: []Section{}, Scripts: []Script{}, Installs: []Install{}, Shell: Shell{DesktopSystemIcons: []string{}}}
	if err := checkUnattendXML(data); err != nil {
		return ov, err
	}
	m := generatorLink.FindSubmatch(data)
	if m == nil {
		return ov, nil
	}
	ov.URL = string(m[1])
	_, query, _ := strings.Cut(ov.URL, "?")
	describe(&ov, parseQuery(query))
	return ov, nil
}

// checkUnattendXML makes sure the document is well-formed and its root is
// <unattend>.
func checkUnattendXML(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	root := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF && root != "" {
			return nil
		}
		if err != nil {
			return fmt.Errorf("not a valid XML document: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && root == "" {
			root = se.Name.Local
			if root != "unattend" {
				return fmt.Errorf("the root element is <%s>, not <unattend>: this is not an answer file", root)
			}
		}
	}
}

type param struct{ key, value string }

// parseQuery keeps the order of the link, which is the order of the form.
func parseQuery(q string) []param {
	var out []param
	for _, kv := range strings.Split(q, "&") {
		k, v, _ := strings.Cut(kv, "=")
		k, err1 := url.QueryUnescape(k)
		v, err2 := url.QueryUnescape(v)
		if k == "" || err1 != nil || err2 != nil {
			continue
		}
		out = append(out, param{k, v})
	}
	return out
}

// Labels of the settings worth a sentence; anything else is shown by its
// name in the generator, split into words.
var (
	regionLabels = map[string]string{
		"UILanguage": "Idioma do Windows", "Locale": "Formato regional", "Keyboard": "Teclado",
		"Locale2": "Segundo idioma de entrada", "Keyboard2": "Segundo teclado",
		"Locale3": "Terceiro idioma de entrada", "Keyboard3": "Terceiro teclado",
		"GeoLocation": "Localização (GeoID)", "TimeZone": "Fuso horário",
	}
	installLabels = map[string]string{
		"WindowsEdition": "Edição", "ProductKey": "Chave do produto", "ProcessorArchitecture": "Arquitetura",
		"ComputerName": "Nome do computador", "ComputerNameMode": "Nome do computador", "PEMode": "Windows PE",
		"PartitionMode": "Particionamento", "PartitionLayout": "Tabela de partições",
		"WifiMode": "Wi-Fi", "ExpressSettings": "Configurações de privacidade",
	}
	accountLabels = map[string]string{
		"UserAccountMode": "Criação de contas", "AutoLogonMode": "Login automático",
		"PasswordExpirationMode": "Expiração de senha", "LockoutMode": "Bloqueio por tentativas",
	}
	tweakLabels = map[string]string{
		"BypassRequirementsCheck": "Ignora os requisitos do Windows 11 (TPM, Secure Boot, RAM)",
		"BypassNetworkCheck":      "Permite instalar sem internet",
		"HidePowerShellWindows":   "Oculta as janelas do PowerShell durante a instalação",
		"ObscurePasswords":        "Senhas ofuscadas no arquivo (Base64, reversível)",
		"ShowFileExtensions":      "Mostra as extensões de arquivo",
		"LaunchToThisPC":          "Explorador abre em Este Computador",
		"ShowEndTask":             "“Finalizar tarefa” no menu da barra de tarefas",
		"DisableWidgets":          "Widgets desativados",
		"HideTaskViewButton":      "Botão Visão de Tarefas oculto",
		"DisableBingResults":      "Busca sem resultados do Bing",
		"DisableSac":              "Smart App Control desativado",
		"DisableSmartScreen":      "SmartScreen desativado",
		"DisableDefender":         "Microsoft Defender desativado",
		"DisableUac":              "UAC desativado",
		"DisableWindowsUpdate":    "Windows Update desativado",
		"DisableFastStartup":      "Inicialização rápida desativada",
		"DisableSystemRestore":    "Restauração do sistema desativada",
		"EnableLongPaths":         "Caminhos longos habilitados",
		"EnableRemoteDesktop":     "Área de trabalho remota habilitada",
		"HardenSystemDriveAcl":    "Permissões da unidade do sistema reforçadas",
		"AllowPowerShellScripts":  "Scripts do PowerShell permitidos (RemoteSigned)",
		"DisableLastAccess":       "Sem registro de último acesso (NTFS)",
		"DisableAppSuggestions":   "Sem sugestões nem apps patrocinados",
		"PreventDeviceEncryption": "Sem criptografia automática do dispositivo (BitLocker)",
		"PreventAutomaticReboot":  "Sem reinício automático com usuário logado",
		"TurnOffSystemSounds":     "Sons do sistema desligados",
		"ClassicContextMenu":      "Menu de contexto clássico",
		"LeftTaskbar":             "Barra de tarefas alinhada à esquerda",
		"HideEdgeFre":             "Edge sem tela de boas-vindas",
		"DisableEdgeStartupBoost": "Edge sem Startup Boost",
		"MakeEdgeUninstallable":   "Edge pode ser desinstalado",
		"DeleteEdgeDesktopIcon":   "Sem ícone do Edge na área de trabalho",
		"DisablePointerPrecision": "Sem aceleração do mouse",
		"DeleteWindowsOld":        "Apaga a pasta Windows.old",
		"VBoxGuestAdditions":      "Instala o VirtualBox Guest Additions",
		"VMwareTools":             "Instala o VMware Tools",
		"VirtIoGuestTools":        "Instala o VirtIO Guest Tools",
	}
	phaseLabels = map[string]string{
		"System": "Sistema, antes de criar as contas", "DefaultUser": "Perfil padrão de usuário",
		"FirstLogon": "Primeiro login", "UserOnce": "Primeiro login de cada usuário",
	}
	// Covered by another setting, or noise.
	hidden = map[string]bool{"LanguageMode": true, "UseKeyboard2": true, "UseKeyboard3": true, "TimeZoneMode": true, "WindowsEditionMode": true}

	scriptKey  = regexp.MustCompile(`^(System|DefaultUser|FirstLogon|UserOnce)Script(Type)?(\d+)$`)
	accountKey = regexp.MustCompile(`^Account(Name|DisplayName|Password|Group)(\d+)$`)
)

func describe(ov *Overview, params []param) {
	var region, install, accounts, tweaks, others, removed []Item
	scripts := map[string]*Script{}
	var scriptOrder []string
	names := map[string]string{}

	for _, p := range params {
		k, v := p.key, p.value
		if m := scriptKey.FindStringSubmatch(k); m != nil {
			id := m[1] + m[3]
			s := scripts[id]
			if s == nil {
				s = &Script{Phase: phaseLabels[m[1]]}
				scripts[id] = s
				scriptOrder = append(scriptOrder, id)
			}
			if m[2] == "" {
				s.Body = strings.ReplaceAll(v, "\r\n", "\n")
			} else {
				s.Type = v
			}
			continue
		}
		if m := accountKey.FindStringSubmatch(k); m != nil {
			switch m[1] {
			case "Name":
				names[m[2]] = v
			case "Group":
				accounts = append(accounts, Item{"Conta " + names[m[2]], v})
			case "Password":
				ov.HasPassword = ov.HasPassword || v != ""
			}
			continue
		}
		ov.Shell.read(k, v)
		switch {
		case hidden[k]:
		case regionLabels[k] != "":
			region = append(region, Item{regionLabels[k], v})
		case installLabels[k] != "":
			install = append(install, Item{installLabels[k], v})
		case accountLabels[k] != "":
			accounts = append(accounts, Item{accountLabels[k], v})
		case strings.HasPrefix(k, "Remove") && v == "true":
			removed = append(removed, Item{Label: words(strings.TrimPrefix(k, "Remove"))})
		case v == "true":
			label := tweakLabels[k]
			if label == "" {
				label = words(k)
			}
			tweaks = append(tweaks, Item{Label: label})
		case v == "" || v == "false" || v == "Default" || v == "Skip":
			// left as Windows ships it
		default:
			others = append(others, Item{words(strings.TrimSuffix(k, "Mode")), v})
		}
	}
	for _, s := range []Section{
		{"Idioma e região", region}, {"Instalação", install}, {"Contas", accounts},
		{"Ajustes ativados", tweaks}, {"Outras opções", others}, {"Apps e recursos removidos", removed},
	} {
		if len(s.Items) > 0 {
			ov.Sections = append(ov.Sections, s)
		}
	}
	for _, id := range scriptOrder {
		s := scripts[id]
		if strings.TrimSpace(s.Body) == "" {
			continue
		}
		ov.Scripts = append(ov.Scripts, *s)
		ov.NiniteHook = ov.NiniteHook || strings.Contains(strings.ToLower(s.Body), "ninite")
		ov.addInstalls(s.Body)
	}
}

func (s *Shell) read(k, v string) {
	on := v == "true"
	switch k {
	case "LeftTaskbar":
		s.TaskbarLeft = on
	case "TaskbarSearch":
		s.HideSearch = v == "Hide"
	case "HideTaskViewButton":
		s.HideTaskView = on
	case "DisableWidgets":
		s.NoWidgets = on
	case "StartPinsMode":
		s.StartPinsEmpty = v == "Empty"
	case "DesktopIconsMode":
		s.DesktopIcons = v
	case "DeleteEdgeDesktopIcon":
		s.NoEdgeDesktopIcon = on
	default:
		if name, ok := strings.CutPrefix(k, "DesktopIcon"); ok && on && name != "sMode" && name != "" {
			s.DesktopSystemIcons = append(s.DesktopSystemIcons, words(name))
		}
	}
}

var (
	githubRepo = regexp.MustCompile(`\$repository\s*=\s*'([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)'`)
	vcRedist   = regexp.MustCompile(`(?i)vc_redist`)
)

// addInstalls recognises what a first-logon script installs. Only the
// patterns the generator's own examples use are known; the rest stays
// visible as a script.
func (ov *Overview) addInstalls(body string) {
	add := func(i Install) {
		for _, have := range ov.Installs {
			if have.ID == i.ID {
				return
			}
		}
		ov.Installs = append(ov.Installs, i)
	}
	if strings.Contains(strings.ToLower(body), "ninite") {
		add(Install{"ninite", "Ninite", "ninite"})
	}
	if m := githubRepo.FindStringSubmatch(body); m != nil {
		id := strings.ToLower(m[2])
		add(Install{id, strings.ToUpper(id[:1]) + id[1:], "github"})
	}
	if vcRedist.MatchString(body) {
		add(Install{"vc-redist", "Visual C++ Redistributable", "runtime"})
	}
	if strings.Contains(body, "NetFx3") {
		add(Install{"netfx3", ".NET Framework 3.5", "runtime"})
	}
}

// words splits a CamelCase name: Remove3DViewer's "3DViewer" becomes
// "3D Viewer", "Office365" becomes "Office 365".
func words(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i, c := range r {
		if i > 0 {
			prev := r[i-1]
			next := rune(0)
			if i+1 < len(r) {
				next = r[i+1]
			}
			switch {
			case unicode.IsUpper(c) && unicode.IsLower(prev),
				unicode.IsUpper(c) && unicode.IsUpper(prev) && unicode.IsLower(next),
				unicode.IsDigit(c) && unicode.IsLetter(prev),
				unicode.IsUpper(c) && unicode.IsDigit(prev) && unicode.IsLower(next):
				b.WriteByte(' ')
			}
		}
		b.WriteRune(c)
	}
	return b.String()
}
