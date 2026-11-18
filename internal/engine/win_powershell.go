package engine

import (
	"os"
	"path/filepath"
)

// Helpers shared by the Windows features: they all run PowerShell.

// writeScript saves a PowerShell script next to its work files. The BOM
// makes Windows PowerShell read it as UTF-8.
func writeScript(dir, name, body string) (string, error) {
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, body...), 0o600)
}

// psQuote quotes s as a PowerShell single-quoted string.
func psQuote(s string) string {
	b := []byte{'\''}
	for _, c := range []byte(s) {
		if c == '\'' {
			b = append(b, '\'')
		}
		b = append(b, c)
	}
	return string(append(b, '\''))
}
