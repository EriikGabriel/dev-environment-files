package engine

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

// DockItem is one pinned app of the dock preview.
type DockItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`  // data URL; empty when no icon was found
	Found bool   `json:"found"` // a desktop entry exists on this machine
}

// AppEntry is an installed application that can be pinned.
type AppEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DesktopPreview is everything a front-end needs to draw the desktop.
type DesktopPreview struct {
	Kind      string `json:"kind"`
	IconTheme string `json:"icon_theme"`
	Colors    struct {
		Accent string `json:"accent"`
		Panel  string `json:"panel"`
		Text   string `json:"text"`
	} `json:"colors"`
	Wallpaper       string `json:"wallpaper"` // data URL
	WallpaperSource string `json:"wallpaper_source"`
	WallpaperError  string `json:"wallpaper_error"`
	Dock            struct {
		Position string     `json:"position"`
		AutoHide bool       `json:"autohide"`
		Items    []DockItem `json:"items"`
	} `json:"dock"`
	Bar struct {
		Position string   `json:"position"`
		Start    []string `json:"start"`
		Center   []string `json:"center"`
		End      []string `json:"end"`
	} `json:"bar"`
	Apps []AppEntry `json:"apps"`
}

// Preview builds the desktop preview for a profile on this machine. Icons and
// names come from the desktop entries and icon theme that are installed here,
// so apps or themes that are not installed yet show up as placeholders.
func Preview(ctx context.Context, cfg *config.Config, profileID string, env *Env) (DesktopPreview, error) {
	prof, ok := cfg.Profiles[profileID]
	if !ok {
		return DesktopPreview{}, fmt.Errorf("unknown profile %q", profileID)
	}
	d := prof.Desktop
	var p DesktopPreview
	p.Kind, p.IconTheme = d.Kind, d.IconTheme
	p.Colors.Accent, p.Colors.Panel, p.Colors.Text = d.Colors.Accent, d.Colors.Panel, d.Colors.Text
	p.Dock.Position, p.Dock.AutoHide = orDefault(d.Dock.Position, "bottom"), d.Dock.AutoHide
	p.Bar.Position = orDefault(d.Bar.Position, "top")
	p.Bar.Start, p.Bar.Center, p.Bar.End = nz(d.Bar.Start), nz(d.Bar.Center), nz(d.Bar.End)

	p.Dock.Items = []DockItem{}
	for _, id := range d.Dock.Pinned {
		it := DockItem{ID: id, Name: id}
		if e, ok := findDesktop(env.Home, id); ok {
			it.Name, it.Found = e.Name, true
			if icon := findIcon(env.Home, d.IconTheme, e.Icon); icon != "" {
				it.Icon = dataURL(icon, maxIconBytes)
			}
		}
		p.Dock.Items = append(p.Dock.Items, it)
	}

	p.Apps = []AppEntry{}
	for _, e := range listDesktop(env.Home) {
		p.Apps = append(p.Apps, AppEntry{ID: e.ID, Name: e.Name})
	}

	if d.Wallpaper != "" {
		p.WallpaperSource = d.Wallpaper
		url, err := loadWallpaper(ctx, env, d.Wallpaper)
		p.Wallpaper = url
		if err != nil {
			p.WallpaperError = err.Error()
		}
	}
	return p, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func nz(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

const maxWallpaperBytes = 12 << 20

// loadWallpaper reads a repo, ~ or https wallpaper as a data URL. Downloads are
// cached under the user cache directory.
func loadWallpaper(ctx context.Context, env *Env, src string) (string, error) {
	var file string
	switch {
	case strings.HasPrefix(src, "https://"):
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		sum := sha1.Sum([]byte(src))
		ext := strings.ToLower(path.Ext(strings.SplitN(src, "?", 2)[0]))
		if ext == "" {
			ext = ".jpg"
		}
		file = filepath.Join(cache, "envsetup", "wallpapers", hex.EncodeToString(sum[:])+ext)
		if _, err := os.Stat(file); err != nil {
			if err := download2(ctx, src, file); err != nil {
				return "", fmt.Errorf("download wallpaper: %w", err)
			}
		}
	case strings.HasPrefix(src, "~/") || filepath.IsAbs(src):
		file = expandHome(src, env.Home)
	default:
		file = filepath.Join(env.Root, filepath.FromSlash(src))
	}
	url := dataURL(file, maxWallpaperBytes)
	if url == "" {
		return "", fmt.Errorf("wallpaper %s is missing, too big (over 12 MB) or not an image", src)
	}
	return url, nil
}

func download2(ctx context.Context, url, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxWallpaperBytes+1))
	f.Close()
	if err != nil || n > maxWallpaperBytes {
		os.Remove(tmp)
		if err == nil {
			err = fmt.Errorf("file is over 12 MB")
		}
		return err
	}
	return os.Rename(tmp, dest)
}
