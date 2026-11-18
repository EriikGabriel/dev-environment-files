package icons

import (
	"encoding/json"
	"fmt"
	"strings"
)

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// shellIcon is C# that asks the shell for the icon of a file at any size,
// the way Explorer does (IShellItemImageFactory): it honours the
// IconLocation of a shortcut, has no shortcut arrow and keeps the alpha.
const shellIcon = `Add-Type -AssemblyName System.Drawing
Add-Type -TypeDefinition @'
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;
public static class ShellIcon {
  [StructLayout(LayoutKind.Sequential)] struct SIZE { public int cx, cy; }
  [StructLayout(LayoutKind.Sequential)] struct BITMAP { public int bmType, bmWidth, bmHeight, bmWidthBytes; public short bmPlanes, bmBitsPixel; public IntPtr bmBits; }
  [ComImport, Guid("bcc18b79-ba16-442f-80c4-8a59c30c463b"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
  interface IShellItemImageFactory { [PreserveSig] int GetImage(SIZE size, int flags, out IntPtr phbm); }
  [DllImport("shell32.dll", CharSet = CharSet.Unicode, PreserveSig = false)]
  static extern void SHCreateItemFromParsingName(string path, IntPtr pbc, [In] ref Guid riid, [MarshalAs(UnmanagedType.Interface)] out IShellItemImageFactory item);
  [DllImport("gdi32.dll")] static extern bool DeleteObject(IntPtr h);
  [DllImport("gdi32.dll")] static extern int GetObject(IntPtr h, int cb, ref BITMAP bm);
  public static Bitmap Get(string path, int size) {
    Guid iid = new Guid("bcc18b79-ba16-442f-80c4-8a59c30c463b");
    IShellItemImageFactory f;
    SHCreateItemFromParsingName(path, IntPtr.Zero, ref iid, out f);
    IntPtr hbm;
    SIZE s = new SIZE { cx = size, cy = size };
    int hr = f.GetImage(s, 0x4, out hbm); // SIIGBF_ICONONLY
    if (hr != 0) throw new Exception("GetImage failed 0x" + hr.ToString("x"));
    try {
      BITMAP bm = new BITMAP();
      GetObject(hbm, Marshal.SizeOf(typeof(BITMAP)), ref bm);
      if (bm.bmBitsPixel != 32 || bm.bmBits == IntPtr.Zero) throw new Exception("unexpected bitmap format");
      int w = bm.bmWidth, h = Math.Abs(bm.bmHeight);
      byte[] buf = new byte[w * h * 4];
      Marshal.Copy(bm.bmBits, buf, 0, buf.Length);
      // the shell hands out premultiplied alpha; PNG wants straight alpha
      for (int i = 0; i < buf.Length; i += 4) {
        int a = buf[i + 3];
        if (a > 0 && a < 255) {
          buf[i] = (byte)Math.Min(255, buf[i] * 255 / a);
          buf[i + 1] = (byte)Math.Min(255, buf[i + 1] * 255 / a);
          buf[i + 2] = (byte)Math.Min(255, buf[i + 2] * 255 / a);
        }
      }
      Bitmap res = new Bitmap(w, h, PixelFormat.Format32bppArgb);
      BitmapData d = res.LockBits(new Rectangle(0, 0, w, h), ImageLockMode.WriteOnly, PixelFormat.Format32bppArgb);
      for (int y = 0; y < h; y++) {
        int row = bm.bmHeight > 0 ? h - 1 - y : y;
        Marshal.Copy(buf, row * w * 4, new IntPtr(d.Scan0.ToInt64() + y * d.Stride), w * 4);
      }
      res.UnlockBits(d);
      return res;
    } finally { DeleteObject(hbm); }
  }
}
'@ -ReferencedAssemblies System.Drawing
`

// ScanScript lists the shortcuts of the scopes whose target is an .exe,
// saves the icon each one shows as a PNG and writes scan.json in work. A
// shortcut that already points into icoDir is not asked again: its original
// icon is the one the manifest kept. Advertised shortcuts (no target) and
// uninstall/readme links are left out.
func ScanScript(work, manifest, icoDir string, scopes []string) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	b.WriteString(shellIcon)
	fmt.Fprintf(&b, "$work = %s\n$manifestPath = %s\n$icoDir = %s\n", psQuote(work), psQuote(manifest), psQuote(icoDir))
	b.WriteString(`$srcDir = Join-Path $work 'src'
New-Item -ItemType Directory -Force -Path $srcDir | Out-Null
$manifest = @{}
$byIco = @{}
if (Test-Path -LiteralPath $manifestPath) {
  $m = Get-Content -Raw -Encoding UTF8 -LiteralPath $manifestPath | ConvertFrom-Json
  foreach ($e in @($m.entries)) { $manifest[$e.path.ToLowerInvariant()] = $e; if ($e.ico) { $byIco[$e.ico.ToLowerInvariant()] = $e } }
}
$roots = New-Object System.Collections.ArrayList
function AddRoot($scope, $path, $recurse) { [void]$roots.Add([pscustomobject]@{ Scope = $scope; Path = $path; Recurse = $recurse }) }
`)
	for _, s := range scopes {
		switch s {
		case ScopeDesktop:
			b.WriteString("AddRoot 'desktop' ([Environment]::GetFolderPath('Desktop')) $false\nAddRoot 'desktop' ([Environment]::GetFolderPath('CommonDesktopDirectory')) $false\n")
		case ScopeStartMenu:
			b.WriteString("AddRoot 'start-menu' (Join-Path $env:ProgramData 'Microsoft\\Windows\\Start Menu\\Programs') $true\nAddRoot 'start-menu' (Join-Path $env:APPDATA 'Microsoft\\Windows\\Start Menu\\Programs') $true\n")
		case ScopeTaskbar:
			b.WriteString("AddRoot 'taskbar' (Join-Path $env:APPDATA 'Microsoft\\Internet Explorer\\Quick Launch\\User Pinned\\TaskBar') $false\n")
		}
	}
	b.WriteString(`$explorerName = '(?i)^(File Explorer|Explorador de Arquivos)( \(\d+\))?$'
$skipName = '(?i)uninstall|desinstal|readme|leia-?me|\bhelp\b|ajuda|documenta|manual|web ?site|licen[sc]|release notes|changelog'
$sh = New-Object -ComObject WScript.Shell
$sha = [System.Security.Cryptography.SHA1]::Create()
$out = New-Object System.Collections.ArrayList
$seen = @{}
foreach ($r in $roots) {
  if (-not $r.Path -or -not (Test-Path -LiteralPath $r.Path)) { continue }
  $files = Get-ChildItem -LiteralPath $r.Path -Filter *.lnk -File -Recurse:$r.Recurse -ErrorAction SilentlyContinue
  foreach ($f in $files) {
    $key = $f.FullName.ToLowerInvariant()
    if ($seen[$key] -or $f.BaseName -match $skipName) { continue }
    $seen[$key] = $true
    try { $l = $sh.CreateShortcut($f.FullName) } catch { continue }
    $t = $l.TargetPath
    # the File Explorer shortcut points at a shell folder, not at a file
    if (-not $t -and $f.BaseName -match $explorerName) { $t = Join-Path $env:windir 'explorer.exe' }
    if (-not $t -or [IO.Path]::GetExtension($t) -ne '.exe') { continue }
    $id = (($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($key)) | ForEach-Object { $_.ToString('x2') }) -join '').Substring(0, 12)
    $png = Join-Path $srcDir ($id + '.png')
    $themed = $false
    $m = $manifest[$key]
    $ours = $l.IconLocation.StartsWith($icoDir, [StringComparison]::OrdinalIgnoreCase)
    # a shortcut pinned after its icon was changed is a copy that points at
    # the same .ico: its original is the one of the shortcut it was copied from
    if ($ours -and -not $m) { $m = $byIco[($l.IconLocation -replace ',-?\d+$', '').ToLowerInvariant()] }
    if ($m -and $ours -and (Test-Path -LiteralPath $m.src)) {
      $themed = $true
      $png = $m.src
    } else {
      try { $bmp = [ShellIcon]::Get($f.FullName, 256); $bmp.Save($png, [System.Drawing.Imaging.ImageFormat]::Png); $bmp.Dispose() } catch { continue }
    }
    [void]$out.Add([pscustomobject]@{ id = $id; name = $f.BaseName; scope = $r.Scope; path = $f.FullName; target = $t; themed = $themed; src = $png })
  }
}
$json = ConvertTo-Json -InputObject @($out) -Compress -Depth 3
[IO.File]::WriteAllText((Join-Path $work 'scan.json'), $json, (New-Object System.Text.UTF8Encoding $false))
`)
	return b.String()
}

// Job is what the elevated apply script reads.
type Job struct {
	Manifest string      `json:"manifest"`
	Entries  []JobEntry  `json:"entries"`
	System   []JobSystem `json:"system"`
}

// JobEntry is one shortcut to point at a new icon.
type JobEntry struct {
	Path string `json:"path"`
	Ico  string `json:"ico"`
	Src  string `json:"src"` // PNG of the original icon, kept for the next scan
}

// JobSystem is one registry value to point at a new icon ("file,0").
type JobSystem struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// JSON encodes the job.
func (j Job) JSON() []byte {
	if j.Entries == nil {
		j.Entries = []JobEntry{}
	}
	if j.System == nil {
		j.System = []JobSystem{}
	}
	b, _ := json.Marshal(j)
	return b
}

const refresh = `try { & ie4uinit.exe -show } catch { }
Stop-Process -Name 'explorer' -Force -ErrorAction SilentlyContinue
`

// ApplyScript runs elevated (the Start menu and the public desktop are under
// ProgramData): it points every shortcut of the job at its icon, sets the
// system icons and records the original of each in the manifest.
func ApplyScript(jobFile string) string {
	return `$ErrorActionPreference = 'Stop'
$job = Get-Content -Raw -Encoding UTF8 -LiteralPath ` + psQuote(jobFile) + ` | ConvertFrom-Json
$sh = New-Object -ComObject WScript.Shell
$old = @{}
$byIco = @{}
$sysOld = @{}
if (Test-Path -LiteralPath $job.manifest) {
  $m = Get-Content -Raw -Encoding UTF8 -LiteralPath $job.manifest | ConvertFrom-Json
  foreach ($e in @($m.entries)) { $old[$e.path.ToLowerInvariant()] = $e; if ($e.ico) { $byIco[$e.ico.ToLowerInvariant()] = $e } }
  foreach ($s in @($m.system)) { $sysOld[$s.key + '|' + $s.name] = $s }
}
$icoDir = $null
foreach ($e in @($job.entries)) { $icoDir = Split-Path -Parent $e.ico; break }
$done = 0; $failed = 0
foreach ($e in @($job.entries)) {
  if (-not (Test-Path -LiteralPath $e.path)) { continue }
  try {
    $l = $sh.CreateShortcut($e.path)
    $key = $e.path.ToLowerInvariant()
    $orig = $l.IconLocation
    if ($old.ContainsKey($key)) { $orig = $old[$key].orig }
    elseif ($icoDir -and $orig.StartsWith($icoDir, [StringComparison]::OrdinalIgnoreCase)) {
      # a copy of a shortcut that was already changed: its original is that one's
      $from = $byIco[($orig -replace ',-?\d+$', '').ToLowerInvariant()]
      $orig = if ($from) { $from.orig } else { ',0' }
    }
    $l.IconLocation = $e.ico + ',0'
    $l.Save()
    $old[$key] = [pscustomobject]@{ path = $e.path; orig = $orig; src = $e.src; ico = $e.ico }
    $done++
  } catch {
    $failed++
    Write-Output ('could not change ' + $e.path + ': ' + $_.Exception.Message)
  }
}
foreach ($s in @($job.system)) {
  $id = $s.key + '|' + $s.name
  $valueName = if ($s.name -eq '(default)') { '' } else { $s.name }
  if (-not $sysOld.ContainsKey($id)) {
    $prev = $null
    $item = Get-Item -LiteralPath $s.key -ErrorAction SilentlyContinue
    if ($item) { $prev = $item.GetValue($valueName) }
    $sysOld[$id] = [pscustomobject]@{ key = $s.key; name = $s.name; prev = $prev }
  }
  if (-not (Test-Path -LiteralPath $s.key)) { New-Item -Path $s.key -Force | Out-Null }
  Set-ItemProperty -LiteralPath $s.key -Name $s.name -Value $s.value
}
$dir = Split-Path -Parent $job.manifest
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$doc = [pscustomobject]@{ entries = @($old.Values); system = @($sysOld.Values) }
[IO.File]::WriteAllText($job.manifest, (ConvertTo-Json -InputObject $doc -Depth 4), (New-Object System.Text.UTF8Encoding $false))
# An icon that changes gets a new file name, because Windows keeps the picture
# of a path in its icon cache; the files nothing points at any more go away.
$used = @{}
foreach ($e in @($old.Values)) { if ($e.ico) { $used[$e.ico.ToLowerInvariant()] = $true } }
foreach ($s in @($job.system)) { $used[($s.value -replace ',-?\d+$', '').ToLowerInvariant()] = $true }
if ($icoDir) {
  Get-ChildItem -LiteralPath $icoDir -Filter *.ico -File -ErrorAction SilentlyContinue | Where-Object { -not $used[$_.FullName.ToLowerInvariant()] } | Remove-Item -Force -ErrorAction SilentlyContinue
}
Write-Output ('changed ' + $done + ' shortcuts, ' + $failed + ' failed')
` + refresh + "exit 0\n"
}
