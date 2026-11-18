// Package icons gives the shortcuts of a Windows machine one icon style.
//
// Windows has no icon theme: the icon of an executable is inside the file
// and the shell shows it as it is. What can be changed from outside is the
// icon of each shortcut (.lnk) on the desktop, in the Start menu and pinned to
// the taskbar, and a few system icons (This PC, Recycle Bin...). This package
// finds those shortcuts, takes the icon each one shows now and either uses
// the icon the user's icon pack has for the app or draws one in a macOS style
// (the original icon on a rounded-square plate), then points the shortcut at
// it. The original icon location of every shortcut is kept to undo it.
package icons

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scopes of shortcuts.
const (
	ScopeDesktop   = "desktop"
	ScopeStartMenu = "start-menu"
	ScopeTaskbar   = "taskbar"
)

// Plate styles of the drawn icons.
const (
	StyleLight = "light"
	StyleDark  = "dark"
)

// Config is config/icons.yaml.
type Config struct {
	// Enabled is the choice of the app; apply-icons does nothing when off.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Style is the plate of the drawn icons: light or dark.
	Style string `yaml:"style" json:"style"`
	// Pack is a folder of .ico or .png files named after the apps (relative
	// to the repo, or absolute). An app with a file there gets it as it is.
	Pack string `yaml:"pack" json:"pack"`
	// Scopes are the places whose shortcuts are changed.
	Scopes []string `yaml:"scopes" json:"scopes"`
	// SystemIcons also sets This PC, Recycle Bin, User files and Network from
	// the pack (this-pc, recycle-bin, recycle-bin-full, user-files, network).
	SystemIcons bool `yaml:"system_icons" json:"system_icons"`
	// Skip are shortcut names that keep their icon.
	Skip []string `yaml:"skip" json:"skip"`
	// Overrides map a shortcut name to an image file for it.
	Overrides map[string]string `yaml:"overrides" json:"overrides"`
}

// Default is what a missing file means.
func Default() Config {
	return Config{Style: StyleLight, Scopes: []string{ScopeDesktop, ScopeStartMenu, ScopeTaskbar}, SystemIcons: true,
		Skip: []string{}, Overrides: map[string]string{}}
}

// Path is where the choice lives in the repo.
func Path(root string) string { return filepath.Join(root, "config", "icons.yaml") }

var scopeSet = map[string]bool{ScopeDesktop: true, ScopeStartMenu: true, ScopeTaskbar: true}

// Validate refuses what the scripts could not take safely.
func (c Config) Validate() error {
	if c.Style != StyleLight && c.Style != StyleDark {
		return fmt.Errorf("style must be %q or %q, got %q", StyleLight, StyleDark, c.Style)
	}
	for _, s := range c.Scopes {
		if !scopeSet[s] {
			return fmt.Errorf("unknown scope %q", s)
		}
	}
	for name, file := range c.Overrides {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(file) == "" {
			return fmt.Errorf("an override needs a shortcut name and a file")
		}
	}
	return nil
}

// Load reads the choice. A missing file is not an error: exists tells.
func Load(path string) (cfg Config, exists bool, err error) {
	cfg = Default()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return Default(), true, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Style == "" {
		cfg.Style = StyleLight
	}
	if cfg.Scopes == nil {
		cfg.Scopes = []string{}
	}
	if cfg.Skip == nil {
		cfg.Skip = []string{}
	}
	if cfg.Overrides == nil {
		cfg.Overrides = map[string]string{}
	}
	if err := cfg.Validate(); err != nil {
		return cfg, true, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, true, nil
}

const header = `# Icons of the shortcuts on the Windows desktop, in the Start menu and on the
# taskbar. Edited in the app's Preview tab or with ` + "`envsetup set-icons`" + `;
# ` + "`envsetup apply-icons`" + ` writes it to the machine and ` + "`restore-icons`" + ` undoes it.
#
# enabled       apply-icons does nothing while this is false
# style         plate of the drawn icons: light or dark
# pack          folder of .ico/.png files named after the apps (repo-relative or
#               absolute); an app with a file there gets it untouched. The
#               repo ships windows/icons/mactahoe, the macOS Tahoe icons
# scopes        desktop, start-menu, taskbar
# system_icons  also set This PC, Recycle Bin, User files and Network from the
#               pack (this-pc, recycle-bin, recycle-bin-full, user-files, network)
# skip          shortcut names that keep their icon
# overrides     shortcut name: image file
`

// Save writes the choice, replacing the file.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	body, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append([]byte(header+"\n"), body...), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Key is how names are compared: lower case, letters and digits only.
func Key(name string) string { return nonAlnum.ReplaceAllString(strings.ToLower(name), "") }
