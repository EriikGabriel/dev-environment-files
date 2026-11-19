# Points the Playnite library at a folder that moved: every text value of the
# library databases (games, emulators, game scanners) that starts with -From
# is rewritten to start with -To. Used when the games folder or the RetroArch
# folder changes place (another drive letter, D:\Games instead of C:\...).
#
# The databases are LiteDB files; they are edited with the LiteDB.dll that
# comes with Playnite, so Playnite must be closed. Prints one line per
# database changed and, at the end, "changed: <n>".
param(
  [Parameter(Mandatory)][string]$Playnite,  # the Playnite folder
  [Parameter(Mandatory)][string]$From,
  [Parameter(Mandatory)][string]$To
)
$ErrorActionPreference = 'Stop'

$from = $From.TrimEnd('\')
$to = $To.TrimEnd('\')
if ($from -eq '' -or $from -ieq $to) { Write-Output 'changed: 0'; return }
if (Get-Process -Name 'Playnite.DesktopApp', 'Playnite.FullscreenApp' -ErrorAction SilentlyContinue) {
  throw 'close Playnite first: its library cannot be edited while it is open'
}
Add-Type -Path (Join-Path $Playnite 'LiteDB.dll')

# rewrite returns the value with the prefix replaced, or $null when it does
# not start with the old folder (as a whole path segment).
function Rewrite([string]$s) {
  if ($s.Length -lt $from.Length -or -not $s.StartsWith($from, [StringComparison]::OrdinalIgnoreCase)) { return $null }
  if ($s.Length -gt $from.Length -and $s[$from.Length] -ne '\' -and $s[$from.Length] -ne '/') { return $null }
  return $to + $s.Substring($from.Length)
}

# fix walks a BSON value and returns how many strings it changed.
function Fix($value) {
  $n = 0
  if ($value -is [LiteDB.BsonDocument]) {
    foreach ($k in @($value.Keys)) {
      $v = $value[$k]
      if ($v.IsString) {
        $new = Rewrite $v.AsString
        if ($null -ne $new) { $value[$k] = New-Object LiteDB.BsonValue($new); $n++ }
      } elseif ($v.IsDocument -or $v.IsArray) {
        $n += Fix $(if ($v.IsDocument) { $v.AsDocument } else { $v.AsArray })
      }
    }
  } elseif ($value -is [LiteDB.BsonArray]) {
    for ($i = 0; $i -lt $value.Count; $i++) {
      $v = $value[$i]
      if ($v.IsString) {
        $new = Rewrite $v.AsString
        if ($null -ne $new) { $value[$i] = New-Object LiteDB.BsonValue($new); $n++ }
      } elseif ($v.IsDocument -or $v.IsArray) {
        $n += Fix $(if ($v.IsDocument) { $v.AsDocument } else { $v.AsArray })
      }
    }
  }
  return $n
}

$total = 0
foreach ($name in 'games', 'emulators', 'scanners') {
  $file = Join-Path $Playnite "library\$name.db"
  if (-not (Test-Path $file)) { continue }
  $db = New-Object LiteDB.LiteDatabase("Filename=$file")
  try {
    $count = 0
    foreach ($c in $db.GetCollectionNames()) {
      $col = $db.GetCollection($c)
      foreach ($doc in @($col.FindAll())) {
        $n = Fix $doc
        if ($n -gt 0) { [void]$col.Update($doc); $count += $n }
      }
    }
    if ($count -gt 0) { Write-Output ("{0}.db: {1} paths" -f $name, $count) }
    $total += $count
  } finally {
    $db.Dispose()
  }
}
Write-Output "changed: $total"
