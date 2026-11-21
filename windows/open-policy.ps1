# Opens a group policy where it can be changed by hand.
#
#   -Gpedit    opens the Local Group Policy Editor, walks its tree to the
#              policy's folder (Computer Configuration > Administrative
#              Templates > -Path), selects the policy and opens its window.
#              gpedit cannot be told where to open, so this drives its window
#              with UI Automation; gpedit runs elevated, so this does too.
#   -Registry  opens the Registry Editor at -Key (the deepest part of it that
#              exists): regedit opens at the key in its LastKey setting.
#
# Prints what it did; when a step fails, what was left to do by hand.
param(
  [string]$Path,     # "Componentes do Windows > Coleta de Dados e Compilações de Visualização"
  [string]$Title,    # "Permitir Dados de Diagnóstico"
  [string]$Also,     # other names of the policy in other builds, separated by "|"
  [string]$Key,      # HKLM\SOFTWARE\Policies\Microsoft\Windows\DataCollection
  [switch]$Registry,
  [string]$Result    # (internal) where the elevated copy writes its report
)
$ErrorActionPreference = 'Stop'

if ($Registry) {
  if (-not $Key) { throw 'give -Key' }
  # the deepest part of the key that exists (a policy not applied yet has
  # no key of its own)
  $full = $Key -replace '^HKLM\\', 'HKEY_LOCAL_MACHINE\' -replace '^HKCU\\', 'HKEY_CURRENT_USER\'
  $at = $full
  while ($at -match '\\' -and -not (Test-Path -LiteralPath "Registry::$at")) { $at = $at.Substring(0, $at.LastIndexOf('\')) }
  if (Get-Process -Name regedit -ErrorAction SilentlyContinue) {
    throw 'close the Registry Editor first: it only goes to a key when it starts'
  }
  $applet = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Applets\Regedit'
  if (-not (Test-Path $applet)) { New-Item -Path $applet | Out-Null }
  # LastKey starts with the localized name of the root ("Computador", "Computer")
  $prefix = (Get-ItemProperty -Path $applet -Name LastKey -ErrorAction SilentlyContinue).LastKey -replace '\\HKEY.*$', ''
  if (-not $prefix) { $prefix = if ((Get-UICulture).Name -like 'pt*') { 'Computador' } else { 'Computer' } }
  Set-ItemProperty -Path $applet -Name LastKey -Value "$prefix\$at"
  Start-Process regedit.exe
  if ($at -ne $full) { Write-Output "opened at $at ($full does not exist yet: the policy is not applied)" } else { Write-Output "opened at $full" }
  return
}

if (-not (Test-Path (Join-Path $env:SystemRoot 'System32\gpedit.msc'))) {
  throw 'this Windows has no Group Policy editor (gpedit.msc comes with Pro, Enterprise and Education)'
}

# UI Automation cannot reach an elevated window from a normal process: run
# elevated (one UAC prompt) and read the report afterwards.
$admin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $admin) {
  $report = Join-Path $env:TEMP ('envsetup-policy-' + [guid]::NewGuid().ToString('N').Substring(0, 8) + '.txt')
  $q = { param($s) "'" + ($s -replace "'", "''") + "'" }
  $cmd = "& $(& $q $PSCommandPath) -Path $(& $q $Path) -Title $(& $q $Title) -Also $(& $q $Also) -Result $(& $q $report)"
  $enc = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($cmd))
  $p = Start-Process powershell.exe -Verb RunAs -Wait -PassThru -WindowStyle Hidden -ArgumentList '-NoProfile', '-ExecutionPolicy', 'Bypass', '-EncodedCommand', $enc
  if (Test-Path $report) { Get-Content -LiteralPath $report -Encoding UTF8; Remove-Item -LiteralPath $report }
  if ($p.ExitCode) { exit $p.ExitCode }
  return
}

$log = New-Object System.Collections.Generic.List[string]
# every step also goes to a log file, to see where the navigation stopped
$logFile = Join-Path $env:TEMP 'envsetup-open-policy.log'
"---- $(Get-Date -Format s) path='$Path' title='$Title'" | Out-File -LiteralPath $logFile -Encoding UTF8
function Trace([string]$s) { $s | Out-File -LiteralPath $logFile -Append -Encoding UTF8 }
function Say([string]$s) { $log.Add($s); Trace "SAY $s" }
function Finish([int]$code) {
  if ($Result) { $log | Set-Content -LiteralPath $Result -Encoding UTF8 } else { $log }
  exit $code
}

try {
  Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes, System.Windows.Forms
  $A = [Windows.Automation.AutomationElement]
  $scope = [Windows.Automation.TreeScope]
  $ctl = [Windows.Automation.ControlType]
  $keys = [System.Windows.Forms.SendKeys]
  # EnvsetupTyper types text as Unicode characters (SendInput), so letters
  # with accents work whatever the keyboard layout
  if (-not ('EnvsetupTyper' -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class EnvsetupTyper {
  [StructLayout(LayoutKind.Sequential)] struct KEYBDINPUT { public ushort wVk; public ushort wScan; public uint dwFlags; public uint time; public IntPtr dwExtraInfo; }
  [StructLayout(LayoutKind.Sequential)] struct MOUSEINPUT { public int dx; public int dy; public uint mouseData; public uint dwFlags; public uint time; public IntPtr dwExtraInfo; }
  [StructLayout(LayoutKind.Explicit)] struct UNION { [FieldOffset(0)] public KEYBDINPUT ki; [FieldOffset(0)] public MOUSEINPUT mi; }
  [StructLayout(LayoutKind.Sequential)] struct INPUT { public uint type; public UNION u; }
  [DllImport("user32.dll", SetLastError = true)] static extern uint SendInput(uint n, INPUT[] inputs, int size);
  public static void Type(string text) {
    foreach (char c in text) {
      INPUT[] io = new INPUT[2];
      io[0].type = 1; io[0].u.ki.wScan = c; io[0].u.ki.dwFlags = 4;
      io[1].type = 1; io[1].u.ki.wScan = c; io[1].u.ki.dwFlags = 4 | 2;
      SendInput(2, io, Marshal.SizeOf(typeof(INPUT)));
    }
  }
}
"@
  }

  $isTree = New-Object Windows.Automation.PropertyCondition($A::ClassNameProperty, 'SysTreeView32')
  $isMain = New-Object Windows.Automation.PropertyCondition($A::ClassNameProperty, 'MMCMainFrame')
  $mmc = Start-Process mmc.exe -ArgumentList 'gpedit.msc' -PassThru
  Trace "started mmc pid $($mmc.Id)"
  # the editor window: an MMC main window with a tree, of the mmc started
  # now or (mmc may hand over to another process) any new one
  $win = $null
  for ($i = 0; $i -lt 60 -and -not $win; $i++) {
    Start-Sleep -Milliseconds 500
    foreach ($w in [Windows.Automation.AutomationElement]::RootElement.FindAll($scope::Children, $isMain)) {
      if ($w.FindFirst($scope::Descendants, $isTree) -and ($w.Current.ProcessId -eq $mmc.Id -or $w.Current.Name -match 'Pol[ií]tica de Grupo|Group Policy')) { $win = $w; break }
    }
  }
  if (-not $win) { Say 'the Group Policy editor did not open'; Finish 1 }
  Trace "window '$($win.Current.Name)' pid $($win.Current.ProcessId)"
  $tree = $win.FindFirst($scope::Descendants, $isTree)
  $isItem = New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::TreeItem)
  try { $win.SetFocus() } catch {}

  function Norm([string]$s) { ($s -replace '\s+', ' ').Trim().ToLowerInvariant() }
  # the policy's names: its title and, from other builds, the -Also ones
  $titles = @(@($Title) + @($Also -split '\|') | Where-Object { $_ } | ForEach-Object { Norm $_ })
  function IsPolicy([string]$name) { $titles -contains (Norm $name) }
  function Kids($parent) { @($parent.FindAll($scope::Children, $isItem)) }
  function Pick($item) {
    try { $item.GetCurrentPattern([Windows.Automation.SelectionItemPattern]::Pattern).Select() } catch {}
    try { $item.SetFocus() } catch {}
    Start-Sleep -Milliseconds 400
  }
  # Open shows the children of a tree item: the Expand of UI Automation,
  # then, if MMC did not fill them in, the right arrow on the selected item
  function Open($item) {
    try { $item.GetCurrentPattern([Windows.Automation.ExpandCollapsePattern]::Pattern).Expand() } catch { Trace "expand failed: $($_.Exception.Message)" }
    for ($t = 0; $t -lt 8; $t++) { if ((Kids $item).Count) { return }; Start-Sleep -Milliseconds 250 }
    Pick $item
    $keys::SendWait('{RIGHT}')
    for ($t = 0; $t -lt 12; $t++) { if ((Kids $item).Count) { return }; Start-Sleep -Milliseconds 250 }
  }
  # Child finds the item under $parent named one of $names (exact, then the
  # start of the name)
  function Child($parent, [string[]]$names) {
    if ($parent.Current.ControlType -eq $ctl::TreeItem) { Open $parent }
    $kids = Kids $parent
    Trace ("under '{0}': {1}" -f $parent.Current.Name, (($kids | ForEach-Object { $_.Current.Name }) -join ' | '))
    foreach ($n in $names) { foreach ($k in $kids) { if ((Norm $k.Current.Name) -eq (Norm $n)) { return $k } } }
    foreach ($n in $names) { foreach ($k in $kids) { if ((Norm $k.Current.Name).StartsWith((Norm $n))) { return $k } } }
    return $null
  }

  $steps = @(, @('Configuração do Computador', 'Computer Configuration'))
  $steps += , @('Modelos Administrativos', 'Administrative Templates')
  foreach ($seg in ($Path -split '\s*>\s*')) { if ($seg) { $steps += , @($seg) } }

  $roots = Kids $tree
  Trace ("tree items at the top: {0}" -f $roots.Count)
  if ($roots.Count -eq 0) {
    # UI Automation cannot see the items: walk the tree with the keyboard.
    # Typing the start of a name selects the next item that begins with it.
    Trace 'keyboard mode'
    $tree.SetFocus(); Start-Sleep -Milliseconds 300
    $keys::SendWait('{HOME}'); Start-Sleep -Milliseconds 300
    foreach ($names in $steps) {
      $keys::SendWait('{RIGHT}'); Start-Sleep -Milliseconds 800
      # only the part before the first accented letter: SendKeys cannot type
      # every letter on every keyboard, and a prefix is enough to find it
      $typed = ($names[0] -replace '[^\x20-\x7E].*$', '')
      if ($typed.Length -gt 24) { $typed = $typed.Substring(0, 24) }
      $keys::SendWait(($typed -replace '([+^%~(){}\[\]])', '{$1}')); Start-Sleep -Milliseconds 900
      Trace "typed '$typed'"
    }
    # the list of policies on the right: Tab does not reach it (it goes to
    # the Extended/Standard tabs), so it gets the focus directly
    Start-Sleep -Milliseconds 800
    $isList = New-Object Windows.Automation.PropertyCondition($A::ClassNameProperty, 'SysListView32')
    $list = $null; $best = 0
    foreach ($l in $win.FindAll($scope::Descendants, $isList)) {
      $r = $l.Current.BoundingRectangle
      if (-not $l.Current.IsOffscreen -and $r.Width * $r.Height -gt $best) { $best = $r.Width * $r.Height; $list = $l }
    }
    if (-not $list) {
      [System.Windows.Forms.Clipboard]::SetText($Title)
      Say ("the folder is open, but its list of policies was not found: look for '{0}' there (the name was copied to the clipboard)" -f $Title)
      Finish 2
    }
    $isRow = New-Object Windows.Automation.OrCondition(
      (New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::ListItem)),
      (New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::DataItem)))
    $rows = @($list.FindAll($scope::Descendants, $isRow))
    Trace ("list rows seen: {0}" -f $rows.Count)
    $row = $null
    foreach ($r in $rows) { if (IsPolicy $r.Current.Name) { $row = $r; break } }
    $shown = $Title
    if ($row) {
      $shown = $row.Current.Name
      Pick $row
    } else {
      # the rows cannot be read either: type the whole name, which selects
      # the row that begins with it. Typed as Unicode characters, so accents
      # work on any keyboard layout.
      $list.SetFocus(); Start-Sleep -Milliseconds 300
      $keys::SendWait('{HOME}'); Start-Sleep -Milliseconds 1200
      [EnvsetupTyper]::Type($Title); Start-Sleep -Milliseconds 600
      Trace "typed the title in the list"
    }
    $keys::SendWait('{ENTER}')
    # the policy window has the policy's name as its title; it can take a few
    # seconds to show up, and may not be a direct child of the desktop
    $isTitled = New-Object Windows.Automation.PropertyCondition($A::NameProperty, $shown)
    $dialog = $null
    for ($i = 0; $i -lt 30 -and -not $dialog; $i++) {
      Start-Sleep -Milliseconds 300
      $dialog = [Windows.Automation.AutomationElement]::RootElement.FindFirst($scope::Children, $isTitled)
      if (-not $dialog) { $dialog = $win.FindFirst($scope::Descendants, (New-Object Windows.Automation.AndCondition($isTitled, (New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::Window))))) }
    }
    if (-not $dialog) {
      [System.Windows.Forms.Clipboard]::SetText($Title)
      Say ("selected '{0}' in its folder; if its window did not open, press Enter (the name was copied to the clipboard)" -f $Title)
      Finish 0
    }
    Trace "dialog '$($dialog.Current.Name)'"
    Say ("opened: {0}" -f $shown)
    Finish 0
  }

  $node = $null
  foreach ($r in $roots) { if ((Norm $r.Current.Name) -match 'pol[ií]tica|policy') { $node = $r } }
  if (-not $node) { $node = $roots[0] }
  $where = @($node.Current.Name)
  foreach ($names in $steps) {
    $next = Child $node $names
    if (-not $next) {
      Pick $node
      $there = (Kids $node | ForEach-Object { $_.Current.Name }) -join ', '
      Say ("stopped at '{0}': '{1}' is not there (it has: {2})" -f ($where -join ' > '), $names[0], $there)
      Finish 2
    }
    $node = $next
    $where += $node.Current.Name
  }
  Pick $node
  Say ("folder: {0}" -f ($where -join ' > '))

  # the policy in the list on the right (a details view: its rows are data
  # items, or list items); open it like a double click
  $isRow = New-Object Windows.Automation.OrCondition(
    (New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::ListItem)),
    (New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::DataItem)))
  $row = $null
  for ($i = 0; $i -lt 20 -and -not $row; $i++) {
    $rows = @($win.FindAll($scope::Descendants, $isRow))
    foreach ($r in $rows) { if (IsPolicy $r.Current.Name) { $row = $r; break } }
    if (-not $row) { Start-Sleep -Milliseconds 300 }
  }
  if (-not $row) {
    Trace ("rows: {0}" -f (($rows | ForEach-Object { $_.Current.Name }) -join ' | '))
    [System.Windows.Forms.Clipboard]::SetText($Title)
    Say ("the folder is open, but '{0}' is not in its list: look for it there (the name was copied to the clipboard)" -f $Title)
    Finish 2
  }
  Pick $row
  $keys::SendWait('{ENTER}')
  Say ("opened: {0}" -f $row.Current.Name)
  Finish 0
} catch {
  Say ("error: {0} (line {1})" -f $_.Exception.Message, $_.InvocationInfo.ScriptLineNumber)
  Finish 1
}
