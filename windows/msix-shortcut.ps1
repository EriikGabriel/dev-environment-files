# Gives an MSIX app (Dia) a Start menu shortcut of this user, so it is themed
# like the other apps: the icons step only changes .lnk files, and an MSIX app
# has none. The shortcut's target is the app's own executable (its execution
# alias in %LOCALAPPDATA%/Microsoft/WindowsApps, which a shortcut can start) and
# it carries the app's AppUserModelID, so its taskbar pin and the app's window
# are the same button. It must not target explorer.exe (shell:AppsFolder/...):
# the shell then takes it for File Explorer, and the pin and the icons break.
# No elevation.
#
#   msix-shortcut.ps1 -Package TheBrowserCompany.Dia -Name Dia
#   msix-shortcut.ps1 -Package TheBrowserCompany.Dia -Name Dia -Check    exit 1 when missing
#   msix-shortcut.ps1 -Name Dia -Remove
param(
  [string]$Package,
  [Parameter(Mandatory)][string]$Name,
  [switch]$Check,
  [switch]$Remove
)
$ErrorActionPreference = 'Stop'
$lnk = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\$Name.lnk"

if ($Remove) {
  Remove-Item -LiteralPath $lnk -Force -ErrorAction SilentlyContinue
  Write-Output "$Name shortcut removed"
  exit 0
}
if ($Check) {
  if (-not (Test-Path -LiteralPath $lnk)) { exit 1 }
  # a shortcut an earlier version made opened the app through explorer.exe:
  # it counts as missing, so the item is pending and the shortcut is redone
  $t = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk).TargetPath
  if ([IO.Path]::GetFileName($t) -ieq 'explorer.exe') { exit 1 }
  exit 0
}
if (-not $Package) { throw '-Package is needed' }

$pkg = Get-AppxPackage -Name $Package | Sort-Object Version -Descending | Select-Object -First 1
if (-not $pkg) { throw "the package $Package is not installed" }
$app = @((Get-AppxPackageManifest -Package $pkg).Package.Applications.Application)[0]
if (-not $app) { throw "the package $Package has no application" }
$aumid = $pkg.PackageFamilyName + '!' + $app.Id
$exe = if ($app.Executable) { Join-Path $pkg.InstallLocation $app.Executable } else { $null }

# What the shortcut starts: the app's execution alias, which lives in a folder
# a user can read and which Windows starts as the app (the executable inside
# Program Files\WindowsApps cannot be started from a shortcut).
$alias = $null
foreach ($ext in @($app.Extensions.Extension)) {
  $a = $ext.AppExecutionAlias.ExecutionAlias.Alias
  if ($a) {
    $p = Join-Path $env:LOCALAPPDATA "Microsoft\WindowsApps\$a"
    if (Test-Path -LiteralPath $p) { $alias = $p; break }
  }
}
if (-not $alias) { throw "the package $Package has no execution alias to start: $Name would have to open through explorer.exe, which the shell takes for File Explorer" }

$sh = New-Object -ComObject WScript.Shell
$l = $sh.CreateShortcut($lnk)
$l.TargetPath = $alias
$l.Arguments = ''
$l.Description = $Name
# the app's own icon until the icons step points it at the pack's
if ($exe -and (Test-Path -LiteralPath $exe)) { $l.IconLocation = "$exe,0" }
$l.Save()

# System.AppUserModel.ID, which WScript.Shell cannot write
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class LinkAppId {
  [ComImport, Guid("00021401-0000-0000-C000-000000000046")] class CShellLink { }
  [ComImport, Guid("0000010b-0000-0000-C000-000000000046"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
  interface IPersistFile {
    void GetClassID(out Guid id); [PreserveSig] int IsDirty();
    void Load([MarshalAs(UnmanagedType.LPWStr)] string file, uint mode);
    void Save([MarshalAs(UnmanagedType.LPWStr)] string file, bool remember);
    void SaveCompleted([MarshalAs(UnmanagedType.LPWStr)] string file);
    void GetCurFile([MarshalAs(UnmanagedType.LPWStr)] out string file);
  }
  [StructLayout(LayoutKind.Sequential, Pack = 4)] struct PROPERTYKEY { public Guid fmtid; public uint pid; }
  [StructLayout(LayoutKind.Explicit, Size = 24)] struct PROPVARIANT { [FieldOffset(0)] public ushort vt; [FieldOffset(8)] public IntPtr p; }
  [ComImport, Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
  interface IPropertyStore {
    void GetCount(out uint n); void GetAt(uint i, out PROPERTYKEY key);
    void GetValue(ref PROPERTYKEY key, out PROPVARIANT v); void SetValue(ref PROPERTYKEY key, ref PROPVARIANT v); void Commit();
  }
  public static void Set(string lnk, string id) {
    object link = new CShellLink();
    ((IPersistFile)link).Load(lnk, 2); // STGM_READWRITE
    PROPERTYKEY key = new PROPERTYKEY { fmtid = new Guid("9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3"), pid = 5 };
    PROPVARIANT v = new PROPVARIANT { vt = 31, p = Marshal.StringToCoTaskMemUni(id) }; // VT_LPWSTR
    try {
      IPropertyStore store = (IPropertyStore)link;
      store.SetValue(ref key, ref v);
      store.Commit();
    } finally { Marshal.FreeCoTaskMem(v.p); }
    ((IPersistFile)link).Save(lnk, true);
  }
}
'@
[LinkAppId]::Set($lnk, $aumid)
Write-Output "$Name shortcut: $lnk ($aumid)"
