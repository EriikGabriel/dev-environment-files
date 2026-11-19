# Installs the newest Windhawk 2 from the project's GitHub releases.
#
# winget only has Windhawk 1.x (the stable line), and 1.x has no
# windhawk-cli.exe, which envsetup needs to install the mods. Windhawk 2 is
# published as a pre-release for now, so this picks the newest release (pre-release
# or not) whose version is 2 or later, and installs it over what is there
# (an upgrade from 1.x keeps the settings and the mods).
#
#   -DryRun   resolve, download and verify the installer, then stop before running it
#
# It does nothing when that release is already installed.
param([switch]$DryRun)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is slow with the progress bar
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$repo = 'ramensoftware/windhawk'
$asset = 'windhawk_setup.exe' # the small online installer; the _offline one carries the development tools too
$headers = @{ 'User-Agent' = 'envsetup'; 'Accept' = 'application/vnd.github+json' }

# windhawk-cli.exe only exists from Windhawk 2 on, which is what makes it the test.
function Find-Cli {
  foreach ($dir in @($env:ProgramFiles, $env:ProgramW6432)) {
    if ($dir) {
      $p = Join-Path $dir 'Windhawk\windhawk-cli.exe'
      if (Test-Path -LiteralPath $p) { return $p }
    }
  }
}

function Test-Wanted([string]$tag) { return ($tag -match '^v?([0-9]+)\.') -and ([int]$Matches[1] -ge 2) }

# The newest release of version 2 or later: from the API (it also gives the
# SHA-256 of the file) or, when that is refused (rate limit), from the Atom feed.
function Get-Release {
  # Write-Host, not Write-Output: this function's output is its return value.
  try {
    # Windows PowerShell 5.1 hands the JSON list back as ONE object holding the
    # array; piping it through ForEach-Object gives one release at a time.
    $all = @(Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases?per_page=30" -Headers $headers | ForEach-Object { $_ })
    $r = $all | Where-Object { -not $_.draft -and (Test-Wanted $_.tag_name) } |
      Sort-Object { [datetime]$_.published_at } -Descending | Select-Object -First 1
    $a = if ($r) { $r.assets | Where-Object { $_.name -eq $asset } | Select-Object -First 1 }
    if ($a) {
      $sha = $null
      if ($a.digest -match '^sha256:([0-9a-fA-F]{64})$') { $sha = $Matches[1] }
      return [pscustomobject]@{ Tag = $r.tag_name; Url = $a.browser_download_url; Sha256 = $sha; Pre = [bool]$r.prerelease }
    }
    Write-Host "the GitHub API listed $($all.Count) releases and none of version 2 with $asset; reading the releases feed"
  } catch {
    Write-Host "the GitHub API did not answer ($($_.Exception.Message)); reading the releases feed"
  }
  $feed = [xml](Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/$repo/releases.atom" -Headers $headers).Content
  foreach ($e in @($feed.feed.entry | Sort-Object { [datetime]$_.updated } -Descending)) {
    $tag = ($e.link.href -split '/tag/')[-1]
    if (Test-Wanted $tag) {
      return [pscustomobject]@{ Tag = $tag; Url = "https://github.com/$repo/releases/download/$tag/$asset"; Sha256 = $null; Pre = $null }
    }
  }
  throw "no Windhawk 2 release found at github.com/$repo/releases"
}

$rel = Get-Release
$kind = if ($rel.Pre) { ' (pre-release)' } else { '' }
Write-Output "newest Windhawk 2: $($rel.Tag)$kind"

$have = Find-Cli
if ($have) {
  $v = (Get-Item -LiteralPath $have).VersionInfo.ProductVersion
  if ($v -eq $rel.Tag.TrimStart('v')) {
    Write-Output "Windhawk $v is already installed"
    return
  }
  Write-Output "Windhawk $v is installed: upgrading"
}

$dir = Join-Path $env:TEMP ('envsetup-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Path $dir | Out-Null
try {
  $file = Join-Path $dir $asset
  Write-Output "downloading $($rel.Url)"
  Invoke-WebRequest -UseBasicParsing -Uri $rel.Url -OutFile $file -Headers $headers
  if ($rel.Sha256) {
    $got = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash
    if ($got -ne $rel.Sha256) { throw "the download does not match the SHA-256 GitHub lists ($got, expected $($rel.Sha256))" }
    Write-Output 'SHA-256 matches the one GitHub lists'
  }
  $sig = Get-AuthenticodeSignature -FilePath $file
  Write-Output ("installer: {0} (signature: {1}{2})" -f $asset, $sig.Status, $(if ($sig.SignerCertificate) { ', ' + $sig.SignerCertificate.GetNameInfo('SimpleName', $false) } else { '' }))
  if ($sig.Status -eq 'HashMismatch') { throw 'the installer signature does not match the file' }
  if ($DryRun) { Write-Output 'dry run: the installer is ready and was not run'; return }

  # The installer asks for elevation by itself. /S is silent; /STANDARD is the
  # normal (not portable) install, the same switches winget uses for 1.x.
  Write-Output 'installing without questions (accept the elevation prompt)'
  $p = Start-Process -FilePath $file -ArgumentList '/S', '/STANDARD' -Wait -PassThru
  if (Find-Cli) { Write-Output "installed Windhawk $($rel.Tag)"; return }

  Write-Output "the silent install did not leave windhawk-cli.exe (exit code $($p.ExitCode)); the installer is open: follow it to the end"
  Start-Process -FilePath $file -Wait
  if (-not (Find-Cli)) { throw 'windhawk-cli.exe is not in C:\Program Files\Windhawk after the installer closed' }
  Write-Output "installed Windhawk $($rel.Tag)"
} finally {
  Remove-Item -Recurse -Force $dir -ErrorAction SilentlyContinue
}
