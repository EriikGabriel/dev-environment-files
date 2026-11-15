// Package platform detects the operating system, distro family and CPU
// architecture the tool is running on.
package platform

import (
	"bufio"
	"io"
	"os"
	"runtime"
	"strings"
)

// Info describes the host.
type Info struct {
	OS     string   // runtime.GOOS: linux, windows, darwin
	Arch   string   // runtime.GOARCH: amd64, arm64
	ID     string   // os-release ID, e.g. ubuntu, arch, cachyos
	Like   []string // os-release ID_LIKE, e.g. [arch] or [ubuntu debian]
	Family string   // normalized family: debian, arch, windows, unknown
	Name   string   // PRETTY_NAME, for display
}

// Candidates returns the keys to look up distro overrides with, most
// specific first: the exact ID, then each ID_LIKE entry.
func (i Info) Candidates() []string {
	out := []string{i.ID}
	out = append(out, i.Like...)
	return out
}

// Detect inspects the running system.
func Detect() Info {
	info := Info{OS: runtime.GOOS, Arch: runtime.GOARCH}
	if info.OS == "linux" {
		for _, p := range []string{"/etc/os-release", "/usr/lib/os-release"} {
			if f, err := os.Open(p); err == nil {
				info = fromOSRelease(f, info)
				f.Close()
				break
			}
		}
	}
	info.Family = family(info)
	return info
}

// ParseOSRelease builds an Info from os-release content. Used by tests.
func ParseOSRelease(r io.Reader, goos, goarch string) Info {
	info := fromOSRelease(r, Info{OS: goos, Arch: goarch})
	info.Family = family(info)
	return info
}

func fromOSRelease(r io.Reader, info Info) Info {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"'`)
		switch k {
		case "ID":
			info.ID = strings.ToLower(v)
		case "ID_LIKE":
			info.Like = strings.Fields(strings.ToLower(v))
		case "PRETTY_NAME":
			info.Name = v
		}
	}
	return info
}

// family maps a distro (and its ID_LIKE chain) to a package-manager family.
func family(i Info) string {
	if i.OS == "windows" {
		return "windows"
	}
	for _, id := range i.Candidates() {
		switch id {
		case "arch", "manjaro", "endeavouros", "cachyos", "garuda":
			return "arch"
		case "debian", "ubuntu", "linuxmint", "pop", "zorin", "elementary":
			return "debian"
		}
	}
	return "unknown"
}
