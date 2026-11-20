package layout

import (
	"fmt"
	"strings"
)

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// What earlier versions left on the machine: the taskbar was handed to
// Windows as a layout file through the "Start layout" policy, and a guard
// started at sign-in put it back. Windows re-read that file at every sign-in
// and dropped the pins it could not resolve (most of them, on recent builds),
// leaving copies such as "Brave (2).lnk" behind each time. The layout step now
// removes all of it.
const (
	legacyDir      = `envsetup`
	legacyXML      = `TaskbarLayout.xml`
	legacyPins     = `taskbar-pins.txt`
	legacyGuard    = `taskbar-guard.ps1`
	legacyRunValue = `envsetup-taskbar`
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

// pinHelper reads the taskbar's own list (to check the pins after Explorer
// restarts) and turns a path into the shell's item id list.
const pinHelper = `Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
public static class EnvsetupTaskbar {
  [ComImport, Guid("0DD79AE2-D156-45D4-9EEB-3B549769E940"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
  interface IPinnedList3 { [PreserveSig] int EnumObjects(out IEnumFullIDList e); }
  [ComImport, Guid("D0191542-7954-4908-BC06-B2360BBE45BA"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
  interface IEnumFullIDList { [PreserveSig] int Next(uint n, out IntPtr pidl, out uint got); }
  [DllImport("shell32.dll", CharSet = CharSet.Unicode)] static extern int SHParseDisplayName(string name, IntPtr bc, out IntPtr pidl, uint sfgao, out uint attrs);
  [DllImport("shell32.dll", CharSet = CharSet.Unicode)] static extern int SHGetNameFromIDList(IntPtr pidl, uint sigdn, out IntPtr name);
  [DllImport("shell32.dll")] static extern uint ILGetSize(IntPtr pidl);
  public static byte[] IdList(string path) {
    IntPtr p; uint a;
    int hr = SHParseDisplayName(path, IntPtr.Zero, out p, 0, out a);
    if (hr != 0) throw new COMException(path, hr);
    int n = (int)ILGetSize(p); byte[] b = new byte[n]; Marshal.Copy(p, b, 0, n); Marshal.FreeCoTaskMem(p);
    return b;
  }
  public static string[] Pinned() {
    Type t = Type.GetTypeFromCLSID(new Guid("90AA3A4E-1CBA-4233-B8BB-535773D48449"));
    IPinnedList3 list = (IPinnedList3)Activator.CreateInstance(t);
    IEnumFullIDList e; List<string> o = new List<string>();
    if (list.EnumObjects(out e) != 0) return o.ToArray();
    IntPtr p; uint got;
    while (e.Next(1, out p, out got) == 0 && got == 1) {
      IntPtr s;
      if (SHGetNameFromIDList(p, 0x80058000, out s) == 0) { o.Add(Marshal.PtrToStringUni(s)); Marshal.FreeCoTaskMem(s); }
      Marshal.FreeCoTaskMem(p);
    }
    return o.ToArray();
  }
}
'@
`

// pinFunctions find the Start menu shortcut of each chosen app. A pin is a
// copy of that shortcut, so it carries the icon the icons step gave it.
const pinFunctions = `$roots = @((Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs'), (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs'))
$links = @(foreach ($r in $roots) { if (Test-Path -LiteralPath $r) { Get-ChildItem -LiteralPath $r -Recurse -Filter *.lnk -ErrorAction SilentlyContinue } })
$startApps = @(try { Get-StartApps } catch { })
$shell = New-Object -ComObject Shell.Application
$wsh = New-Object -ComObject WScript.Shell
function LinkProp($p, $name) {
  try { $shell.NameSpace([IO.Path]::GetDirectoryName($p)).ParseName([IO.Path]::GetFileName($p)).ExtendedProperty($name) } catch { $null }
}
function FindLink($names) {
  foreach ($n in $names) { $m = $links | Where-Object { $_.BaseName -ieq $n } | Select-Object -First 1; if ($m) { return $m.FullName } }
  # a partial match is by whole words: "Dia" must not take "Windows Media Player"
  foreach ($n in $names) { $re = '(?i)(^|[^\p{L}\p{N}])' + [regex]::Escape($n) + '($|[^\p{L}\p{N}])'; $m = $links | Where-Object { $_.BaseName -match $re } | Select-Object -First 1; if ($m) { return $m.FullName } }
  $null
}
# The app id the taskbar files a pin under: the shortcut's own, else the one
# Windows gives the program (what Get-StartApps lists), else the target with
# the known folder written as its id, the way Windows writes it.
function AppId($p) {
  $id = LinkProp $p 'System.AppUserModel.ID'
  if ($id) { return [string]$id }
  $base = [IO.Path]::GetFileNameWithoutExtension($p)
  $a = $startApps | Where-Object { $_.Name -ieq $base -and $_.AppID -notmatch '!' } | Select-Object -First 1
  if ($a) { return [string]$a.AppID }
  $t = $wsh.CreateShortcut($p).TargetPath
  if (-not $t) { return '' }
  $known = @(
    @([Environment]::GetFolderPath('System'), '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}'),
    @([Environment]::GetFolderPath('ProgramFilesX86'), '{7C5A40EF-A0FB-4BFC-874A-C0F2E0B9FA8E}'),
    @($env:ProgramW6432, '{6D809377-6AF0-444B-8957-A3773F02200E}'),
    @([Environment]::GetFolderPath('Windows'), '{F38BF404-1D43-42F2-9305-67DE0B28FC23}'))
  foreach ($k in $known) { if ($k[0] -and $t.StartsWith($k[0] + '\', [StringComparison]::OrdinalIgnoreCase)) { return $k[1] + $t.Substring($k[0].Length) } }
  $t
}
$pins = New-Object System.Collections.ArrayList
$seen = @{}
function AddPin($name, $p) {
  $id = AppId $p
  $key = if ($id) { $id.ToLowerInvariant() } else { $p.ToLowerInvariant() }
  if ($seen[$key]) { Write-Output ('already pinned, left out: ' + $name + ' (' + [IO.Path]::GetFileNameWithoutExtension($p) + ')'); return }
  $seen[$key] = $true
  [void]$pins.Add([pscustomobject]@{ Name = $name; Link = $p; Id = $id })
}
function PinLink($name, $names) {
  $p = FindLink $names
  if ($p) { AddPin $name $p; return }
  $uwa = $startApps | Where-Object { $names -icontains $_.Name -and $_.AppID -match '!' } | Select-Object -First 1
  if ($uwa) { Write-Output ('a store app with no Start menu shortcut, left out: ' + $name + ' (' + $uwa.Name + ')'); return }
  Write-Output ('not found in the Start menu, left out: ' + $name + ' (install it first)')
}
# Windows' own apps (File Explorer, Edge): their Start menu shortcut, by name
# or, in another language, by the app id it carries.
function PinId($name, $id, $names) {
  $p = FindLink $names
  if (-not $p) { $m = $links | Where-Object { (LinkProp $_.FullName 'System.AppUserModel.ID') -eq $id } | Select-Object -First 1; if ($m) { $p = $m.FullName } }
  if ($p) { AddPin $name $p } else { Write-Output ('no Start menu shortcut, left out: ' + $name) }
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
`

// pinWrite replaces the taskbar's pins with $pins, the way Explorer keeps
// them: a shortcut in User Pinned\TaskBar for each, and the list in
// HKCU\...\Taskband (Favorites: the item ids of those shortcuts, in order;
// FavoritesResolve: a link to each, to find it again). They are the user's
// own pins: no policy reapplies them at sign-in, they stay until unpinned.
//
// Each Favorites record is 0x00, the length, then an item id list under the
// "User Pinned" folder ({1f3427c8-...}): the TaskBar folder and the shortcut,
// whose item carries three more blocks Explorer checks (a time, the app id
// and "UserPinned"). A record Explorer does not accept makes it drop that pin
// and every one after it, so all of them are written the way it writes its own.
const pinWrite = `$dir = Join-Path $env:APPDATA 'Microsoft\Internet Explorer\Quick Launch\User Pinned\TaskBar'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
# the shortcuts there now go: the pins being replaced and the copies earlier
# layouts left behind ("Brave (2).lnk", "File Explorer (6).lnk")
Get-ChildItem -LiteralPath $dir -Filter *.lnk -Force | Remove-Item -Force
# give the shell a moment to see them go before its list is rewritten
Start-Sleep -Seconds 2
$ini = Join-Path $dir 'desktop.ini'
$names = New-Object System.Collections.ArrayList
$used = @{}
foreach ($p in $pins) {
  $base = [IO.Path]::GetFileNameWithoutExtension($p.Link); $file = $base + '.lnk'; $n = 2
  while ($used[$file.ToLowerInvariant()]) { $file = $base + ' (' + $n + ').lnk'; $n++ }
  $used[$file.ToLowerInvariant()] = $true
  $p | Add-Member -NotePropertyName Pinned -NotePropertyValue (Join-Path $dir $file)
  Copy-Item -LiteralPath $p.Link -Destination $p.Pinned -Force
  # the name Windows shows for it ("File Explorer.lnk" is "Explorador de Arquivos")
  $src = Join-Path ([IO.Path]::GetDirectoryName($p.Link)) 'desktop.ini'
  # (the all-users Start menu's desktop.ini cannot be read by a user)
  $line = try { Get-Content -LiteralPath $src -Force -ErrorAction Stop | Where-Object { $_ -like ([IO.Path]::GetFileName($p.Link) + '=*') } | Select-Object -First 1 } catch { $null }
  if ($line) { [void]$names.Add($file + $line.Substring($line.IndexOf('='))) }
}
if (Test-Path -LiteralPath $ini) { (Get-Item -LiteralPath $ini -Force).Attributes = 'Normal'; Remove-Item -LiteralPath $ini -Force }
if ($names.Count -gt 0) {
  [IO.File]::WriteAllText($ini, (((@('[LocalizedFileNames]') + $names) -join [Environment]::NewLine) + [Environment]::NewLine))
  (Get-Item -LiteralPath $ini -Force).Attributes = 'Hidden, System'
}
$now = [BitConverter]::GetBytes([DateTime]::UtcNow.ToFileTimeUtc())
$root = [byte[]](@(0x3a, 0x00, 0x1f, 0x80) + (New-Object Guid '1f3427c8-5c10-4210-aa03-2ee45287d668').ToByteArray() + @(0x26, 0x00, 0x01, 0x00, 0x26, 0x00, 0xef, 0xbe, 0x12, 0x00, 0x00, 0x00) + $now + $now + $now + @(0x14, 0x00))
function Block($sig, $text, $back) {
  $s = [Text.Encoding]::Unicode.GetBytes($text + [char]0)
  [byte[]]([BitConverter]::GetBytes([uint16](12 + $s.Length)) + @(0x00, 0x00, $sig, 0x00, 0xef, 0xbe, 0x02, 0x00) + $s + $back)
}
$fav = New-Object System.Collections.Generic.List[byte]
$res = New-Object System.Collections.Generic.List[byte]
foreach ($p in $pins) {
  $abs = [EnvsetupTaskbar]::IdList($p.Pinned)
  $items = @(); $i = 0
  while ($true) { $cb = [BitConverter]::ToUInt16($abs, $i); if ($cb -eq 0) { break }; $items += , @($i, $cb); $i += $cb }
  $f = $items[-2]; $folder = [byte[]]$abs[$f[0]..($f[0] + $f[1] - 1)]
  $f = $items[-1]; $item = [byte[]]$abs[$f[0]..($f[0] + $f[1] - 1)]
  # every extension block ends with the offset of the first one
  $back = [byte[]]$item[($item.Length - 2)..($item.Length - 1)]
  $ext = @(0x12, 0x00, 0x00, 0x00, 0x2b, 0x00, 0xef, 0xbe) + $now + $back
  if ($p.Id) { $ext += Block 0x1d $p.Id $back }
  $ext += Block 0x1e 'UserPinned' $back
  $item = [byte[]]($item + $ext)
  $len = [BitConverter]::GetBytes([uint16]$item.Length); $item[0] = $len[0]; $item[1] = $len[1]
  $rel = [byte[]]($root + $folder + $item + @(0x00, 0x00))
  $fav.Add(0); $fav.AddRange([BitConverter]::GetBytes([uint32]$rel.Length)); $fav.AddRange($rel)
  $tmp = Join-Path $env:TEMP ('envsetup-pin-' + [guid]::NewGuid().ToString('N') + '.lnk')
  $l = $wsh.CreateShortcut($tmp); $l.TargetPath = $p.Pinned; $l.Save()
  $raw = [IO.File]::ReadAllBytes($tmp); Remove-Item -LiteralPath $tmp -Force
  $res.AddRange([BitConverter]::GetBytes([uint32]$raw.Length)); $res.AddRange($raw)
}
$fav.Add(0xff)
$tb = 'Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Explorer\Taskband'
function WritePins {
  if (-not (Test-Path -LiteralPath $tb)) { New-Item -Path $tb -Force | Out-Null }
  Set-ItemProperty -LiteralPath $tb -Name 'Favorites' -Value ([byte[]]$fav.ToArray()) -Type Binary
  Set-ItemProperty -LiteralPath $tb -Name 'FavoritesResolve' -Value ([byte[]]$res.ToArray()) -Type Binary
  # Explorer reads the list when it starts; it starts again by itself
  Stop-Process -Name 'explorer' -Force -ErrorAction SilentlyContinue
  for ($i = 0; $i -lt 30; $i++) { Start-Sleep -Seconds 1; if (Get-Process -Name 'explorer' -ErrorAction SilentlyContinue) { break } }
  Start-Sleep -Seconds 4
}
function Missing {
  $have = @{}
  foreach ($x in [EnvsetupTaskbar]::Pinned()) { $have[$x.ToLowerInvariant()] = $true }
  @($pins | Where-Object { -not $have[$_.Pinned.ToLowerInvariant()] })
}
WritePins
$miss = @(Missing)
# a change the shell was still making to its list can win the race: once more
if ($miss.Count -gt 0) { WritePins; $miss = @(Missing) }
foreach ($p in $pins) {
  if ($miss | Where-Object { $_.Pinned -eq $p.Pinned }) { Write-Output ('not on the taskbar after Explorer restarted: ' + $p.Name) }
  else { Write-Output ('pinned: ' + $p.Name + ' (' + [IO.Path]::GetFileNameWithoutExtension($p.Pinned) + ')') }
}
if ($miss.Count -eq $pins.Count) { throw 'Explorer did not take the pins' }
`

// AdminScript runs elevated: the policies it sets are under HKLM. It takes
// out the "Start layout" policy earlier versions pointed at their taskbar
// file (Windows re-read it at every sign-in and dropped the pins), and the
// Start menu pins go through the policy the answer file's SetStartPins
// script uses.
func AdminScript(c Config) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	b.WriteString(`$ex = 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Microsoft\Windows\Explorer'
$st = 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\PolicyManager\current\device\Start'
$old = (Get-ItemProperty -LiteralPath $ex -ErrorAction SilentlyContinue).StartLayoutFile
if ($old -and $old -like ('*\' + ` + psQuote(legacyDir+`\`+legacyXML) + `)) {
  Remove-ItemProperty -LiteralPath $ex -Name 'StartLayoutFile' -ErrorAction SilentlyContinue
  Remove-ItemProperty -LiteralPath $ex -Name 'LockedStartLayout' -ErrorAction SilentlyContinue
}
`)
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
// or shows the desktop icons, clears what earlier versions left for the
// taskbar, resets the Start menu state when its pins are cleared, pins the
// apps and restarts the shell (Windows starts it again by itself). With
// pins, the restart is the pin step's, which then checks the taskbar took
// every one.
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
	fmt.Fprintf(&b, `Remove-ItemProperty -LiteralPath 'Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run' -Name %s -ErrorAction SilentlyContinue
foreach ($f in @(%s, %s, %s)) { Remove-Item -LiteralPath (Join-Path (Join-Path $env:ProgramData %s) $f) -Force -ErrorAction SilentlyContinue }
foreach ($f in @('taskbar-guard.txt', 'taskbar-guard.log')) { Remove-Item -LiteralPath (Join-Path $env:LOCALAPPDATA ('envsetup\' + $f)) -Force -ErrorAction SilentlyContinue }
`, psQuote(legacyRunValue), psQuote(legacyXML), psQuote(legacyPins), psQuote(legacyGuard), psQuote(legacyDir))
	if c.ClearStart {
		b.WriteString(`$state = Join-Path $env:LOCALAPPDATA 'Packages\Microsoft.Windows.StartMenuExperienceHost_cw5n1h2txyewy\LocalState'
if (Test-Path -LiteralPath $state) { Get-ChildItem -LiteralPath $state -Filter 'start*.bin' -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue }
Stop-Process -Name 'StartMenuExperienceHost' -Force -ErrorAction SilentlyContinue
`)
	}
	pinned := Pinned(c.Taskbar)
	if len(pinned) == 0 {
		b.WriteString("Stop-Process -Name 'explorer' -Force -ErrorAction SilentlyContinue\n")
		return b.String()
	}
	b.WriteString(pinHelper)
	b.WriteString(pinFunctions)
	for _, it := range pinned {
		names := make([]string, len(it.Match))
		for i, m := range it.Match {
			names[i] = psQuote(m)
		}
		list := "@(" + strings.Join(names, ", ") + ")"
		switch {
		case it.AUMID != "":
			fmt.Fprintf(&b, "PinId %s %s %s\n", psQuote(it.Name), psQuote(it.AUMID), list)
		case it.Browser:
			fmt.Fprintf(&b, "PinBrowser %s %s\n", psQuote(it.Name), list)
		default:
			fmt.Fprintf(&b, "PinLink %s %s\n", psQuote(it.Name), list)
		}
	}
	b.WriteString(`if ($pins.Count -eq 0) {
  Stop-Process -Name 'explorer' -Force -ErrorAction SilentlyContinue
  throw 'none of the chosen apps was found in the Start menu: install them first, then apply again'
}
`)
	b.WriteString(pinWrite)
	return b.String()
}
