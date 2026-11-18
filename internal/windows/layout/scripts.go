package layout

import (
	"fmt"
	"strings"
)

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// XMLDir and XMLFile are where the taskbar layout file is written, under
// %ProgramData%: the policy that points at it is read at sign-in, by
// whichever user signs in.
const (
	XMLDir  = `envsetup`
	XMLFile = `TaskbarLayout.xml`
)

// Pinned resolves the saved ids to the items that can be pinned, in order.
func Pinned(ids []string) []Item {
	var out []Item
	for _, id := range ids {
		if it, ok := Find(id); ok && it.Pinnable {
			out = append(out, it)
		}
	}
	return out
}

// PrepareScript runs as the user: it finds the Start menu shortcut of every
// pinned app and writes the taskbar layout file. An app that is not installed
// yet is reported and left out; if none is found nothing is written, since a
// layout with no apps would empty the taskbar.
func PrepareScript(c Config) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	b.WriteString("$dir = Join-Path $env:ProgramData " + psQuote(XMLDir) + "\n")
	b.WriteString("New-Item -ItemType Directory -Force -Path $dir | Out-Null\n")
	b.WriteString(`$roots = @((Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs'), (Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs'))
$links = @(foreach ($r in $roots) { if (Test-Path -LiteralPath $r) { Get-ChildItem -LiteralPath $r -Recurse -Filter *.lnk -ErrorAction SilentlyContinue } })
function FindLink($names) {
  foreach ($n in $names) { $m = $links | Where-Object { $_.BaseName -ieq $n } | Select-Object -First 1; if ($m) { return $m.FullName } }
  foreach ($n in $names) { $m = $links | Where-Object { $_.BaseName.IndexOf($n, [StringComparison]::OrdinalIgnoreCase) -ge 0 } | Select-Object -First 1; if ($m) { return $m.FullName } }
  $null
}
$esc = { param($s) [System.Security.SecurityElement]::Escape($s) }
$pins = New-Object System.Collections.ArrayList
function PinId($name, $id) {
  [void]$pins.Add('        <taskbar:DesktopApp DesktopApplicationID="' + (& $esc $id) + '" />')
  Write-Output ('pinned: ' + $name)
}
function PinLink($name, $names) {
  $p = FindLink $names
  if ($p) {
    [void]$pins.Add('        <taskbar:DesktopApp DesktopApplicationLinkPath="' + (& $esc $p) + '" />')
    Write-Output ('pinned: ' + $name)
  } else {
    Write-Output ('not found in the Start menu, left out: ' + $name + ' (install it first)')
  }
}
`)
	for _, it := range Pinned(c.Taskbar) {
		if it.AUMID != "" {
			fmt.Fprintf(&b, "PinId %s %s\n", psQuote(it.Name), psQuote(it.AUMID))
			continue
		}
		names := make([]string, len(it.Match))
		for i, m := range it.Match {
			names[i] = psQuote(m)
		}
		fmt.Fprintf(&b, "PinLink %s @(%s)\n", psQuote(it.Name), strings.Join(names, ", "))
	}
	b.WriteString(`if ($pins.Count -eq 0) { throw 'none of the chosen apps was found in the Start menu: install them first, then apply again' }
$xml = @(
  '<?xml version="1.0" encoding="utf-8"?>',
  '<LayoutModificationTemplate xmlns="http://schemas.microsoft.com/Start/2014/LayoutModification" xmlns:defaultlayout="http://schemas.microsoft.com/Start/2014/FullDefaultLayout" xmlns:start="http://schemas.microsoft.com/Start/2014/StartLayout" xmlns:taskbar="http://schemas.microsoft.com/Start/2014/TaskbarLayout" Version="1">',
  '  <CustomTaskbarLayoutCollection PinListPlacement="Replace">',
  '    <defaultlayout:TaskbarLayout>',
  '      <taskbar:TaskbarPinList>'
) + $pins + @(
  '      </taskbar:TaskbarPinList>',
  '    </defaultlayout:TaskbarLayout>',
  '  </CustomTaskbarLayoutCollection>',
  '</LayoutModificationTemplate>'
)
[IO.File]::WriteAllText((Join-Path $dir ` + psQuote(XMLFile) + `), (($xml -join [Environment]::NewLine) + [Environment]::NewLine), (New-Object System.Text.UTF8Encoding $false))
`)
	return b.String()
}

// AdminScript runs elevated: the policies it sets are under HKLM. The
// taskbar layout is handed to Windows through the "Start layout" policy, the
// Start menu pins through the policy the answer file's SetStartPins script
// uses.
func AdminScript(c Config) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	b.WriteString(`$ex = 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Microsoft\Windows\Explorer'
$st = 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\PolicyManager\current\device\Start'
`)
	if len(Pinned(c.Taskbar)) > 0 {
		b.WriteString(`if (-not (Test-Path -LiteralPath $ex)) { New-Item -Path $ex -Force | Out-Null }
$layout = Join-Path $env:ProgramData ` + psQuote(XMLDir+`\`+XMLFile) + `
New-ItemProperty -LiteralPath $ex -Name 'StartLayoutFile' -Value $layout -PropertyType ExpandString -Force | Out-Null
New-ItemProperty -LiteralPath $ex -Name 'LockedStartLayout' -Value 0 -PropertyType DWord -Force | Out-Null
`)
	} else {
		b.WriteString(`if (Test-Path -LiteralPath $ex) {
  Remove-ItemProperty -LiteralPath $ex -Name 'StartLayoutFile' -ErrorAction SilentlyContinue
  Remove-ItemProperty -LiteralPath $ex -Name 'LockedStartLayout' -ErrorAction SilentlyContinue
}
`)
	}
	if c.ClearStart {
		b.WriteString(`if (-not (Test-Path -LiteralPath $st)) { New-Item -Path $st -Force | Out-Null }
Set-ItemProperty -LiteralPath $st -Name 'ConfigureStartPins' -Value '{"pinnedList":[]}' -Type String
`)
	} else {
		b.WriteString(`if (Test-Path -LiteralPath $st) { Remove-ItemProperty -LiteralPath $st -Name 'ConfigureStartPins' -ErrorAction SilentlyContinue }
`)
	}
	b.WriteString("exit 0\n")
	return b.String()
}

// FinishScript runs as the user, after the policies are in place: it hides
// or shows the desktop icons, resets what the shell remembers about the
// taskbar and the Start menu so the new layout is read, and restarts the
// shell (Windows starts it again by itself).
func FinishScript(c Config) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	hide := 0
	if c.HideDesktopIcons {
		hide = 1
	}
	fmt.Fprintf(&b, "Set-ItemProperty -LiteralPath 'Registry::HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced' -Name 'HideIcons' -Value %d -Type DWord\n", hide)
	if len(Pinned(c.Taskbar)) > 0 {
		b.WriteString("Remove-Item -LiteralPath 'Registry::HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Taskband' -Recurse -Force -ErrorAction SilentlyContinue\n")
	}
	if c.ClearStart {
		b.WriteString(`$state = Join-Path $env:LOCALAPPDATA 'Packages\Microsoft.Windows.StartMenuExperienceHost_cw5n1h2txyewy\LocalState'
if (Test-Path -LiteralPath $state) { Get-ChildItem -LiteralPath $state -Filter 'start*.bin' -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue }
Stop-Process -Name 'StartMenuExperienceHost' -Force -ErrorAction SilentlyContinue
`)
	}
	b.WriteString("Stop-Process -Name 'explorer' -Force -ErrorAction SilentlyContinue\n")
	return b.String()
}
