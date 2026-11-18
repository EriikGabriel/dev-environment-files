# Makes a portable program a regular app of this user, without moving it:
# a Start menu shortcut (so Windows search, Raycast and the Start menu find it)
# and an entry in Settings > Apps > Installed apps, whose Uninstall only takes
# the registration away; the program's folder is never touched. No elevation.
#
#   register-portable-app.ps1 -Path C:\Playnite\Playnite.DesktopApp.exe -Name Playnite
#   register-portable-app.ps1 -Name Playnite -Unregister
param(
  [string]$Path,
  [Parameter(Mandatory)][string]$Name,
  [string]$Publisher,
  [switch]$Unregister
)
$ErrorActionPreference = 'Stop'
$lnk = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$Name.lnk"
$key = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\envsetup-$Name"

if ($Unregister) {
  Remove-Item -LiteralPath $lnk -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $key -Recurse -Force -ErrorAction SilentlyContinue
  Write-Output "$Name unregistered"
  exit 0
}
if (-not $Path -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "the program was not found: $Path" }
$exe = (Resolve-Path -LiteralPath $Path).Path
$dir = Split-Path -Parent $exe
$info = (Get-Item -LiteralPath $exe).VersionInfo

$sh = New-Object -ComObject WScript.Shell
$l = $sh.CreateShortcut($lnk)
$l.TargetPath = $exe
$l.WorkingDirectory = $dir
$l.IconLocation = "$exe,0"
$l.Description = $Name
$l.Save()

New-Item -Path $key -Force | Out-Null
$v = if ($info.ProductVersion) { $info.ProductVersion } else { $info.FileVersion }
$props = @{
  DisplayName     = $Name
  DisplayIcon     = "$exe,0"
  InstallLocation = $dir
  Publisher       = $(if ($Publisher) { $Publisher } elseif ($info.CompanyName) { $info.CompanyName } else { $Name })
  DisplayVersion  = $(if ($v) { $v } else { '' })
  UninstallString = 'powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "' + $PSCommandPath + '" -Name "' + $Name + '" -Unregister'
}
foreach ($k in $props.Keys) { Set-ItemProperty -LiteralPath $key -Name $k -Value $props[$k] -Type String }
foreach ($k in 'NoModify', 'NoRepair') { Set-ItemProperty -LiteralPath $key -Name $k -Value 1 -Type DWord }
Write-Output "$Name registered: $lnk"
