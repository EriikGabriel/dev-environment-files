# Reset Explode (github.com/GustavoReset/ResetExplode): a portable .NET program
# (one ResetExplode.exe, asks for elevation when it opens), published on the
# release with the tag "latest". It goes to %LOCALAPPDATA%\Programs\ResetExplode
# and is registered like Playnite: Start menu shortcut and an entry in
# Installed apps (register-portable-app.ps1). The download must match the
# SHA-256 GitHub publishes for the asset; the program is not signed.
#
#   install-reset-explode.ps1          install, or update when the release changed
#   install-reset-explode.ps1 -Check   exit 1 when it is not installed and registered
param([switch]$Check)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$name = 'Reset Explode'
$dir = Join-Path $env:LOCALAPPDATA 'Programs\ResetExplode'
$exe = Join-Path $dir 'ResetExplode.exe'
$lnk = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$name.lnk"

if ($Check) {
  if ((Test-Path -LiteralPath $exe) -and (Test-Path -LiteralPath $lnk)) { exit 0 }
  exit 1
}

$api = 'https://api.github.com/repos/GustavoReset/ResetExplode/releases/tags/latest'
$release = Invoke-RestMethod -Uri $api -Headers @{ Accept = 'application/vnd.github+json'; 'User-Agent' = 'envsetup' }
# PowerShell 5.1 hands a JSON array back as one object: unroll it
$asset = @($release.assets | ForEach-Object { $_ }) | Where-Object { $_.name -eq 'ResetExplode.exe' } | Select-Object -First 1
if (-not $asset) { throw 'ResetExplode.exe is not on the "latest" release of GustavoReset/ResetExplode' }
if ($asset.digest -notmatch '^sha256:([0-9a-fA-F]{64})$') { throw 'GitHub did not publish the SHA-256 of ResetExplode.exe: not installing an unchecked download' }
$want = $Matches[1].ToLowerInvariant()

if ((Test-Path -LiteralPath $exe) -and (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant() -eq $want) {
  Write-Output "Reset Explode is up to date in $dir"
} else {
  $tmp = Join-Path $env:TEMP ('ResetExplode-' + [guid]::NewGuid().ToString('N') + '.exe')
  Write-Output ("downloading Reset Explode ({0:N0} MB)" -f ($asset.size / 1MB))
  try {
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $tmp -UseBasicParsing
    $got = (Get-FileHash -LiteralPath $tmp -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($got -ne $want) { throw "the download does not match the SHA-256 GitHub published ($got, want $want)" }
    Get-Process -Name 'ResetExplode' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Move-Item -LiteralPath $tmp -Destination $exe -Force
  } finally {
    Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
  }
  Unblock-File -LiteralPath $exe
  Write-Output "Reset Explode installed in $dir"
}
& (Join-Path $PSScriptRoot 'register-portable-app.ps1') -Path $exe -Name $name -Publisher 'GustavoReset'
