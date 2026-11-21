# 8BitDo Ultimate Software V1: the legacy software for the first-generation
# Ultimate 2.4G controllers. 8BitDo distributes it as a signed portable ZIP,
# not as an MSI or winget package.
param([switch]$Check)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$name = '8BitDo Ultimate Software V1'
$dir = Join-Path $env:LOCALAPPDATA 'Programs\8BitDo Ultimate Software V1'
$exe = Join-Path $dir '8BitDo Ultimate Software.exe'
$lnk = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$name.lnk"

if ($Check) {
  if ((Test-Path -LiteralPath $exe) -and (Test-Path -LiteralPath $lnk)) { exit 0 }
  exit 1
}

$url = 'https://download.8bitdo.com/Ultimate-Software/8BitDo-Ultimate-Software-for-Windows.zip'
$tmp = Join-Path $env:TEMP ('8BitDo-V1-' + [guid]::NewGuid().ToString('N'))
$zip = "$tmp.zip"
$copyFrom = $null
try {
  Write-Output "downloading 8BitDo Ultimate Software V1"
  Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
  New-Item -ItemType Directory -Force -Path $tmp | Out-Null
  Expand-Archive -LiteralPath $zip -DestinationPath $tmp -Force
  $copyFrom = Get-ChildItem -LiteralPath $tmp -Recurse -Filter '8BitDo Ultimate Software.exe' |
    Select-Object -First 1 -ExpandProperty FullName
  if (-not $copyFrom) { throw 'the official 8BitDo ZIP did not contain 8BitDo Ultimate Software.exe' }

  $sig = Get-AuthenticodeSignature -LiteralPath $copyFrom
  if ($sig.Status -ne 'Valid') { throw "the 8BitDo executable signature is not valid: $($sig.Status)" }
  if ($sig.SignerCertificate.Subject -notmatch '8BitDo|壹位堂|8BITDO') {
    throw "unexpected signer for 8BitDo Ultimate Software V1: $($sig.SignerCertificate.Subject)"
  }

  Get-Process -Name '8BitDo Ultimate Software' -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  $sourceDir = Split-Path -Parent $copyFrom
  Get-ChildItem -LiteralPath $sourceDir -Force |
    Copy-Item -Destination $dir -Recurse -Force
  Unblock-File -LiteralPath $exe
  Write-Output "8BitDo Ultimate Software V1 installed in $dir"
} finally {
  Remove-Item -LiteralPath $zip -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

& (Join-Path $PSScriptRoot 'register-portable-app.ps1') -Path $exe -Name $name -Publisher '8BitDo'
