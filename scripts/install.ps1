# Downloads the latest envsetup installer from the GitHub releases and runs it.
#
#   irm https://github.com/EriikGabriel/dev-environment-files/releases/latest/download/install.ps1 | iex
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$repo = 'EriikGabriel/dev-environment-files'

$release = Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/$repo/releases/latest"
$asset = $release.assets | Where-Object { $_.name -like '*-setup.exe' } | Select-Object -First 1
if (-not $asset) { throw "the release $($release.tag_name) has no Windows installer (*-setup.exe)" }

$file = Join-Path $env:TEMP $asset.name
Write-Host "downloading envsetup $($release.tag_name) ($([math]::Round($asset.size / 1MB)) MB)"
Invoke-WebRequest -UseBasicParsing -Uri $asset.browser_download_url -OutFile $file
Write-Host 'the installer is open: follow it, then open envsetup from the Start menu'
Start-Process -FilePath $file -Wait
Remove-Item $file -ErrorAction SilentlyContinue
