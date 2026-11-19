# Tells whether a program is installed: $true when an entry of Installed apps
# (the Uninstall keys, machine and user) has a DisplayName like the pattern.
# Used by the `check` of catalogue apps that are downloaded from their site:
#   if (-not (& "$env:ENVSETUP_ROOT\windows\installed.ps1" 'MCHOSE HUB*')) { exit 1 }
param([Parameter(Mandatory)][string]$Name)

$keys = @(
  'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
  'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
  'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*'
)
$found = Get-ItemProperty -Path $keys -ErrorAction SilentlyContinue |
  Where-Object { $_.DisplayName -like $Name } | Select-Object -First 1
return [bool]$found
