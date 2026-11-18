package platform

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// maxShellURL is the longest link the Windows shell opens reliably
// (INTERNET_MAX_URL_LENGTH is 2083).
const maxShellURL = 2000

// OpenURL shows an https link in the default browser.
//
// The generator link of an answer file is around 9000 characters, more than
// the Windows shell accepts for a URL. Such links are opened through a small
// local page that redirects to them.
func OpenURL(link string) error {
	if !strings.HasPrefix(link, "https://") {
		return fmt.Errorf("only https links can be opened")
	}
	target := link
	if runtime.GOOS == "windows" && len(link) > maxShellURL {
		page := filepath.Join(os.TempDir(), "envsetup-open.html")
		if err := os.WriteFile(page, redirectPage(link), 0o600); err != nil {
			return err
		}
		target = page
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open the browser: %w", err)
	}
	go cmd.Wait()
	return nil
}

func redirectPage(link string) []byte {
	esc := html.EscapeString(link)
	return []byte(`<!doctype html><meta charset="utf-8"><title>envsetup</title>` +
		`<meta http-equiv="refresh" content="0;url=` + esc + `">` +
		`<p><a href="` + esc + `">Abrir a página</a></p>`)
}
