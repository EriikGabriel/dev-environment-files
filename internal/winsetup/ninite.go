// Package winsetup handles what a Windows machine needs before envsetup can
// run on it: the unattended-install answer file, the Ninite installer and the
// Ventoy USB drive that carries both.
package winsetup

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// NiniteHome is the site where a selection can be reviewed and changed.
const NiniteHome = "https://ninite.com/"

// niniteSlug is what one program looks like in a Ninite URL (chrome,
// paint.net, .net8, operaChromium). Slugs never contain "-", which is the
// separator.
var niniteSlug = regexp.MustCompile(`^[A-Za-z0-9.]+$`)

// ParseNinite turns what the user typed into a list of slugs. Every argument
// may be a slug, several slugs separated by spaces, commas or dashes, or a
// ninite.com link (https://ninite.com/7zip-steam/ or .../ninite.exe), so a
// selection made on the site can be pasted back. Duplicates are dropped and
// the order is kept.
func ParseNinite(args []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if strings.Contains(arg, "://") {
			u, err := url.Parse(arg)
			if err != nil || !strings.EqualFold(strings.TrimPrefix(u.Hostname(), "www."), "ninite.com") {
				return nil, fmt.Errorf("%q is not a ninite.com link", arg)
			}
			arg = ""
			for _, part := range strings.Split(u.Path, "/") {
				if part != "" && !strings.EqualFold(part, "ninite.exe") {
					arg = part
					break
				}
			}
		}
		for _, s := range strings.FieldsFunc(arg, func(r rune) bool { return r == '-' || r == ',' || r == ' ' || r == '\t' }) {
			if !niniteSlug.MatchString(s) {
				return nil, fmt.Errorf("%q is not a Ninite app name", s)
			}
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// NinitePage is the ninite.com page that shows the selection.
func NinitePage(slugs []string) string {
	if len(slugs) == 0 {
		return NiniteHome
	}
	return NiniteHome + strings.Join(slugs, "-") + "/"
}

// NiniteInstaller is the download link of the installer for the selection.
func NiniteInstaller(slugs []string) string {
	return NinitePage(slugs) + "ninite.exe"
}

// UnknownNinite returns the slugs that are not in NiniteCatalog. The site
// does not reject them (it just builds an installer without them), so the
// caller should tell the user.
func UnknownNinite(slugs []string) []string {
	known := map[string]bool{}
	for _, a := range NiniteCatalog {
		known[a.Slug] = true
	}
	out := []string{}
	for _, s := range slugs {
		if !known[s] {
			out = append(out, s)
		}
	}
	return out
}
