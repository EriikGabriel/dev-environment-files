package main

import (
	"os"
	"runtime"
	"strings"
)

// Every script of the engine runs in Windows PowerShell 5.1. Started from a
// PowerShell 7 terminal (or by an app started from one), envsetup inherits a
// PSModulePath that lists PowerShell 7's module folders first, and Windows
// PowerShell then autoloads PowerShell 7's own Microsoft.PowerShell.Security
// or Archive, which fail to load there: Get-AuthenticodeSignature,
// Expand-Archive and the like stop existing for the scripts.
func init() {
	if runtime.GOOS != "windows" {
		return
	}
	if v, ok := os.LookupEnv("PSModulePath"); ok {
		if clean := windowsPowerShellModulePath(v); clean != v {
			os.Setenv("PSModulePath", clean)
		}
	}
}

// windowsPowerShellModulePath drops PowerShell 7's module folders
// (Documents\PowerShell\Modules, Program Files\PowerShell\..., its Store
// package) from a PSModulePath, keeping the rest in order. Windows
// PowerShell's folders are named WindowsPowerShell and stay.
func windowsPowerShellModulePath(v string) string {
	var keep []string
	for _, p := range strings.Split(v, ";") {
		l := strings.ToLower(strings.ReplaceAll(p, "/", `\`))
		if strings.Contains(l, `\powershell\`) || strings.Contains(l, `\microsoft.powershell_`) {
			continue
		}
		keep = append(keep, p)
	}
	return strings.Join(keep, ";")
}
