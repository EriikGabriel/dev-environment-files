// Package windhawk keeps the Windhawk mods and their settings in the repo.
//
// Windhawk 2 exports everything (its app settings, the installed mods, their
// settings and configuration) to one JSON file, and windhawk-cli.exe imports
// it back: it installs each mod from the repository and applies the
// settings. The repo keeps that file; envsetup compares it with what is
// installed, imports it and refreshes it from the machine.
package windhawk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Format is the "format" of a Windhawk 2 user-data export.
const Format = "windhawk-user-data-v1"

// BackupPath is where the export lives in the repo.
func BackupPath(root string) string { return filepath.Join(root, "windows", "windhawk", "backup.json") }

// Mod is one mod of an export.
type Mod struct {
	ID       string         `json:"modId"`
	Version  string         `json:"version"`
	Name     string         `json:"name"`
	Settings map[string]any `json:"settings,omitempty"`
	Config   map[string]any `json:"config,omitempty"`
	// Source is set for local mods and offline exports.
	Source string `json:"source,omitempty"`
}

// Disabled tells whether the export keeps the mod turned off.
func (m Mod) Disabled() bool {
	d, _ := m.Config["disabled"].(bool)
	return d
}

// Backup is a Windhawk 2 user-data export.
type Backup struct {
	Format      string         `json:"format"`
	AppSettings map[string]any `json:"appSettings,omitempty"`
	Mods        []Mod          `json:"mods"`
}

// Parse reads an export and checks its format.
func Parse(raw []byte) (*Backup, error) {
	var b Backup
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}), &b); err != nil {
		return nil, fmt.Errorf("not a Windhawk export: %w", err)
	}
	if b.Format != Format {
		return nil, fmt.Errorf("not a Windhawk 2 export (format %q, want %q)", b.Format, Format)
	}
	return &b, nil
}

// Load reads the export of the repo; nil without error when there is none.
func Load(path string) (*Backup, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return b, nil
}

// States of a mod when the repo is compared with the machine.
const (
	StateSame    = "same"    // installed as in the repo
	StateChanged = "changed" // installed, with another version, settings or on/off state
	StateMissing = "missing" // in the repo, not installed
	StateExtra   = "extra"   // installed, not in the repo
)

// ModStatus is one mod of the repo or of the machine, compared.
type ModStatus struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Version  string         `json:"version"` // in the repo ("" for extra)
	Enabled  bool           `json:"enabled"` // in the repo
	Settings map[string]any `json:"settings"`
	State    string         `json:"state"`
	// Installed is the version on the machine ("" when missing).
	Installed string `json:"installed"`
	// Changes says what differs on the machine, in the words shown to the user.
	Changes []string `json:"changes"`
}

// Compare lines up the mods of the repo (want) with the ones on the machine
// (have, an export of it). have nil means the machine was not read: every
// mod keeps an empty state.
func Compare(want, have *Backup) (mods []ModStatus, appChanged bool) {
	mods = []ModStatus{}
	on := map[string]Mod{}
	if have != nil {
		for _, m := range have.Mods {
			on[m.ID] = m
		}
	}
	seen := map[string]bool{}
	if want != nil {
		for _, m := range want.Mods {
			seen[m.ID] = true
			s := ModStatus{ID: m.ID, Name: m.Name, Version: m.Version, Enabled: !m.Disabled(), Settings: m.Settings, Changes: []string{}}
			if s.Settings == nil {
				s.Settings = map[string]any{}
			}
			if have != nil {
				s.State = compareMod(m, on, &s)
			}
			mods = append(mods, s)
		}
	}
	if have != nil {
		var extra []ModStatus
		for _, m := range have.Mods {
			if !seen[m.ID] {
				extra = append(extra, ModStatus{ID: m.ID, Name: m.Name, Installed: m.Version, Enabled: !m.Disabled(), Settings: map[string]any{}, State: StateExtra, Changes: []string{}})
			}
		}
		sort.Slice(extra, func(i, j int) bool { return extra[i].ID < extra[j].ID })
		mods = append(mods, extra...)
		if want != nil {
			appChanged = !reflect.DeepEqual(norm(want.AppSettings), norm(have.AppSettings))
		}
	}
	return mods, appChanged
}

func compareMod(m Mod, on map[string]Mod, s *ModStatus) string {
	h, ok := on[m.ID]
	if !ok {
		return StateMissing
	}
	s.Installed = h.Version
	if h.Version != m.Version {
		s.Changes = append(s.Changes, fmt.Sprintf("versão %s instalada", h.Version))
	}
	if h.Disabled() != m.Disabled() {
		if h.Disabled() {
			s.Changes = append(s.Changes, "desativado nesta máquina")
		} else {
			s.Changes = append(s.Changes, "ativado nesta máquina")
		}
	}
	keys := map[string]bool{}
	for k := range m.Settings {
		keys[k] = true
	}
	for k := range h.Settings {
		keys[k] = true
	}
	var diff []string
	for k := range keys {
		if !reflect.DeepEqual(norm(m.Settings[k]), norm(h.Settings[k])) {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	if len(diff) > 0 {
		s.Changes = append(s.Changes, "configurações diferentes: "+strings.Join(diff, ", "))
	}
	if len(s.Changes) > 0 {
		return StateChanged
	}
	return StateSame
}

// norm makes numbers compare equal whatever JSON type they were read into.
func norm(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

// Pending tells whether importing would change the machine: a mod of the
// repo is missing or differs. Extra mods do not count (import keeps them).
func Pending(mods []ModStatus) bool {
	for _, m := range mods {
		if m.State == StateMissing || m.State == StateChanged {
			return true
		}
	}
	return false
}

// CLI finds windhawk-cli.exe, which only Windhawk 2 has; "" when missing.
func CLI() string {
	for _, env := range []string{"ProgramFiles", "ProgramW6432"} {
		if dir := os.Getenv(env); dir != "" {
			p := filepath.Join(dir, "Windhawk", "windhawk-cli.exe")
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// ElevatedScript runs windhawk-cli once per call, in order, and stops at the
// first that fails. It runs elevated (Windhawk keeps its data in HKLM and
// ProgramData), so the output can only come back through files: call n
// writes log.n (stdout) and log.n.err (stderr); see ReadLogs.
func ElevatedScript(cli, log string, calls ...[]string) string {
	steps := make([]Step, len(calls))
	for i, c := range calls {
		steps[i] = Step{Args: c}
	}
	return ElevatedSteps(cli, log, steps...)
}

// Step is one thing the elevated script does: a windhawk-cli call (Args) or
// PowerShell (PS), such as writing the mod's local storage.
type Step struct {
	Args []string
	PS   string
}

// ElevatedSteps is ElevatedScript with PowerShell steps among the calls. A
// PowerShell step that fails writes its error where the call would have
// (log.n.err) and stops the script.
func ElevatedSteps(cli, log string, steps ...Step) string {
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	for n, s := range steps {
		if s.PS != "" {
			fmt.Fprintf(&b, "try {\n%s\n} catch { ($_ | Out-String) | Set-Content -LiteralPath %s -Encoding UTF8; exit 1 }\n", s.PS, q(fmt.Sprintf("%s.%d.err", log, n)))
			continue
		}
		args := s.Args
		parts := make([]string, len(args))
		for i, a := range args {
			// Start-Process joins the arguments with spaces: quote the ones that have any
			if strings.ContainsAny(a, " \t") {
				parts[i] = "('\"' + " + q(a) + " + '\"')"
			} else {
				parts[i] = q(a)
			}
		}
		fmt.Fprintf(&b, "$p = Start-Process -FilePath %s -ArgumentList @(%s) -RedirectStandardOutput %s -RedirectStandardError %s -Wait -PassThru -NoNewWindow\n",
			q(cli), strings.Join(parts, ", "), q(fmt.Sprintf("%s.%d", log, n)), q(fmt.Sprintf("%s.%d.err", log, n)))
		b.WriteString("if ($p.ExitCode) { exit $p.ExitCode }\n")
	}
	b.WriteString("exit 0\n")
	return b.String()
}

// ReadLogs joins what the calls of ElevatedScript printed.
func ReadLogs(log string, calls int) (stdout, stderr string) {
	var o, e strings.Builder
	for n := 0; n < calls; n++ {
		if raw, err := os.ReadFile(fmt.Sprintf("%s.%d", log, n)); err == nil {
			o.Write(raw)
			o.WriteString("\n")
		}
		if raw, err := os.ReadFile(fmt.Sprintf("%s.%d.err", log, n)); err == nil {
			e.Write(raw)
		}
	}
	return o.String(), e.String()
}

// ImportArgs import the export at file.
func ImportArgs(file string) []string {
	return []string{"--json", "--yes", "data", "import", file, "--confirm-app-restart"}
}

// The dock of the mod "Ilha e Dock estilo Mac" (mac-island-dock): its
// taskbarMode setting is dock, windowsTransparent, windowsApps or windows
// (the Windows taskbar left alone).
const (
	DockMod     = "mac-island-dock"
	DockSetting = "taskbarMode"
	DockOn      = "dock"
	DockOff     = "windows"
)

// DockModes are the values of taskbarMode.
var DockModes = []string{"dock", "windowsTransparent", "windowsApps", "windows"}

// SetDockArgs set the taskbar mode of the dock mod.
func SetDockArgs(mode string) []string {
	return []string{"--json", "mod", "settings", "set", DockMod, DockSetting + "=" + mode}
}

// InstallDockArgs install the dock mod from the Windhawk repository.
func InstallDockArgs() []string { return []string{"--json", "--yes", "mod", "install", DockMod} }

// ParseSetting reads the answer of `--json mod settings get <id> <key>`.
func ParseSetting(out, key string) (any, error) {
	var r struct {
		Success bool `json:"success"`
		Data    struct {
			Settings map[string]any `json:"settings"`
			Value    any            `json:"value"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &r); err != nil || !r.Success {
		return nil, fmt.Errorf("%s", CLIError(out))
	}
	if v, ok := r.Data.Settings[key]; ok {
		return v, nil
	}
	return r.Data.Value, nil
}

// CLIError takes the message out of a failed --json answer of the CLI.
func CLIError(out string) string {
	for _, line := range strings.Split(out, "\n") {
		var r struct {
			Success bool `json:"success"`
			Error   any  `json:"error"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &r) == nil && !r.Success && r.Error != nil {
			switch e := r.Error.(type) {
			case string:
				return e
			case map[string]any:
				if msg, ok := e["message"].(string); ok {
					return msg
				}
			}
			raw, _ := json.Marshal(r.Error)
			return string(raw)
		}
	}
	return strings.TrimSpace(out)
}
