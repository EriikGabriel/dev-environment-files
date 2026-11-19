package icons

// RestoreScript runs elevated and puts the Windows icons back, whatever the
// manifest knows:
//
//   - every shortcut on the desktops, in the Start menus and pinned (taskbar,
//     Start, jump lists) that points into icoDir gets the icon location it had
//     (the manifest's, or the one of the shortcut it was copied from) or, when
//     nothing is known about it, the icon of its target;
//   - the system icons (This PC, user files, Network, Recycle Bin) set from
//     the pack are removed;
//   - the manifest and the made icons are deleted;
//   - the icon cache of Windows is rebuilt, so no changed icon stays on
//     screen.
func RestoreScript(manifest, icoDir string) string {
	return `$ErrorActionPreference = 'Stop'
$manifest = ` + psQuote(manifest) + `
$icoDir = ` + psQuote(icoDir) + `
$old = @{}
$byIco = @{}
$system = @()
if (Test-Path -LiteralPath $manifest) {
  try {
    $m = Get-Content -Raw -Encoding UTF8 -LiteralPath $manifest | ConvertFrom-Json
    foreach ($e in @($m.entries)) { $old[$e.path.ToLowerInvariant()] = $e; if ($e.ico) { $byIco[$e.ico.ToLowerInvariant()] = $e } }
    $system = @($m.system)
  } catch { Write-Output ('the manifest could not be read: ' + $_.Exception.Message) }
}
function Ours($loc) { return $loc -and $loc.StartsWith($icoDir, [StringComparison]::OrdinalIgnoreCase) }
$roots = @(
  [Environment]::GetFolderPath('Desktop'),
  [Environment]::GetFolderPath('CommonDesktopDirectory'),
  (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu'),
  (Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu'),
  (Join-Path $env:APPDATA 'Microsoft\Internet Explorer\Quick Launch')
)
$files = @{}
foreach ($r in $roots) {
  if (-not $r -or -not (Test-Path -LiteralPath $r)) { continue }
  Get-ChildItem -LiteralPath $r -Filter *.lnk -File -Recurse -Force -ErrorAction SilentlyContinue | ForEach-Object { $files[$_.FullName.ToLowerInvariant()] = $_.FullName }
}
foreach ($e in $old.Values) { if (Test-Path -LiteralPath $e.path) { $files[$e.path.ToLowerInvariant()] = $e.path } }
$sh = New-Object -ComObject WScript.Shell
$done = 0; $failed = 0
foreach ($key in @($files.Keys)) {
  $path = $files[$key]
  try {
    $l = $sh.CreateShortcut($path)
    $loc = $l.IconLocation
    if (-not (Ours $loc)) { continue }
    $orig = ',0'
    $from = $old[$key]
    if (-not $from) { $from = $byIco[($loc -replace ',-?\d+$', '').ToLowerInvariant()] }
    if ($from -and $from.orig -and -not (Ours $from.orig)) { $orig = $from.orig }
    $l.IconLocation = $orig
    $l.Save()
    $done++
  } catch {
    $failed++
    Write-Output ('could not restore ' + $path + ': ' + $_.Exception.Message)
  }
}
# what the manifest says the system icons were
foreach ($s in $system) {
  try {
    if ($null -ne $s.prev -and -not (Ours ([string]$s.prev))) { Set-ItemProperty -LiteralPath $s.key -Name $s.name -Value $s.prev }
    elseif ($s.name -eq '(default)') { Remove-Item -LiteralPath $s.key -Recurse -Force -ErrorAction SilentlyContinue }
    else { Remove-ItemProperty -LiteralPath $s.key -Name $s.name -ErrorAction SilentlyContinue }
  } catch { }
}
# and whatever still points at a made icon, manifest or not
$classes = 'Registry::HKEY_CURRENT_USER\Software\Classes\CLSID\'
foreach ($clsid in '{20D04FE0-3AEA-1069-A2D8-08002B30309D}', '{59031a47-3f72-44a7-89c5-5595fe6b30ee}', '{F02C1A0D-BE21-4350-88B0-7367FC96EF3C}') {
  $k = $classes + $clsid + '\DefaultIcon'
  $item = Get-Item -LiteralPath $k -ErrorAction SilentlyContinue
  if ($item -and (Ours ([string]$item.GetValue('')))) { Remove-Item -LiteralPath $k -Recurse -Force -ErrorAction SilentlyContinue }
}
$bin = 'Registry::HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Explorer\CLSID\{645FF040-5081-101B-9F08-00AA002F954E}\DefaultIcon'
$item = Get-Item -LiteralPath $bin -ErrorAction SilentlyContinue
if ($item) {
  $defaults = @{ '' = '%SystemRoot%\System32\imageres.dll,-55'; 'empty' = '%SystemRoot%\System32\imageres.dll,-55'; 'full' = '%SystemRoot%\System32\imageres.dll,-54' }
  foreach ($n in $item.GetValueNames()) {
    if ((Ours ([string]$item.GetValue($n))) -and $defaults.ContainsKey($n)) {
      $name = if ($n -eq '') { '(default)' } else { $n }
      Set-ItemProperty -LiteralPath $bin -Name $name -Value $defaults[$n] -Type ExpandString
    }
  }
}
Remove-Item -LiteralPath $manifest -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $icoDir -Recurse -Force -ErrorAction SilentlyContinue
Write-Output ('restored ' + $done + ' shortcuts, ' + $failed + ' failed')
# Windows draws icons from its cache: rebuild it, or the changed ones stay.
try { & ie4uinit.exe -show } catch { }
Stop-Process -Name 'explorer' -Force -ErrorAction SilentlyContinue
$cache = Join-Path $env:LOCALAPPDATA 'Microsoft\Windows\Explorer'
for ($i = 0; $i -lt 3; $i++) {
  Get-ChildItem -LiteralPath $cache -Filter 'iconcache*.db' -Force -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 200
}
Remove-Item -LiteralPath (Join-Path $env:LOCALAPPDATA 'IconCache.db') -Force -ErrorAction SilentlyContinue
exit 0
`
}
