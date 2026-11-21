// Package policy applies Windows group-policy settings by writing the
// registry values the Group Policy editor would write. The list lives in
// config/policies.yaml, with an explanation and a choice for each one.
package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Choices every policy accepts besides its own options.
const (
	ChoiceVideo = "video" // the value shown in the source video
	ChoiceOff   = "off"   // not configured: the values are removed
)

// Value is one DWORD under a policy key, or (Delete) the order to delete it.
type Value struct {
	Key   string `yaml:"key" json:"key"` // HKLM\SOFTWARE\Policies\...
	Name  string `yaml:"name" json:"name"`
	Value uint32 `yaml:"value" json:"value"`
	// Delete is what gpedit writes for a policy, or a part of one, it set to
	// Disabled that has no value of its own for that state: a "**del.<name>"
	// entry in the Registry.pol, which deletes the value from the registry.
	// Without it gpedit reads the policy as not configured (or as enabled,
	// when another value is left behind).
	Delete bool `yaml:"delete,omitempty" json:"delete,omitempty"`
}

// Option is an alternative to the video's setting.
type Option struct {
	ID    string  `yaml:"id" json:"id"`
	Title string  `yaml:"title" json:"title"`
	Set   []Value `yaml:"set" json:"set"`
}

// Policy is one setting of the Group Policy editor.
type Policy struct {
	ID    string `yaml:"id" json:"id"`
	Group string `yaml:"group" json:"group"`
	// Path is where it is in gpedit.msc, under Computer Configuration >
	// Administrative Templates.
	Path  string `yaml:"path" json:"path"`
	Title string `yaml:"title" json:"title"`
	// Aliases are other names the policy has in gpedit, in other Windows
	// builds; the "open in gpedit" link looks for them too.
	Aliases []string `yaml:"aliases" json:"aliases,omitempty"`
	// Video is the state the video sets, as gpedit words it.
	Video string `yaml:"video" json:"video"`
	// Default is what Windows 11 does with the policy not configured, the
	// state it comes in.
	Default string `yaml:"default" json:"default"`
	About   string `yaml:"about" json:"about"`
	// Caution is shown when the setting can get in the way.
	Caution string `yaml:"caution" json:"caution,omitempty"`
	// Choice is video, off or the id of an option.
	Choice  string   `yaml:"choice" json:"choice"`
	Set     []Value  `yaml:"set" json:"set"`
	Options []Option `yaml:"options" json:"options"`
}

// File is config/policies.yaml.
type File struct {
	// Source is where the settings come from.
	Source   string   `yaml:"source" json:"source"`
	Policies []Policy `yaml:"policies" json:"policies"`
}

var (
	idRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	keyRe  = regexp.MustCompile(`^HKLM\\SOFTWARE\\(Policies\\|Microsoft\\Windows\\CurrentVersion\\Policies\\)[A-Za-z0-9 _.\\-]+$`)
	nameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

// Load reads and validates a policies file.
func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	seen := map[string]bool{}
	for i := range f.Policies {
		p := &f.Policies[i]
		if !idRe.MatchString(p.ID) || seen[p.ID] {
			return nil, fmt.Errorf("%s: bad or duplicate policy id %q", path, p.ID)
		}
		seen[p.ID] = true
		if p.Options == nil {
			p.Options = []Option{}
		}
		p.addOpposite()
		if p.Choice == "" {
			p.Choice = ChoiceVideo
		}
		if len(p.Set) == 0 {
			return nil, fmt.Errorf("%s: policy %q sets nothing", path, p.ID)
		}
		sets := [][]Value{p.Set}
		for _, o := range p.Options {
			if !idRe.MatchString(o.ID) || o.ID == ChoiceVideo || o.ID == ChoiceOff {
				return nil, fmt.Errorf("%s: policy %q: bad option id %q", path, p.ID, o.ID)
			}
			sets = append(sets, o.Set)
		}
		for _, set := range sets {
			for _, v := range set {
				if !keyRe.MatchString(v.Key) || strings.Contains(v.Key, `\\`) || !nameRe.MatchString(v.Name) {
					return nil, fmt.Errorf("%s: policy %q: %s\\%s is not a policy value under HKLM\\SOFTWARE", path, p.ID, v.Key, v.Name)
				}
				if v.Delete && v.Value != 0 {
					return nil, fmt.Errorf("%s: policy %q: %s is deleted, it cannot also have a value", path, p.ID, v.Name)
				}
			}
		}
		if !p.valid(p.Choice) {
			return nil, fmt.Errorf("%s: policy %q: unknown choice %q", path, p.ID, p.Choice)
		}
	}
	return &f, nil
}

// addOpposite gives a plain on/off policy the state the video did not pick,
// so it can be flipped in the app. It only applies to policies with no
// options of their own whose values are all 0 or 1 and whose video state is
// exactly "Habilitado" or "Desabilitado"; the others list their options in
// the file.
func (p *Policy) addOpposite() {
	if len(p.Options) > 0 || len(p.Set) == 0 {
		return
	}
	var id, title string
	switch p.Video {
	case "Habilitado":
		id, title = "disabled", "Desabilitado"
	case "Desabilitado":
		id, title = "enabled", "Habilitado"
	default:
		return
	}
	flipped := make([]Value, len(p.Set))
	for i, v := range p.Set {
		if v.Value > 1 || v.Delete {
			return
		}
		v.Value = 1 - v.Value
		flipped[i] = v
	}
	p.Options = []Option{{ID: id, Title: title, Set: flipped}}
}

func (p Policy) valid(choice string) bool {
	if choice == ChoiceVideo || choice == ChoiceOff {
		return true
	}
	for _, o := range p.Options {
		if o.ID == choice {
			return true
		}
	}
	return false
}

// Wanted returns the values the current choice writes and the ones it
// removes (those of the other choices, back to "not configured").
func (p Policy) Wanted() (set, remove []Value) {
	switch p.Choice {
	case ChoiceOff:
	case ChoiceVideo, "":
		set = p.Set
	default:
		for _, o := range p.Options {
			if o.ID == p.Choice {
				set = o.Set
			}
		}
	}
	kept := map[string]bool{}
	for _, v := range set {
		kept[v.Key+`\`+v.Name] = true
	}
	all := append([]Value{}, p.Set...)
	for _, o := range p.Options {
		all = append(all, o.Set...)
	}
	for _, v := range all {
		if id := v.Key + `\` + v.Name; !kept[id] {
			kept[id] = true
			remove = append(remove, v)
		}
	}
	return set, remove
}

func regPath(key string) string {
	return "Registry::HKEY_LOCAL_MACHINE" + strings.TrimPrefix(key, "HKLM")
}

const readValue = `function V($k, $n) { try { (Get-ItemProperty -LiteralPath $k -Name $n -ErrorAction Stop).$n } catch { $null } }` + "\n"

// StateScript is a PowerShell script that prints, as JSON, whether each
// policy is as chosen ({"id": true}): in the registry, which Windows obeys,
// and in the local Registry.pol, which is what gpedit.msc shows and what a
// policy refresh writes back to the registry. Checking only the registry
// took a policy whose Registry.pol entry was missing or unreadable (an old
// file, a value set by another tool) as applied, so it was never written
// again and gpedit kept showing it as not configured.
func StateScript(policies []Policy) string {
	var b strings.Builder
	b.WriteString(readValue)
	b.WriteString(polLib)
	// an unreadable file, or one in the format of envsetup 0.1.0-0.1.7, is
	// not what gpedit reads: every policy that needs it is pending
	b.WriteString("$polOk = $true\ntry { PolLoad } catch { $polOk = $false }\nif ($script:polOld) { $polOk = $false }\n")
	b.WriteString("$gptOk = PolGptOk\n")
	b.WriteString("$r = [ordered]@{}\n")
	for _, p := range policies {
		set, remove := p.Wanted()
		conds := []string{"$polOk"}
		if len(set) > 0 {
			conds = append(conds, "$gptOk")
		}
		for _, v := range set {
			if v.Delete {
				conds = append(conds,
					fmt.Sprintf("($null -eq (V '%s' '%s'))", regPath(v.Key), v.Name),
					fmt.Sprintf("(PolIsDel '%s' '%s')", polKey(v.Key), v.Name))
				continue
			}
			conds = append(conds,
				fmt.Sprintf("((V '%s' '%s') -eq %d)", regPath(v.Key), v.Name, v.Value),
				fmt.Sprintf("(PolIs '%s' '%s' %d)", polKey(v.Key), v.Name, v.Value))
		}
		for _, v := range remove {
			conds = append(conds,
				fmt.Sprintf("($null -eq (V '%s' '%s'))", regPath(v.Key), v.Name),
				fmt.Sprintf("(PolNone '%s' '%s')", polKey(v.Key), v.Name))
		}
		fmt.Fprintf(&b, "$r['%s'] = [bool](%s)\n", p.ID, strings.Join(conds, " -and "))
	}
	b.WriteString("$r | ConvertTo-Json -Compress\n")
	return b.String()
}

// ParseState reads the output of StateScript.
func ParseState(out string) (map[string]bool, error) {
	st := map[string]bool{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &st); err != nil {
		return nil, fmt.Errorf("unexpected answer while reading the policies: %w", err)
	}
	return st, nil
}

// polLib is the PowerShell that reads and writes the local Registry.pol, the
// file gpedit.msc reads: PolLoad reads it, PolIs / PolIsDel / PolNone check
// an entry, PolSet / PolSetDel / PolDel change one and PolSave writes the
// file and gpt.ini. The format is documented as "Registry Policy File
// Format" (PReg): the header "PReg" + version 1, then entries
// [key;value;type;size;data]. Strings and separators are UTF-16LE; the key
// and the value name each end with a NUL (UTF-16 0x0000) before their ";":
// without it Windows reads past the end of the file and gpedit.msc /
// gpupdate fail with "end of file reached". Type, size and data are binary
// little-endian. A value name "**del.<name>" is the order to delete <name>,
// which gpedit writes for some policies set to Disabled.
const polLib = `$polFile = Join-Path $env:SystemRoot 'System32\GroupPolicy\Machine\Registry.pol'
$polDir = Split-Path $polFile
$script:pol = New-Object System.Collections.ArrayList
$script:bytes = $null
$script:pos = 0
$script:polOld = $false
function ReadStr {
  $sb = New-Object System.Text.StringBuilder
  while ($script:pos + 1 -lt $script:bytes.Length) {
    $c = [BitConverter]::ToUInt16($script:bytes, $script:pos); $script:pos += 2
    if ($c -eq 0x3B) {
      # the NUL that ends the string comes before the ";"; files written by
      # envsetup 0.1.0-0.1.7 left it out, and are read and rewritten right
      $s = $sb.ToString()
      if (-not $s.EndsWith([string][char]0)) { $script:polOld = $true }
      return $s.TrimEnd([char]0)
    }
    [void]$sb.Append([char]$c)
  }
  throw 'Invalid Registry.pol: missing string separator.'
}
function ReadU32 {
  if ($script:pos + 4 -gt $script:bytes.Length) { throw 'Invalid Registry.pol: truncated integer.' }
  $v = [BitConverter]::ToUInt32($script:bytes, $script:pos); $script:pos += 4; $v
}
function PolLoad {
  if (-not (Test-Path -LiteralPath $polFile)) { return }
  $script:bytes = [IO.File]::ReadAllBytes($polFile)
  if ($script:bytes.Length -lt 8 -or [Text.Encoding]::ASCII.GetString($script:bytes, 0, 4) -ne 'PReg' -or [BitConverter]::ToUInt32($script:bytes, 4) -ne 1) {
    throw 'Invalid Registry.pol header; envsetup did not change it. Back up or repair the file, then apply policies again.'
  }
  $script:pos = 8
  while ($script:pos -lt $script:bytes.Length) {
    if ($script:pos + 2 -gt $script:bytes.Length -or [BitConverter]::ToUInt16($script:bytes, $script:pos) -ne 0x5B) { throw 'Invalid Registry.pol entry.' }
    $script:pos += 2
    $k = ReadStr
    $n = ReadStr
    $t = ReadU32
    if ($script:pos + 2 -gt $script:bytes.Length -or [BitConverter]::ToUInt16($script:bytes, $script:pos) -ne 0x3B) { throw 'Invalid Registry.pol type separator.' }
    $script:pos += 2
    $s = ReadU32
    if ($script:pos + 2 -gt $script:bytes.Length -or [BitConverter]::ToUInt16($script:bytes, $script:pos) -ne 0x3B) { throw 'Invalid Registry.pol size separator.' }
    $script:pos += 2
    if ($s -gt $script:bytes.Length - $script:pos - 2) { throw 'Invalid Registry.pol data length.' }
    $d = New-Object byte[] $s
    [Array]::Copy($script:bytes, $script:pos, $d, 0, $s); $script:pos += $s
    if ($script:pos + 2 -gt $script:bytes.Length -or [BitConverter]::ToUInt16($script:bytes, $script:pos) -ne 0x5D) { throw 'Invalid Registry.pol entry terminator.' }
    $script:pos += 2
    [void]$script:pol.Add([pscustomobject]@{ Key = $k; Name = $n; Type = $t; Data = $d })
  }
}
function PolGet($k, $n) {
  foreach ($e in $script:pol) { if ($e.Key -ieq $k -and $e.Name -ieq $n) { return $e } }
  $null
}
function PolIs($k, $n, $v) {
  $e = PolGet $k $n
  [bool]($e -and $e.Type -eq 4 -and $e.Data.Length -eq 4 -and [BitConverter]::ToUInt32($e.Data, 0) -eq $v -and -not (PolGet $k "**del.$n"))
}
function PolIsDel($k, $n) { [bool]((PolGet $k "**del.$n") -and -not (PolGet $k $n)) }
function PolNone($k, $n) { [bool](-not (PolGet $k $n) -and -not (PolGet $k "**del.$n")) }
# gpt.ini names the extensions that process the machine policies; without
# the registry one, {35378EAC...}, the Registry.pol is skipped
function PolGptOk {
  $gpt = Join-Path (Split-Path $polDir) 'gpt.ini'
  if (-not (Test-Path -LiteralPath $gpt)) { return $false }
  [bool]((Get-Content -LiteralPath $gpt -Raw) -match 'gPCMachineExtensionNames\s*=.*35378EAC-683F-11D2-A89A-00C04FBBCFA2')
}
function PolDel($k, $n) {
  for ($i = $script:pol.Count - 1; $i -ge 0; $i--) {
    $e = $script:pol[$i]
    if ($e.Key -ieq $k -and ($e.Name -ieq $n -or $e.Name -ieq "**del.$n")) { $script:pol.RemoveAt($i) }
  }
}
function PolSet($k, $n, $v) {
  PolDel $k $n
  [void]$script:pol.Add([pscustomobject]@{ Key = $k; Name = $n; Type = 4; Data = [BitConverter]::GetBytes([uint32]$v) })
}
# PolSetDel writes the order to delete a value, as gpedit does: REG_SZ " "
function PolSetDel($k, $n) {
  PolDel $k $n
  [void]$script:pol.Add([pscustomobject]@{ Key = $k; Name = "**del.$n"; Type = 1; Data = [byte[]](0x20,0,0,0) })
}
function PolSave {
  if (-not (Test-Path -LiteralPath $polDir)) { New-Item -ItemType Directory -Path $polDir -Force | Out-Null }
  $ms = New-Object System.IO.MemoryStream
  $w = New-Object System.IO.BinaryWriter($ms)
  $w.Write([byte[]](0x50,0x52,0x65,0x67)); $w.Write([uint32]1)
  $sep = [byte[]](0x3B,0)
  $nul = [byte[]](0,0)
  foreach ($e in $script:pol) {
    $w.Write([byte[]](0x5B,0))
    $w.Write([Text.Encoding]::Unicode.GetBytes($e.Key)); $w.Write($nul); $w.Write($sep)
    $w.Write([Text.Encoding]::Unicode.GetBytes($e.Name)); $w.Write($nul); $w.Write($sep)
    $w.Write([uint32]$e.Type); $w.Write($sep)
    $w.Write([uint32]$e.Data.Length); $w.Write($sep)
    $w.Write([byte[]]$e.Data)
    $w.Write([byte[]](0x5D,0))
  }
  $w.Flush()
  if (Test-Path -LiteralPath $polFile) { (Get-Item -LiteralPath $polFile -Force).Attributes = 'Normal' }
  [IO.File]::WriteAllBytes($polFile, $ms.ToArray())
  # gpt.ini tells Windows which extensions process the machine policies (the
  # registry one, {35378EAC...}, with the Administrative Templates tool
  # {D02B1F72...}) and, by its version, that they changed. With only
  # "[General]" in it (as gpedit.msc may leave it) the Registry.pol is skipped.
  $gpt = Join-Path (Split-Path $polDir) 'gpt.ini'
  $reg = '[{35378EAC-683F-11D2-A89A-00C04FBBCFA2}{D02B1F72-3407-48AE-BA88-E8213C6761F1}]'
  $lines = @()
  if (Test-Path -LiteralPath $gpt) {
    (Get-Item -LiteralPath $gpt -Force).Attributes = 'Normal'
    $lines = @(Get-Content -LiteralPath $gpt | Where-Object { $_.Trim() })
  }
  $ext = $lines | Where-Object { $_ -match '^\s*gPCMachineExtensionNames\s*=' } | Select-Object -First 1
  $ext = if ($ext) { ($ext -split '=', 2)[1].Trim() } else { '' }
  if ($ext.IndexOf('{35378EAC-683F-11D2-A89A-00C04FBBCFA2}', [StringComparison]::OrdinalIgnoreCase) -lt 0) { $ext = $ext + $reg }
  $ver = [uint32]0
  $old = $lines | Where-Object { $_ -match '^\s*Version\s*=\s*\d+' } | Select-Object -First 1
  if ($old -and $old -match '=\s*(\d+)') { $ver = [uint32]$Matches[1] }
  # the low 16 bits count the changes to the machine policies, the high ones the user's
  $ver = ($ver -band 0xFFFF0000) -bor ((($ver -band 0xFFFF) + 1) -band 0xFFFF)
  $keep = @($lines | Where-Object { $_ -notmatch '^\s*(\[General\]|gPCMachineExtensionNames\s*=|Version\s*=)' })
  $out = @('[General]', ('gPCMachineExtensionNames=' + $ext), ('Version=' + $ver)) + $keep
  [IO.File]::WriteAllText($gpt, (($out -join [Environment]::NewLine) + [Environment]::NewLine), [Text.Encoding]::ASCII)
}
`

// polKey is a registry key as the Registry.pol stores it, without the hive.
func polKey(key string) string { return strings.TrimPrefix(key, `HKLM\`) }

// ApplyScript is a PowerShell script that writes every policy's choice. It
// has to run elevated: the keys are under HKLM and the Registry.pol is under
// System32. Each value goes to both the Registry.pol, so gpedit.msc shows the
// policy as configured and a policy refresh keeps it, and the registry, so it
// takes effect right away. The Registry.pol is always written again, so a
// file in an old format is fixed even when nothing else changed.
func ApplyScript(policies []Policy) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	b.WriteString(polLib)
	b.WriteString("PolLoad\n")
	for _, p := range policies {
		set, remove := p.Wanted()
		for _, v := range set {
			k := regPath(v.Key)
			if v.Delete {
				fmt.Fprintf(&b, "PolSetDel '%s' '%s'\n", polKey(v.Key), v.Name)
				fmt.Fprintf(&b, "if (Test-Path -LiteralPath '%s') { Remove-ItemProperty -LiteralPath '%s' -Name '%s' -ErrorAction SilentlyContinue }\n", k, k, v.Name)
				continue
			}
			fmt.Fprintf(&b, "PolSet '%s' '%s' %d\n", polKey(v.Key), v.Name, v.Value)
			fmt.Fprintf(&b, "if (-not (Test-Path -LiteralPath '%s')) { New-Item -Path '%s' -Force | Out-Null }\n", k, k)
			fmt.Fprintf(&b, "Set-ItemProperty -LiteralPath '%s' -Name '%s' -Value %d -Type DWord\n", k, v.Name, v.Value)
		}
		for _, v := range remove {
			fmt.Fprintf(&b, "PolDel '%s' '%s'\n", polKey(v.Key), v.Name)
			k := regPath(v.Key)
			fmt.Fprintf(&b, "if (Test-Path -LiteralPath '%s') { Remove-ItemProperty -LiteralPath '%s' -Name '%s' -ErrorAction SilentlyContinue }\n", k, k, v.Name)
		}
	}
	b.WriteString("PolSave\n")
	// Refresh now so the values are rewritten from the Registry.pol; a failure
	// here only delays that to the next refresh.
	b.WriteString("try { & gpupdate.exe /target:computer /force | Out-Null } catch { }\n")
	b.WriteString("exit 0\n")
	return b.String()
}

// ElevatedCommand is the PowerShell that runs a script file as
// administrator (one UAC prompt) and passes its exit code on.
func ElevatedCommand(scriptPath string) string {
	quoted := strings.ReplaceAll(scriptPath, "'", "''")
	return "$p = Start-Process -FilePath 'powershell.exe' -Verb RunAs -Wait -PassThru -WindowStyle Hidden " +
		"-ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','\"" + quoted + "\"'; exit $p.ExitCode"
}

// SetChoice changes the `choice` of one policy in the file. Only that line
// is rewritten, so comments and layout stay as they are.
func SetChoice(path, id, choice string) error {
	f, err := Load(path)
	if err != nil {
		return err
	}
	found := false
	for _, p := range f.Policies {
		if p.ID == id {
			found = true
			if !p.valid(choice) {
				return fmt.Errorf("policy %q has no choice %q", id, choice)
			}
		}
	}
	if !found {
		return fmt.Errorf("unknown policy %q", id)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	var list *yaml.Node
	top := doc.Content[0]
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "policies" {
			list = top.Content[i+1]
		}
	}
	for _, n := range list.Content {
		var key, val *yaml.Node
		match := false
		for i := 0; i+1 < len(n.Content); i += 2 {
			switch n.Content[i].Value {
			case "id":
				match = n.Content[i+1].Value == id
			case "choice":
				key, val = n.Content[i], n.Content[i+1]
			}
		}
		if !match {
			continue
		}
		if key == nil || val.Kind != yaml.ScalarNode || key.Line != val.Line || key.Line < 1 || key.Line > len(lines) {
			return fmt.Errorf("%s: policy %s needs a `choice:` line to be edited", path, id)
		}
		line := lines[key.Line-1]
		end := ""
		if strings.HasSuffix(line, "\r") {
			end = "\r"
		}
		lines[key.Line-1] = line[:key.Column-1] + "choice: " + choice + end
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, path)
	}
	return fmt.Errorf("unknown policy %q", id)
}

// Key is the registry key the policy writes: of its own values, else of its
// first option.
func (p Policy) Key() string {
	if len(p.Set) > 0 {
		return p.Set[0].Key
	}
	for _, o := range p.Options {
		if len(o.Set) > 0 {
			return o.Set[0].Key
		}
	}
	return ""
}
