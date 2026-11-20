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
  # a partial match is by whole words: "Dia" must not take "Windows Media Player"
  foreach ($n in $names) { $re = '(?i)(^|[^\p{L}\p{N}])' + [regex]::Escape($n) + '($|[^\p{L}\p{N}])'; $m = $links | Where-Object { $_.BaseName -match $re } | Select-Object -First 1; if ($m) { return $m.FullName } }
  $null
}
$esc = { param($s) [System.Security.SecurityElement]::Escape($s) }
$pins = New-Object System.Collections.ArrayList
$seen = @{}
# What each pin starts, for the sign-in guard to tell whether the pins are there.
$expected = New-Object System.Collections.ArrayList
$startApps = @(try { Get-StartApps } catch { })
$wsh = New-Object -ComObject WScript.Shell
function PinId($name, $id) {
  [void]$pins.Add('        <taskbar:DesktopApp DesktopApplicationID="' + (& $esc $id) + '" />')
  Write-Output ('pinned: ' + $name)
}
# How a shortcut goes in the layout: by its path, written with %APPDATA% or
# %ALLUSERSPROFILE% as the layout format expects. Windows re-reads the layout
# at sign-in; there a shortcut written as a plain C:\Users\... path was
# dropped, and so were the apps given by their app id (Brave,
# com.squirrel.Discord.Discord), while the shortcuts written with the
# variables stayed.
function PinXml($p) {
  $v = $p
  foreach ($pair in @(@($env:APPDATA, '%APPDATA%'), @($env:ProgramData, '%ALLUSERSPROFILE%'))) {
    if ($pair[0] -and $p.StartsWith($pair[0] + '\', [StringComparison]::OrdinalIgnoreCase)) { $v = $pair[1] + $p.Substring($pair[0].Length); break }
  }
  '        <taskbar:DesktopApp DesktopApplicationLinkPath="' + (& $esc $v) + '" />'
}
function PinLink($name, $names) {
  # the names in order, each as a shortcut or as an MSIX app (Dia without the
  # shortcut windows\msix-shortcut.ps1 makes is pinned by its app id); then
  # a partial shortcut match
  $p = $null; $uwa = $null
  foreach ($n in $names) {
    $m = $links | Where-Object { $_.BaseName -ieq $n } | Select-Object -First 1
    if ($m) { $p = $m.FullName; break }
    $uwa = $startApps | Where-Object { $_.Name -ieq $n -and $_.AppID -match '!' } | Select-Object -First 1
    if ($uwa) { break }
  }
  if (-not $p -and -not $uwa) { $p = FindLink $names }
  if (-not $p) {
    if (-not $uwa) { Write-Output ('not found in the Start menu, left out: ' + $name + ' (install it first)'); return }
    if ($seen[$uwa.AppID]) { Write-Output ('already pinned, left out: ' + $name + ' (' + $uwa.Name + ')'); return }
    $seen[$uwa.AppID] = $true
    [void]$pins.Add('        <taskbar:UWA AppUserModelID="' + (& $esc $uwa.AppID) + '" />')
    [void]$expected.Add($uwa.AppID + '|' + $uwa.Name + '|' + $uwa.AppID)
    Write-Output ('pinned: ' + $name + ' (' + $uwa.Name + ', store app)')
    return
  }
  if ($seen[$p]) { Write-Output ('already pinned, left out: ' + $name + ' (' + [IO.Path]::GetFileNameWithoutExtension($p) + ')'); return }
  $seen[$p] = $true
  [void]$pins.Add((PinXml $p))
  # target|shortcut name|app id: any of them in the taskbar's list tells the pin is there
  $t = $wsh.CreateShortcut($p).TargetPath
  $base = [IO.Path]::GetFileNameWithoutExtension($p)
  $app = $startApps | Where-Object { $_.Name -ieq $base -and $_.AppID -and $_.AppID -notmatch '[\\/:]' } | Select-Object -First 1
  $aumid = if ($app) { $app.AppID } else { '' }
  if ($t) { [void]$expected.Add($t + '|' + [IO.Path]::GetFileName($p) + '|' + $aumid) }
  Write-Output ('pinned: ' + $name + ' (' + $base + ')')
}
# The default browser (the program that opens https links) comes first; Edge,
# or no choice yet, falls back to the first browser of the list installed.
function PinBrowser($name, $names) {
  $prog = (Get-ItemProperty -LiteralPath 'Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\https\UserChoice' -ErrorAction SilentlyContinue).ProgId
  $byProg = [ordered]@{ 'BraveHTML' = 'Brave'; 'ChromeHTML' = 'Google Chrome'; 'FirefoxURL' = 'Firefox'; 'Opera GX' = 'Opera GX'; 'OperaGX' = 'Opera GX'; 'OperaStable' = 'Opera'; 'VivaldiHTM' = 'Vivaldi' }
  $first = @()
  foreach ($k in $byProg.Keys) { if ($prog -and $prog.StartsWith($k, [StringComparison]::OrdinalIgnoreCase)) { $first += $byProg[$k] } }
  # an MSIX browser (Dia) has an AppX... ProgId: its app id tells which one
  if ($prog -and -not $first) {
    $aumid = (Get-ItemProperty -LiteralPath ('Registry::HKEY_CLASSES_ROOT\' + $prog + '\Application') -ErrorAction SilentlyContinue).AppUserModelID
    if ($aumid -match '^TheBrowserCompany\.Dia_') { $first += 'Dia' }
  }
  PinLink $name (@($first) + @($names))
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
		fn := "PinLink"
		if it.Browser {
			fn = "PinBrowser"
		}
		fmt.Fprintf(&b, "%s %s @(%s)\n", fn, psQuote(it.Name), strings.Join(names, ", "))
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
[IO.File]::WriteAllText((Join-Path $dir ` + psQuote(PinsFile) + `), (($expected -join [Environment]::NewLine) + [Environment]::NewLine), (New-Object System.Text.UTF8Encoding $false))
$guard = @'
` + GuardScript + `'@
[IO.File]::WriteAllText((Join-Path $dir ` + psQuote(GuardFile) + `), $guard, (New-Object System.Text.UTF8Encoding $false))
`)
	return b.String()
}

// The sign-in guard: what the pins start (one target per line) and the
// script that checks them, next to the layout file.
const (
	PinsFile  = `taskbar-pins.txt`
	GuardFile = `taskbar-guard.ps1`
	// RunValue is the HKCU\...\Run value that starts the guard at sign-in.
	RunValue = `envsetup-taskbar`
)

// GuardScript runs at every sign-in of the user who applied the layout. When
// the taskbar comes up with most of the pins missing (Windows re-reading the
// layout at sign-in and failing to resolve them), it hands Windows the layout
// again: newer file, Taskband reset, Explorer restarted. A pin the user took
// off by hand stays off, since most of the others are still there. If the
// repair does not bring the pins back it is not tried again for this layout,
// so the shell is never restarted at every sign-in for nothing.
const GuardScript = `# envsetup: puts the taskbar pins back when a sign-in leaves the taskbar
# (almost) empty. Written by envsetup apply-layout, started at sign-in by the
# HKCU Run value "` + RunValue + `". Its log is %LOCALAPPDATA%\envsetup\taskbar-guard.log.
$ErrorActionPreference = 'SilentlyContinue'
$dir = Join-Path $env:ProgramData '` + XMLDir + `'
$xml = Join-Path $dir '` + XMLFile + `'
$list = Join-Path $dir '` + PinsFile + `'
if (-not (Test-Path -LiteralPath $xml) -or -not (Test-Path -LiteralPath $list)) { return }
$want = @(Get-Content -LiteralPath $list | Where-Object { $_.Trim() })
if ($want.Count -eq 0) { return }
$stateDir = Join-Path $env:LOCALAPPDATA 'envsetup'
$state = Join-Path $stateDir 'taskbar-guard.txt'
$log = Join-Path $stateDir 'taskbar-guard.log'
New-Item -ItemType Directory -Force -Path $stateDir | Out-Null
function Log($m) { Add-Content -LiteralPath $log -Value ((Get-Date -Format 's') + ' ' + $m) }
$sum = (Get-FileHash -LiteralPath $list -Algorithm SHA256).Hash
if ((Get-Content -LiteralPath $state -Raw) -match $sum) { return }
$tb = 'Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Explorer\Taskband'
$pinned = Join-Path $env:APPDATA 'Microsoft\Internet Explorer\Quick Launch\User Pinned'
# Each line is target|shortcut name|app id. A pin is there when the taskbar's
# own list (Taskband\Favorites) names it in any of the ways Windows stores a
# pin: a pinned shortcut (long or 8.3 name) that starts the target, the
# target itself, the Start menu shortcut, or the app id.
function Get-Missing {
  $fav = (Get-ItemProperty -LiteralPath $tb).Favorites
  if (-not $fav) { return @($want | ForEach-Object { ($_ -split '\|')[0] }) }
  # names are UTF-16 starting at even or odd offsets, 8.3 names are ASCII
  $text = [Text.Encoding]::Unicode.GetString($fav) + ' ' + [Text.Encoding]::Unicode.GetString($fav, 1, $fav.Length - 1) + ' ' + [Text.Encoding]::ASCII.GetString($fav)
  $sh = New-Object -ComObject WScript.Shell
  $fso = New-Object -ComObject Scripting.FileSystemObject
  $have = @{}
  foreach ($l in @(Get-ChildItem -LiteralPath $pinned -Recurse -Filter *.lnk)) {
    $short = $fso.GetFile($l.FullName).ShortName
    if ($text.IndexOf($l.Name, [StringComparison]::OrdinalIgnoreCase) -lt 0 -and (-not $short -or $text.IndexOf($short, [StringComparison]::OrdinalIgnoreCase) -lt 0)) { continue }
    $t = $sh.CreateShortcut($l.FullName).TargetPath
    if ($t) { $have[$t.ToLowerInvariant()] = $true }
  }
  foreach ($w in $want) {
    $f = $w -split '\|'
    $target = $f[0]
    $there = $have[$target.ToLowerInvariant()] -or $text.IndexOf($target, [StringComparison]::OrdinalIgnoreCase) -ge 0
    if (-not $there -and $f.Count -gt 1 -and $f[1]) { $there = $text.IndexOf($f[1], [StringComparison]::OrdinalIgnoreCase) -ge 0 }
    if (-not $there -and $f.Count -gt 2 -and $f[2]) { $there = $text -match ('(?i)(^|[^A-Za-z0-9.])' + [regex]::Escape($f[2]) + '($|[^A-Za-z0-9.])') }
    if (-not $there) { $target }
  }
}
for ($i = 0; $i -lt 120 -and -not (Get-Process -Name explorer); $i++) { Start-Sleep -Seconds 1 }
Start-Sleep -Seconds 20
$miss = @(Get-Missing)
if ($miss.Count * 2 -le $want.Count) { return }
Log ('pins missing after sign-in: ' + ($miss -join '; ') + ' - applying the layout again')
(Get-Item -LiteralPath $xml).LastWriteTime = Get-Date
Remove-Item -LiteralPath $tb -Recurse -Force
Stop-Process -Name explorer -Force
Start-Sleep -Seconds 30
$miss = @(Get-Missing)
if ($miss.Count * 2 -gt $want.Count) {
  Log ('still missing: ' + ($miss -join '; ') + ' - not trying again for this layout')
  Set-Content -LiteralPath $state -Value $sum
} else {
  Log 'pins are back'
}
`

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
	// "Automatically hide the taskbar" is byte 8 of StuckRects3: 3 hides, 2 shows.
	// Explorer reads it when it starts, which the end of this script makes it do.
	autohide := 2
	if c.TaskbarAutoHide {
		autohide = 3
	}
	fmt.Fprintf(&b, `$sr = 'Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Explorer\StuckRects3'
$v = (Get-ItemProperty -LiteralPath $sr -ErrorAction SilentlyContinue).Settings
if ($v -and $v.Length -gt 8) { $v[8] = %d; Set-ItemProperty -LiteralPath $sr -Name 'Settings' -Value ([byte[]]$v) -Type Binary }
`, autohide)
	b.WriteString("$run = 'Registry::HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Run'\n")
	if len(Pinned(c.Taskbar)) > 0 {
		b.WriteString("Remove-Item -LiteralPath 'Registry::HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Taskband' -Recurse -Force -ErrorAction SilentlyContinue\n")
		// the guard checks the pins at every sign-in; a new layout gets its
		// chances to repair again
		fmt.Fprintf(&b, `$guard = Join-Path $env:ProgramData %s
Set-ItemProperty -LiteralPath $run -Name %s -Value ('powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + $guard + '"')
Remove-Item -LiteralPath (Join-Path $env:LOCALAPPDATA 'envsetup\taskbar-guard.txt') -Force -ErrorAction SilentlyContinue
`, psQuote(XMLDir+`\`+GuardFile), psQuote(RunValue))
	} else {
		fmt.Fprintf(&b, "Remove-ItemProperty -LiteralPath $run -Name %s -ErrorAction SilentlyContinue\n", psQuote(RunValue))
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
