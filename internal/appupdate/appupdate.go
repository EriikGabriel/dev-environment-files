// Package appupdate finds out whether a newer release of the app is published
// and downloads its Windows installer, so the app can update itself instead of
// asking for a reinstall by hand.
//
// The latest release comes from the GitHub API. The installer is only taken
// from the release downloads of this repository and is only handed back when
// its SHA-256 matches the digest GitHub computed for the asset.
package appupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Repo is where the releases are published.
const Repo = "EriikGabriel/envsetup"

// Asset is a file of a release.
type Asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	// URL is where the file is downloaded from.
	URL string `json:"url"`
	// Digest is "sha256:<hex>", as GitHub computes it when the file is uploaded.
	Digest string `json:"-"`
}

// Release is the latest published release.
type Release struct {
	Tag     string
	Version string // the tag without the leading v
	Notes   string
	Page    string
	Assets  []Asset
}

// Client talks to GitHub. The zero value uses the real one.
type Client struct {
	HTTP *http.Client
	// API and Downloads are the base of the API and the prefix every asset URL
	// must start with; tests point them at a local server.
	API       string
	Downloads string
}

func (c Client) api() string {
	if c.API != "" {
		return strings.TrimRight(c.API, "/")
	}
	return "https://api.github.com"
}

func (c Client) downloads() string {
	if c.Downloads != "" {
		return c.Downloads
	}
	return "https://github.com/" + Repo + "/releases/download/"
}

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if req.URL.Scheme != "https" {
				return errors.New("refusing a redirect that leaves https")
			}
			return nil
		},
	}
}

type apiRelease struct {
	Tag        string `json:"tag_name"`
	Body       string `json:"body"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		URL    string `json:"browser_download_url"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

// Latest asks GitHub for the latest release (never a draft or pre-release).
func (c Client) Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.api()+"/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "envsetup-updater")
	resp, err := c.http().Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return Release{}, errors.New("no release is published yet")
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return Release{}, errors.New("GitHub is limiting the requests of this network; try again in a while")
	case resp.StatusCode != http.StatusOK:
		return Release{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var r apiRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&r); err != nil {
		return Release{}, fmt.Errorf("unreadable answer from GitHub: %w", err)
	}
	if r.Draft || r.Prerelease || r.Tag == "" {
		return Release{}, errors.New("no stable release is published yet")
	}
	out := Release{Tag: r.Tag, Version: strings.TrimPrefix(r.Tag, "v"), Notes: strings.TrimSpace(r.Body), Page: r.HTMLURL}
	for _, a := range r.Assets {
		out.Assets = append(out.Assets, Asset{Name: a.Name, Size: a.Size, URL: a.URL, Digest: a.Digest})
	}
	return out, nil
}

var versionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

// Newer reports whether latest is a higher version than current. Anything after
// the three numbers (a pre-release or build suffix) is ignored.
func Newer(current, latest string) (bool, error) {
	c, err := parse(current)
	if err != nil {
		return false, err
	}
	l, err := parse(latest)
	if err != nil {
		return false, err
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i], nil
		}
	}
	return false, nil
}

func parse(v string) ([3]int, error) {
	m := versionRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return [3]int{}, fmt.Errorf("%q is not a version like 1.2.3", v)
	}
	var out [3]int
	for i := range out {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return out, err
		}
		out[i] = n
	}
	return out, nil
}

// Installer picks the file the app installs itself from on goos: the NSIS
// installer of this release's own version on Windows (a release can carry the
// files of an earlier build too, and the .msi is for managed installs). Other
// systems install from a package manager, so there is none.
func (r Release) Installer(goos string) (Asset, bool) {
	if goos != "windows" {
		return Asset{}, false
	}
	for _, a := range r.Assets {
		if strings.HasPrefix(a.Name, "envsetup_"+r.Version+"_") && strings.HasSuffix(a.Name, "-setup.exe") {
			return a, true
		}
	}
	return Asset{}, false
}

var digestRe = regexp.MustCompile(`^sha256:([0-9a-fA-F]{64})$`)

// Dir is where the installer is downloaded to.
func Dir() string { return filepath.Join(os.TempDir(), "envsetup-update") }

// Download fetches the asset into dir, checks its size and SHA-256 and returns
// the path of the file. A file that does not match is deleted.
func (c Client) Download(ctx context.Context, a Asset, dir string) (string, error) {
	if !strings.HasPrefix(a.URL, c.downloads()) {
		return "", fmt.Errorf("refusing to download %q: it is not a file of the releases of %s", a.URL, Repo)
	}
	m := digestRe.FindStringSubmatch(a.Digest)
	if m == nil {
		return "", errors.New("the release has no checksum for the installer: it will not be run")
	}
	want := strings.ToLower(m[1])
	name := filepath.Base(a.Name)
	if name != a.Name || name == "." || strings.ContainsAny(name, `\/:`) {
		return "", fmt.Errorf("unsafe file name %q", a.Name)
	}

	// an earlier download, finished or not, is not wanted any more
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	part := filepath.Join(dir, name+".part")
	final := filepath.Join(dir, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "envsetup-updater")
	resp, err := c.http().Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(part)
		return "", fmt.Errorf("download failed: %w", err)
	}
	if a.Size > 0 && n != a.Size {
		_ = os.Remove(part)
		return "", fmt.Errorf("the download has %d bytes, the release says %d", n, a.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		_ = os.Remove(part)
		return "", errors.New("the download does not match its checksum: it was discarded")
	}
	if err := os.Rename(part, final); err != nil {
		return "", err
	}
	return final, nil
}
