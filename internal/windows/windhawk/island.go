package windhawk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// The mod "Ilha e Dock estilo Mac" keeps most of its options outside
// Windhawk's settings: in its local storage, the registry key IslandStorage,
// which `windhawk-cli data export` leaves out. The island (top bar, its
// widgets, the order of its items, profiles) lives there. The mod exports
// that storage itself to a text file, from its own settings window:
//
//	Mac Island & Dock backup
//	iislandBar=0          i: a number (a DWORD in the registry)
//	sislandOrder=6m-09…   s: a text, with \\ \n and \t escaped
//
// The repo keeps that file (IslandPath), without the personal values; the
// import writes it back to the storage, and the mod's own "import" accepts it
// as well.

// IslandHeader is the first line of the mod's backup file.
const IslandHeader = "Mac Island & Dock backup"

// IslandStorage is the registry key of the mod's local storage.
const IslandStorage = `HKEY_LOCAL_MACHINE\SOFTWARE\Windhawk\Engine\ModsWritable\` + DockMod + `\LocalStorage`

// IslandPath is where the repo keeps the backup of the mod's own options.
func IslandPath(root string) string {
	return filepath.Join(root, "windows", "windhawk", DockMod+".txt")
}

// IslandPersonal are the values that are the user's own content, not a look:
// where the weather is for (a city and its coordinates), the notes, tasks,
// events, agendas and alarms of the island. They never go to the repo, and an
// import keeps what the machine has.
var IslandPersonal = []string{"weatherPlace", "notes", "tasks", "events", "agendas", "alarms"}

// IslandEntry is one line of the backup.
type IslandEntry struct {
	Name string
	Text bool // s (text); else i (number)
	Int  int64
	Str  string
}

// Island is the mod's backup, in the order of the file. A name can repeat
// (the mod writes a few twice, with the same value).
type Island struct {
	Entries []IslandEntry
}

// ParseIsland reads the mod's backup file.
func ParseIsland(raw []byte) (*Island, error) {
	text := strings.ReplaceAll(string(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if strings.TrimSpace(lines[0]) != IslandHeader {
		return nil, fmt.Errorf("not a backup of the mod Ilha e Dock estilo Mac (the first line is not %q)", IslandHeader)
	}
	is := &Island{}
	for n, line := range lines[1:] {
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line[1:], "=")
		if !ok || name == "" || (line[0] != 'i' && line[0] != 's') {
			return nil, fmt.Errorf("line %d is not i<name>=<number> or s<name>=<text>: %.40q", n+2, line)
		}
		e := IslandEntry{Name: name, Text: line[0] == 's'}
		if e.Text {
			e.Str = unescapeIsland(value)
		} else {
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("line %d: %s is not a number: %q", n+2, name, value)
			}
			e.Int = v
		}
		is.Entries = append(is.Entries, e)
	}
	if len(is.Entries) == 0 {
		return nil, fmt.Errorf("the backup of the mod has no values")
	}
	return is, nil
}

// Bytes writes the backup the way the mod does: UTF-8 with a BOM, LF.
func (is *Island) Bytes() []byte {
	var b strings.Builder
	b.WriteString("\xef\xbb\xbf" + IslandHeader + "\n")
	for _, e := range is.Entries {
		if e.Text {
			b.WriteString("s" + e.Name + "=" + escapeIsland(e.Str) + "\n")
		} else {
			b.WriteString("i" + e.Name + "=" + strconv.FormatInt(e.Int, 10) + "\n")
		}
	}
	return []byte(b.String())
}

func escapeIsland(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\t", `\t`).Replace(s)
}

func unescapeIsland(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '\\':
			b.WriteByte('\\')
		default: // not an escape the mod writes: keep it as it is
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func islandPersonal(name string) bool {
	for _, p := range IslandPersonal {
		if strings.EqualFold(name, p) {
			return true
		}
	}
	return false
}

// Scrub empties the personal values, also inside the profiles (a profile is
// a whole backup kept as one text). It returns how many were emptied.
func (is *Island) Scrub() int {
	n := 0
	for i := range is.Entries {
		e := &is.Entries[i]
		if !e.Text {
			continue
		}
		if islandPersonal(e.Name) {
			if e.Str != "" {
				e.Str, n = "", n+1
			}
			continue
		}
		if strings.Contains(e.Str, "\n") {
			lines := strings.Split(e.Str, "\n")
			for j, l := range lines {
				if name, v, ok := strings.Cut(strings.TrimPrefix(l, "s"), "="); ok && strings.HasPrefix(l, "s") && islandPersonal(name) && v != "" {
					lines[j], n = "s"+name+"=", n+1
				}
			}
			e.Str = strings.Join(lines, "\n")
		}
	}
	return n
}

// IslandValue is a value as the registry holds it.
type IslandValue struct {
	Name string `json:"n"`
	// Kind is "dword" or "string".
	Kind string `json:"k"`
	Int  int64  `json:"i"`
	Str  string `json:"s"`
}

// Values are what an import writes: one per name (the last one wins), without
// the personal values that are empty in the repo, so the machine keeps its
// own. Numbers become the DWORDs the mod reads.
func (is *Island) Values() []IslandValue {
	last := map[string]int{}
	for i, e := range is.Entries {
		last[e.Name] = i
	}
	var out []IslandValue
	for i, e := range is.Entries {
		if last[e.Name] != i {
			continue
		}
		if e.Text {
			if e.Str == "" && islandPersonal(e.Name) {
				continue
			}
			out = append(out, IslandValue{Name: e.Name, Kind: "string", Str: e.Str})
			continue
		}
		out = append(out, IslandValue{Name: e.Name, Kind: "dword", Int: int64(int32(uint32(e.Int)))})
	}
	return out
}

// Update takes the machine's values for the names the backup has (a name the
// backup does not know, such as the per-account state the mod keeps, is
// left out) and empties the personal ones. It returns how many changed.
func (is *Island) Update(machine map[string]IslandValue) int {
	n := 0
	for i := range is.Entries {
		e := &is.Entries[i]
		v, ok := machine[e.Name]
		if !ok || islandPersonal(e.Name) {
			continue
		}
		switch v.Kind {
		case "string":
			if !e.Text || e.Str != v.Str {
				e.Text, e.Str, e.Int, n = true, v.Str, 0, n+1
			}
		case "dword":
			if e.Text || e.Int != v.Int {
				e.Text, e.Int, e.Str, n = false, v.Int, "", n+1
			}
		}
	}
	is.Scrub()
	return n
}

// Diff compares the backup with the machine (personal values left out):
// differ are the values the machine has otherwise, missing the ones it does
// not have at all (the mod stores an option once it is changed from its
// default, so a missing one may or may not match).
func (is *Island) Diff(machine map[string]IslandValue) (differ, missing int) {
	for _, v := range is.Values() {
		have, ok := machine[v.Name]
		switch {
		case !ok:
			missing++
		case have.Kind != v.Kind || have.Int != v.Int || have.Str != v.Str:
			differ++
		}
	}
	return differ, missing
}

// HasMod tells whether the export installs the mod.
func (b *Backup) HasMod(id string) bool {
	if b == nil {
		return false
	}
	for _, m := range b.Mods {
		if m.ID == id {
			return true
		}
	}
	return false
}

// ToggleDockArgs turn the dock mod off or on: the mod reads its storage when
// it starts, and writes it back when it stops, so the storage is written
// while it is off.
func ToggleDockArgs(on bool) []string {
	verb := "disable"
	if on {
		verb = "enable"
	}
	return []string{"--json", "mod", verb, DockMod}
}

func psq(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// IslandReadScript writes the mod's storage to out, as the JSON
// ParseIslandStorage reads. It needs no elevation.
func IslandReadScript(out string) string {
	return `$ErrorActionPreference = 'Stop'
$k = ` + psq(`Registry::`+IslandStorage) + `
$o = @{}
if (Test-Path -LiteralPath $k) {
  $item = Get-Item -LiteralPath $k
  foreach ($n in $item.GetValueNames()) {
    $kind = $item.GetValueKind($n).ToString()
    $v = $item.GetValue($n, $null, 'DoNotExpandEnvironmentNames')
    if ($kind -eq 'DWord') { $o[$n] = @{ k = 'dword'; i = [int64]$v } }
    elseif ($kind -eq 'String' -or $kind -eq 'ExpandString') { $o[$n] = @{ k = 'string'; s = [string]$v } }
  }
}
[IO.File]::WriteAllText(` + psq(out) + `, (ConvertTo-Json -InputObject $o -Depth 3 -Compress), (New-Object System.Text.UTF8Encoding $false))
`
}

// ParseIslandStorage reads what IslandReadScript wrote.
func ParseIslandStorage(raw []byte) (map[string]IslandValue, error) {
	var m map[string]struct {
		K string `json:"k"`
		I int64  `json:"i"`
		S string `json:"s"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")), &m); err != nil {
		return nil, fmt.Errorf("read the storage of the mod: %w", err)
	}
	out := make(map[string]IslandValue, len(m))
	for n, v := range m {
		out[n] = IslandValue{Name: n, Kind: v.K, Int: v.I, Str: v.S}
	}
	return out, nil
}

// IslandWriteScript writes the values in the JSON file (Values) to the mod's
// storage. HKLM: it runs elevated, as a step of ElevatedSteps.
func IslandWriteScript(valuesFile string) string {
	return `$k = ` + psq(`Registry::`+IslandStorage) + `
if (-not (Test-Path -LiteralPath $k)) { New-Item -Path $k -Force | Out-Null }
$vals = [IO.File]::ReadAllText(` + psq(valuesFile) + `, [Text.Encoding]::UTF8) | ConvertFrom-Json
foreach ($v in $vals) {
  if ($v.k -eq 'dword') { New-ItemProperty -LiteralPath $k -Name $v.n -Value ([int32]$v.i) -PropertyType DWord -Force | Out-Null }
  else { New-ItemProperty -LiteralPath $k -Name $v.n -Value ([string]$v.s) -PropertyType String -Force | Out-Null }
}`
}
