# The Raycast extensions installed from the store, kept as a list in the repo.
#
# Raycast's own sync does not bring the extensions to another Windows machine,
# and it has no command to install one: each needs a click on Install in its
# store page. So the list is kept here, and -Install opens the page of each
# missing extension in Raycast, one at a time, and waits until it shows up as
# installed before going to the next.
#
#   raycast-extensions.ps1 -Save      write the extensions of this machine to the list
#   raycast-extensions.ps1 -Check     exit 1 when an extension of the list is missing
#   raycast-extensions.ps1 -Install   open the store page of each missing one
#
# An extension is author/name, as in its store link (raycast.com/author/name).
param(
  [switch]$Save,
  [switch]$Check,
  [switch]$Install,
  [string]$List = (Join-Path $PSScriptRoot 'raycast\extensions.txt'),
  [int]$WaitSeconds = 180
)
$ErrorActionPreference = 'Stop'
$dir = Join-Path $env:USERPROFILE '.config\raycast\extensions'

# author/name of every store extension installed here
function Get-Installed {
  $out = @{}
  if (-not (Test-Path -LiteralPath $dir)) { return $out }
  foreach ($d in Get-ChildItem -LiteralPath $dir -Directory -ErrorAction SilentlyContinue) {
    $pj = Join-Path $d.FullName 'package.json'
    if (-not (Test-Path -LiteralPath $pj)) { continue }
    try { $j = Get-Content -Raw -Encoding UTF8 -LiteralPath $pj | ConvertFrom-Json } catch { continue }
    $author = if ($j.owner) { $j.owner } else { $j.author }
    if ($author -and $j.name) { $out["$author/$($j.name)".ToLowerInvariant()] = [string]$j.title }
  }
  return $out
}

function Get-Wanted {
  if (-not (Test-Path -LiteralPath $List)) { return @() }
  return @(Get-Content -Encoding UTF8 -LiteralPath $List | ForEach-Object { ($_ -replace '#.*$', '').Trim() } | Where-Object { $_ -match '^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$' })
}

if ($Save) {
  $have = Get-Installed
  if ($have.Count -eq 0) { throw "no Raycast extension found in $dir" }
  $lines = @('# Raycast extensions (author/name, as in raycast.com/author/name), installed by',
    '# windows\raycast-extensions.ps1 -Install; refresh with -Save.')
  foreach ($k in ($have.Keys | Sort-Object)) { $lines += ('{0,-40} # {1}' -f $k, $have[$k]) }
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $List) | Out-Null
  [IO.File]::WriteAllText($List, (($lines -join "`n") + "`n"), (New-Object System.Text.UTF8Encoding $false))
  Write-Output "$($have.Count) extensions saved in $List"
  return
}

$wanted = Get-Wanted
$have = Get-Installed
$missing = @($wanted | Where-Object { -not $have.ContainsKey($_.ToLowerInvariant()) })

if ($Check) {
  if ($missing.Count) { Write-Output ("missing: " + ($missing -join ', ')); exit 1 }
  Write-Output "the $($wanted.Count) extensions of the list are installed"
  exit 0
}

if ($Install) {
  if (-not $missing.Count) { Write-Output "the $($wanted.Count) extensions of the list are installed"; return }
  if (-not (Test-Path -LiteralPath 'Registry::HKEY_CLASSES_ROOT\raycast')) { throw 'Raycast is not installed (its raycast:// links are not registered): install it first' }
  $n = 0
  foreach ($ext in $missing) {
    $n++
    Write-Output ("[{0}/{1}] {2}: Raycast opens its store page, click Install" -f $n, $missing.Count, $ext)
    try { Start-Process "raycast://extensions/$ext" } catch { Start-Process "https://www.raycast.com/$ext" }
    $until = (Get-Date).AddSeconds($WaitSeconds)
    while ((Get-Date) -lt $until -and -not (Get-Installed).ContainsKey($ext.ToLowerInvariant())) { Start-Sleep -Seconds 2 }
    if ((Get-Installed).ContainsKey($ext.ToLowerInvariant())) { Write-Output "    installed" } else { Write-Output "    not installed after $WaitSeconds s, going on" }
  }
  $left = @($wanted | Where-Object { -not (Get-Installed).ContainsKey($_.ToLowerInvariant()) })
  if ($left.Count) { throw ("still missing: " + ($left -join ', ') + ' (install them in the Raycast store and apply again)') }
  Write-Output 'all the extensions of the list are installed'
  return
}

Write-Output 'give -Save, -Check or -Install'
exit 2
