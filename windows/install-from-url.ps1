# Downloads a program's installer from its official site and installs it:
# silently first (with -Silent arguments), and when that does not leave it in
# Installed apps, the installer is opened to be followed by hand.
#
#   -Url      the installer, or a .zip with it inside
#   -Page     a page to read the installer link from, with -Pattern (a regex);
#             used instead of -Url when the link changes with each version
#   -Name     the DisplayName in Installed apps (wildcards), to know it worked
#   -Silent   arguments for an unattended install ("/S" for NSIS installers)
param(
  [string]$Url,
  [string]$Page,
  [string]$Pattern,
  [Parameter(Mandatory)][string]$Name,
  [string]$Silent = '/S'
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is slow with the progress bar
$agent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36'
$installed = Join-Path $PSScriptRoot 'installed.ps1'

if ($Page) {
  $html = (Invoke-WebRequest -UseBasicParsing -Uri $Page -UserAgent $agent).Content
  $m = [regex]::Match($html, $Pattern)
  if (-not $m.Success) {
    Start-Process $Page
    throw "could not find the installer link on $Page; the page was opened so you can download it by hand"
  }
  $Url = $m.Value
}
if (-not $Url) { throw 'give -Url or -Page with -Pattern' }

$dir = Join-Path $env:TEMP ('envsetup-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Path $dir | Out-Null
try {
  $file = Join-Path $dir ([IO.Path]::GetFileName(([uri]$Url).AbsolutePath))
  Write-Output "downloading $Url"
  Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $file -UserAgent $agent
  if ($file -like '*.zip') {
    Expand-Archive -Path $file -DestinationPath $dir -Force
    $file = Get-ChildItem -Path $dir -Recurse -Include '*.exe', '*.msi' | Sort-Object Length -Descending | Select-Object -First 1 -ExpandProperty FullName
    if (-not $file) { throw "no installer inside $Url" }
  }
  $sig = Get-AuthenticodeSignature -FilePath $file
  Write-Output ("installer: {0} (signature: {1}{2})" -f [IO.Path]::GetFileName($file), $sig.Status, $(if ($sig.SignerCertificate) { ', ' + $sig.SignerCertificate.GetNameInfo('SimpleName', $false) } else { '' }))

  if ($Silent) {
    Write-Output 'installing without questions (accept the elevation prompt if one shows up)'
    $p = Start-Process -FilePath $file -ArgumentList $Silent -Wait -PassThru
    if (& $installed $Name) { Write-Output 'installed'; return }
    Write-Output "the silent install did not finish (exit code $($p.ExitCode)); opening the installer"
  }
  Write-Output 'the installer is open: follow it to the end'
  Start-Process -FilePath $file -Wait
  if (-not (& $installed $Name)) { throw "$Name is not in Installed apps after the installer closed" }
  Write-Output 'installed'
} finally {
  Remove-Item -Recurse -Force $dir -ErrorAction SilentlyContinue
}
