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
  $cmd = "& $(& $q $PSCommandPath) -Path $(& $q $Path) -Title $(& $q $Title) -Result $(& $q $report)"
  $enc = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($cmd))
  $p = Start-Process powershell.exe -Verb RunAs -Wait -PassThru -WindowStyle Hidden -ArgumentList '-NoProfile', '-ExecutionPolicy', 'Bypass', '-EncodedCommand', $enc
  if (Test-Path $report) { Get-Content -LiteralPath $report -Encoding UTF8; Remove-Item -LiteralPath $report }
  if ($p.ExitCode) { exit $p.ExitCode }
  return
}

$log = New-Object System.Collections.Generic.List[string]
function Say([string]$s) { $log.Add($s) }
function Finish([int]$code) {
  if ($Result) { $log | Set-Content -LiteralPath $Result -Encoding UTF8 } else { $log }
  exit $code
}

try {
  Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes, System.Windows.Forms
  $A = [Windows.Automation.AutomationElement]
  $scope = [Windows.Automation.TreeScope]
  $ctl = [Windows.Automation.ControlType]

  $isTree = New-Object Windows.Automation.PropertyCondition($A::ClassNameProperty, 'SysTreeView32')
  $mmc = Start-Process mmc.exe -ArgumentList 'gpedit.msc' -PassThru
  # the editor window: the mmc started now, with a tree
  $win = $null
  for ($i = 0; $i -lt 60 -and -not $win; $i++) {
    Start-Sleep -Milliseconds 500
    $cond = New-Object Windows.Automation.PropertyCondition($A::ProcessIdProperty, $mmc.Id)
    $w = [Windows.Automation.AutomationElement]::RootElement.FindFirst($scope::Children, $cond)
    # the tree of an MMC window is a SysTreeView32 (its items are only
    # readable from an elevated process, like mmc itself)
    if ($w -and $w.FindFirst($scope::Descendants, $isTree)) { $win = $w }
  }
  if (-not $win) { Say 'the Group Policy editor did not open'; Finish 1 }
  $tree = $win.FindFirst($scope::Descendants, $isTree)
  $isItem = New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::TreeItem)

  function Norm([string]$s) { ($s -replace '\s+', ' ').Trim().ToLowerInvariant() }
  # Child finds the tree item under $parent named one of $names (exact,
  # then the start of the name), expanding $parent first.
  function Child($parent, [string[]]$names) {
    if ($parent.Current.ControlType -eq $ctl::TreeItem) {
      try { $parent.GetCurrentPattern([Windows.Automation.ExpandCollapsePattern]::Pattern).Expand() } catch {}
    }
    for ($try = 0; $try -lt 10; $try++) {
      $kids = $parent.FindAll($scope::Children, $isItem)
      foreach ($n in $names) { foreach ($k in $kids) { if ((Norm $k.Current.Name) -eq (Norm $n)) { return $k } } }
      foreach ($n in $names) { foreach ($k in $kids) { if ((Norm $k.Current.Name).StartsWith((Norm $n))) { return $k } } }
      Start-Sleep -Milliseconds 300
    }
    return $null
  }
  function Pick($item) {
    try { $item.GetCurrentPattern([Windows.Automation.SelectionItemPattern]::Pattern).Select() } catch { $item.SetFocus() }
    Start-Sleep -Milliseconds 400
  }

  $node = Child $tree @('Política Computador Local', 'Local Computer Policy')
  $steps = @(@('Configuração do Computador', 'Computer Configuration'), @('Modelos Administrativos', 'Administrative Templates'))
  foreach ($seg in ($Path -split '\s*>\s*')) { if ($seg) { $steps += , @($seg) } }
  $where = @()
  foreach ($names in $steps) {
    if (-not $node) { break }
    $next = Child $node $names
    if (-not $next) {
      Pick $node
      Say ("stopped at '{0}': '{1}' was not found there. Open it by hand: {2}" -f ($where -join ' > '), $names[0], $Path)
      Finish 2
    }
    $node = $next
    $where += $node.Current.Name
  }
  if (-not $node) { Say 'the tree of the editor could not be read'; Finish 1 }
  Pick $node
  Say ("folder: {0}" -f ($where -join ' > '))

  # the policy in the list on the right (a details view: its rows are data
  # items, or list items); open it like a double click
  $isRow = New-Object Windows.Automation.OrCondition(
    (New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::ListItem)),
    (New-Object Windows.Automation.PropertyCondition($A::ControlTypeProperty, $ctl::DataItem)))
  $row = $null
  for ($i = 0; $i -lt 20 -and -not $row; $i++) {
    foreach ($r in $win.FindAll($scope::Descendants, $isRow)) {
      if ((Norm $r.Current.Name) -eq (Norm $Title)) { $row = $r; break }
    }
    if (-not $row) { Start-Sleep -Milliseconds 300 }
  }
  if (-not $row) {
    [System.Windows.Forms.Clipboard]::SetText($Title)
    Say ("the folder is open, but '{0}' is not in its list: look for it there (the name was copied to the clipboard)" -f $Title)
    Finish 2
  }
  Pick $row
  $row.SetFocus()
  [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
  Say ("opened: {0}" -f $Title)
  Finish 0
} catch {
  Say ("error: {0}" -f $_.Exception.Message)
  Finish 1
}
