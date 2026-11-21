package policy

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
)

// polEntry is one entry as the Registry Policy File Format lays it out:
// [key\0;name\0;type;size;data], strings and separators in UTF-16LE.
func polEntry(key, name string, typ uint32, data []byte) []byte {
	return polEntryNul(key, name, typ, data, true)
}

// polEntryNul with nul false is how envsetup 0.1.0-0.1.7 wrote it.
func polEntryNul(key, name string, typ uint32, data []byte, nul bool) []byte {
	var b bytes.Buffer
	str := func(s string, nul bool) {
		for _, u := range utf16.Encode([]rune(s)) {
			binary.Write(&b, binary.LittleEndian, u)
		}
		if nul {
			b.Write([]byte{0, 0})
		}
	}
	sep := []byte{';', 0}
	b.Write([]byte{'[', 0})
	str(key, nul)
	b.Write(sep)
	str(name, nul)
	b.Write(sep)
	binary.Write(&b, binary.LittleEndian, typ)
	b.Write(sep)
	binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(sep)
	b.Write(data)
	b.Write([]byte{']', 0})
	return b.Bytes()
}

func dword(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

// The Registry.pol the apply script writes is the one Windows reads, and the
// state script reads it back: the
// strings end with a NUL (without it gpedit.msc and gpupdate failed with
// "end of file reached"), a file written by the earlier versions without the
// NULs is read and fixed, and gpt.ini names the registry extension and counts
// the change, even when it only had "[General]".
func TestRegistryPolIsWhatWindowsReads(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("runs the Windows PowerShell writer")
	}
	if _, err := exec.LookPath("powershell"); err != nil {
		t.Skip("no powershell")
	}
	root := t.TempDir()
	gp := filepath.Join(root, "System32", "GroupPolicy")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(gp, "Machine"), 0o755))
	must(os.WriteFile(filepath.Join(gp, "gpt.ini"), []byte("[General]\r\n"), 0o644))

	// a file of envsetup 0.1.0-0.1.7: no NUL before the ";"
	old := []byte("PReg\x01\x00\x00\x00")
	old = append(old, polEntryNul(`Software\Policies\Old`, "Kept", 4, dword(7), false)...)
	pol := filepath.Join(gp, "Machine", "Registry.pol")
	must(os.WriteFile(pol, old, 0o644))

	lib := strings.Replace(polLib, "Join-Path $env:SystemRoot ", "Join-Path '"+strings.ReplaceAll(root, "'", "''")+"' ", 1)
	if lib == polLib {
		t.Fatal("the library no longer finds the Registry.pol under $env:SystemRoot")
	}
	ps := func(body string) string {
		t.Helper()
		f := filepath.Join(t.TempDir(), "pol.ps1")
		must(os.WriteFile(f, []byte("$ErrorActionPreference = 'Stop'\n"+lib+body), 0o644))
		out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", f).CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	// what the state script sees: an old file is told apart, so the policies
	// are written again even when the registry already has their values
	check := `PolLoad
"$($script:polOld) $(PolGptOk) $(PolIs 'Software\Policies\Old' 'Kept' 7) $(PolIs 'Software\Policies\Test' 'A' 0) $(PolIsDel 'Software\Policies\Test' 'B') $(PolNone 'Software\Policies\Test' 'C')"
`
	if got := ps(check); got != "True False True False False True" {
		t.Errorf("old file: %q", got)
	}

	apply := `PolLoad
PolSet 'Software\Policies\Test' 'A' 1
PolSet 'Software\Policies\Test' 'A' 0
PolSet 'Software\Policies\Test' 'B' 1
PolSetDel 'Software\Policies\Test' 'B'
PolSave
`
	ps(apply)

	want := append([]byte("PReg\x01\x00\x00\x00"), polEntry(`Software\Policies\Old`, "Kept", 4, dword(7))...)
	want = append(want, polEntry(`Software\Policies\Test`, "A", 4, dword(0))...)
	// a deleted value is written as gpedit writes it: "**del.<name>", REG_SZ " "
	want = append(want, polEntry(`Software\Policies\Test`, "**del.B", 1, []byte{0x20, 0, 0, 0})...)
	got, err := os.ReadFile(pol)
	must(err)
	if !bytes.Equal(got, want) {
		t.Errorf("Registry.pol:\n got %x\nwant %x", got, want)
	}
	ini, err := os.ReadFile(filepath.Join(gp, "gpt.ini"))
	must(err)
	if s := string(ini); !strings.Contains(s, "gPCMachineExtensionNames=[{35378EAC-683F-11D2-A89A-00C04FBBCFA2}{D02B1F72-3407-48AE-BA88-E8213C6761F1}]") || !strings.Contains(s, "Version=1\r\n") {
		t.Errorf("gpt.ini:\n%s", s)
	}
	if got := ps(check); got != "False True True True True True" {
		t.Errorf("rewritten file: %q", got)
	}

	// applying again keeps the file and counts one more change
	ps(apply)
	got, _ = os.ReadFile(pol)
	ini, _ = os.ReadFile(filepath.Join(gp, "gpt.ini"))
	if !bytes.Equal(got, want) || !strings.Contains(string(ini), "Version=2\r\n") || strings.Count(string(ini), "35378EAC") != 1 {
		t.Errorf("second apply:\n%x\n%s", got, ini)
	}

	// setting a deleted value again drops the order to delete it
	ps("PolLoad\nPolSet 'Software\\Policies\\Test' 'B' 3\nPolSave\n")
	if got := ps("PolLoad\n\"$(PolIs 'Software\\Policies\\Test' 'B' 3) $(PolIsDel 'Software\\Policies\\Test' 'B')\"\n"); got != "True False" {
		t.Errorf("set after delete: %q", got)
	}
}
