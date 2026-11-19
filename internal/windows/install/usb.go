package install

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/EriikGabriel/dev-environment-files/internal/windows/ninite"
)

// AutoInstall is one entry of Ventoy's auto_install plugin: the answer file
// to use when booting an image.
type AutoInstall struct {
	Image    string `json:"image"`
	Template string `json:"template"`
}

// Ventoy is the part of ventoy.json this package reads.
type Ventoy struct {
	AutoInstall []AutoInstall `json:"auto_install"`
}

// ReadVentoy parses a ventoy.json.
func ReadVentoy(data []byte) (Ventoy, error) {
	var v Ventoy
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("ventoy.json: %w", err)
	}
	if v.AutoInstall == nil {
		v.AutoInstall = []AutoInstall{}
	}
	return v, nil
}

// Drive is the state of a Ventoy USB drive compared with the repo.
type Drive struct {
	Path string `json:"path"`
	// Images are the images ventoy.json installs unattended, and whether
	// each one is on the drive.
	Images []DriveImage `json:"images"`
	// Unattend is "missing", "current" (same as the repo) or "outdated".
	Unattend string `json:"unattend"`
	// Config is the same for ventoy.json: its auto_install entries are
	// missing, current or different from the repo's.
	Config string `json:"config"`
	// Ninite tells whether an installer is at the root of the drive.
	Ninite bool `json:"ninite"`
}

// DriveImage is an image referenced by ventoy.json.
type DriveImage struct {
	Path    string `json:"path"`
	Present bool   `json:"present"`
}

// IsVentoy reports whether dir looks like a Ventoy drive: it has the
// `ventoy` folder where the plugin configuration lives.
func IsVentoy(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, "ventoy"))
	return err == nil && fi.IsDir()
}

// FindDrives lists the mounted drives that look like Ventoy drives.
func FindDrives() []string {
	var cands []string
	if runtime.GOOS == "windows" {
		for c := 'A'; c <= 'Z'; c++ {
			cands = append(cands, string(c)+`:\`)
		}
	} else {
		for _, pat := range []string{"/run/media/*/*", "/media/*/*", "/media/*", "/mnt/*"} {
			m, _ := filepath.Glob(pat)
			cands = append(cands, m...)
		}
	}
	out := []string{}
	for _, d := range cands {
		if IsVentoy(d) {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}

// inside joins a drive root and a Ventoy path ("/ventoy/script/x.xml"),
// refusing paths that would leave the drive.
func inside(drive, p string) (string, error) {
	rel := filepath.Clean(filepath.FromSlash(strings.TrimLeft(p, `/\`)))
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%q is not a path inside the drive", p)
	}
	return filepath.Join(drive, rel), nil
}

// Inspect compares a drive with the repo's answer file and ventoy.json.
func Inspect(drive string, unattend, ventoy []byte) (Drive, error) {
	d := Drive{Path: drive, Images: []DriveImage{}, Unattend: "missing", Config: "missing"}
	cfg, err := ReadVentoy(ventoy)
	if err != nil {
		return d, err
	}
	states := map[string]bool{}
	for _, ai := range cfg.AutoInstall {
		if p, err := inside(drive, ai.Image); err == nil {
			_, statErr := os.Stat(p)
			d.Images = append(d.Images, DriveImage{Path: ai.Image, Present: statErr == nil})
		}
		p, err := inside(drive, ai.Template)
		if err != nil {
			continue
		}
		if have, err := os.ReadFile(p); err != nil {
			states["missing"] = true
		} else if bytes.Equal(have, unattend) {
			states["current"] = true
		} else {
			states["outdated"] = true
		}
	}
	switch {
	case states["outdated"]:
		d.Unattend = "outdated"
	case states["missing"] || len(states) == 0:
		d.Unattend = "missing"
	default:
		d.Unattend = "current"
	}
	if have, err := os.ReadFile(filepath.Join(drive, "ventoy", "ventoy.json")); err == nil {
		d.Config = "outdated"
		if merged, err := mergeVentoy(have, ventoy); err == nil && sameJSON(merged, have) {
			d.Config = "current"
		}
	}
	_, err = os.Stat(filepath.Join(drive, ninite.File))
	d.Ninite = err == nil
	return d, nil
}

// mergeVentoy puts every top-level key of the repo's ventoy.json into the
// drive's, keeping the keys only the drive has (themes, other plugins).
func mergeVentoy(drive, repo []byte) ([]byte, error) {
	var src map[string]json.RawMessage
	if err := json.Unmarshal(repo, &src); err != nil {
		return nil, fmt.Errorf("ventoy.json in the repo: %w", err)
	}
	dst := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(drive)) > 0 {
		if err := json.Unmarshal(drive, &dst); err != nil {
			dst = map[string]json.RawMessage{} // unreadable: replaced (the old file is backed up)
		}
	}
	for k, v := range src {
		dst[k] = v
	}
	out, err := json.MarshalIndent(dst, "", "    ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func sameJSON(a, b []byte) bool {
	var x, y bytes.Buffer
	return json.Compact(&x, a) == nil && json.Compact(&y, b) == nil && bytes.Equal(x.Bytes(), y.Bytes())
}

// Written is one file WriteDrive handled.
type Written struct {
	Path string `json:"path"`
	// Action is written, unchanged or downloaded.
	Action string `json:"action"`
	// Backup is where the previous content went, when it was replaced.
	Backup string `json:"backup,omitempty"`
}

// Fetch downloads a URL into a file.
type Fetch func(ctx context.Context, url, dest string) error

// WriteDrive copies the answer file to where ventoy.json expects it, merges
// ventoy.json into the drive's and, when slugs is not empty, downloads the
// Ninite installer to the drive root. A file that would change is backed up
// next to itself first.
func WriteDrive(ctx context.Context, drive string, unattend, ventoy []byte, slugs []string, fetch Fetch) ([]Written, error) {
	if !IsVentoy(drive) {
		return nil, fmt.Errorf("%s does not look like a Ventoy drive (it has no `ventoy` folder)", drive)
	}
	cfg, err := ReadVentoy(ventoy)
	if err != nil {
		return nil, err
	}
	out := []Written{}
	done := map[string]bool{}
	for _, ai := range cfg.AutoInstall {
		p, err := inside(drive, ai.Template)
		if err != nil {
			return out, fmt.Errorf("ventoy.json: template %w", err)
		}
		if done[p] {
			continue
		}
		done[p] = true
		w, err := replace(p, unattend)
		if err != nil {
			return out, err
		}
		out = append(out, w)
	}
	if len(done) == 0 {
		return out, fmt.Errorf("ventoy.json has no auto_install entry with a template, so there is nowhere to put the answer file")
	}

	target := filepath.Join(drive, "ventoy", "ventoy.json")
	have, _ := os.ReadFile(target)
	merged, err := mergeVentoy(have, ventoy)
	if err != nil {
		return out, err
	}
	if have != nil && sameJSON(merged, have) {
		out = append(out, Written{Path: target, Action: "unchanged"})
	} else {
		w, err := replace(target, merged)
		if err != nil {
			return out, err
		}
		out = append(out, w)
	}

	if len(slugs) > 0 {
		dest := filepath.Join(drive, ninite.File)
		part := dest + ".part"
		if err := fetch(ctx, ninite.Installer(slugs), part); err != nil {
			os.Remove(part)
			return out, fmt.Errorf("download the Ninite installer: %w", err)
		}
		if err := os.Rename(part, dest); err != nil {
			os.Remove(part)
			return out, err
		}
		out = append(out, Written{Path: dest, Action: "downloaded"})
	}
	return out, nil
}

// replace writes data to path unless it is already there, keeping the
// previous content in a .bak file.
func replace(path string, data []byte) (Written, error) {
	w := Written{Path: path, Action: "written"}
	have, err := os.ReadFile(path)
	if err == nil && bytes.Equal(have, data) {
		w.Action = "unchanged"
		return w, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return w, err
	}
	if err == nil {
		w.Backup = fmt.Sprintf("%s.envsetup-%d.bak", path, time.Now().Unix())
		if err := os.Rename(path, w.Backup); err != nil {
			return w, err
		}
	}
	return w, os.WriteFile(path, data, 0o644)
}

// Download is the real Fetch. It refuses anything that is not a Windows
// executable, so an error page never lands on the drive as an .exe.
func Download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "envsetup")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	head := make([]byte, 2)
	if _, err := io.ReadFull(resp.Body, head); err != nil || string(head) != "MZ" {
		return fmt.Errorf("%s did not return a Windows executable", url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err = f.Write(head); err == nil {
		_, err = io.Copy(f, resp.Body)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dest)
	}
	return err
}
