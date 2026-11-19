package retroarch

// The program, its asset packs and the cores come from the official libretro
// buildbot, the same place the online updater of RetroArch uses.

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/snapshot"
)

// Buildbot is the base of the downloads (a variable for the tests).
var Buildbot = "https://buildbot.libretro.com"

// ProgramURL is the portable RetroArch of a release (a 7z with one
// RetroArch-Win64 folder).
func ProgramURL(version string) string {
	return fmt.Sprintf("%s/stable/%s/windows/x86_64/RetroArch.7z", Buildbot, version)
}

// ProgramFolder is the folder inside the archive of ProgramURL.
const ProgramFolder = "RetroArch-Win64"

// BundleURL is an asset pack.
func BundleURL(name string) string {
	return fmt.Sprintf("%s/assets/frontend/%s.zip", Buildbot, name)
}

// CoreURL is a core of the nightly builds, what the core updater installs.
func CoreURL(core string) string {
	return fmt.Sprintf("%s/nightly/windows/x86_64/latest/%s_libretro.dll.zip", Buildbot, core)
}

// BundleDirs are where each asset pack goes, relative to the RetroArch folder.
var BundleDirs = map[string]string{
	"assets":           "assets",
	"autoconfig":       "autoconfig",
	"cheats":           "cheats",
	"database-rdb":     `database\rdb`,
	"database-cursors": `database\cursors`,
	"info":             "info",
	"overlays":         "overlays",
	"shaders_slang":    `shaders\shaders_slang`,
	"shaders_glsl":     `shaders\shaders_glsl`,
	"shaders_cg":       `shaders\shaders_cg`,
}

// BundleNames lists BundleDirs, sorted.
func BundleNames() []string {
	out := make([]string, 0, len(BundleDirs))
	for k := range BundleDirs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// CoreNames lists the cores of a RetroArch folder by file name, without
// _libretro.dll, sorted.
func CoreNames(dir string) []string {
	out := []string{}
	files, _ := filepath.Glob(filepath.Join(dir, "cores", "*_libretro.dll"))
	for _, f := range files {
		out = append(out, strings.TrimSuffix(filepath.Base(f), "_libretro.dll"))
	}
	sort.Strings(out)
	return out
}

var versionRE = regexp.MustCompile(`(?m)^Version:\s*([0-9][0-9A-Za-z.\-]*)`)

// ParseVersion reads the release out of `retroarch.exe --version`.
func ParseVersion(out string) string {
	if m := versionRE.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

var client = &http.Client{Timeout: 30 * time.Minute}

// Download saves url in file, writing the progress to out.
func Download(ctx context.Context, url, file string, out io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "envsetup")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	fmt.Fprintf(out, "downloaded %s (%.1f MB)\n", filepath.Base(file), float64(n)/(1<<20))
	return nil
}

// Unzip extracts a zip into dir. Entries whose name Windows refuses (a few
// cheat files) are skipped and counted.
func Unzip(file, dir string) (files, skipped int, err error) {
	r, err := zip.OpenReader(file)
	if err != nil {
		return 0, 0, err
	}
	defer r.Close()
	for _, f := range r.File {
		name := filepath.FromSlash(strings.ReplaceAll(f.Name, `\`, "/"))
		dst := filepath.Join(dir, name)
		if !snapshot.Inside(dst, dir) {
			skipped++
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(dst, 0o755)
			continue
		}
		if err := extract(f, dst); err != nil {
			skipped++
			continue
		}
		files++
	}
	return files, skipped, nil
}

func extract(f *zip.File, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, rc); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
