package main

import "testing"

func TestWindowsPowerShellModulePath(t *testing.T) {
	in := `C:\Users\me\Documents\PowerShell\Modules;C:\Program Files\PowerShell\Modules;` +
		`c:\program files\windowsapps\microsoft.powershell_7.7.0.0_x64__8wekyb3d8bbwe\Modules;` +
		`C:\Program Files\PowerShell\7\Modules;D:\MyModules;` +
		`C:\Program Files\WindowsPowerShell\Modules;C:\WINDOWS\system32\WindowsPowerShell\v1.0\Modules`
	want := `D:\MyModules;C:\Program Files\WindowsPowerShell\Modules;C:\WINDOWS\system32\WindowsPowerShell\v1.0\Modules`
	if got := windowsPowerShellModulePath(in); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	clean := `C:\Program Files\WindowsPowerShell\Modules;C:\WINDOWS\system32\WindowsPowerShell\v1.0\Modules`
	if got := windowsPowerShellModulePath(clean); got != clean {
		t.Errorf("a clean path changed: %s", got)
	}
}
