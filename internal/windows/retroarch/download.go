package retroarch

// The program, its asset packs and the cores come from the official libretro
// buildbot, the same place the online updater of RetroArch uses.

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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

// Download saves url in file (see snapshot.Download).
func Download(ctx context.Context, url, file string, out io.Writer) error {
	return snapshot.Download(ctx, url, file, out)
}

// Unzip extracts a zip into dir (see snapshot.Unzip).
func Unzip(file, dir string) (files, skipped int, err error) { return snapshot.Unzip(file, dir) }
