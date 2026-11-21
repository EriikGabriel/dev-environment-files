package appupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		cur, latest string
		want        bool
	}{
		{"0.1.10", "v0.2.1", true},
		{"0.2.1", "v0.2.1", false},
		{"0.2.1", "0.2.0", false},
		{"0.1.9", "0.1.10", true}, // numbers, not text
		{"0.1.10", "0.1.9", false},
		{"1.0.0", "0.9.9", false},
		{"0.2.0-dev", "0.2.1", true},
		{" v0.2.1 ", "0.3.0", true},
	} {
		got, err := Newer(c.cur, c.latest)
		if err != nil || got != c.want {
			t.Errorf("Newer(%q, %q) = %v, %v; want %v", c.cur, c.latest, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "latest", "1.2", "v"} {
		if _, err := Newer(bad, "1.0.0"); err == nil {
			t.Errorf("%q must not be a version", bad)
		}
	}
}

func TestInstallerIsTheNSISOneOnWindowsOnly(t *testing.T) {
	// a release can carry the installer of an earlier build too (v0.2.0 did):
	// the one of its own version is the one to take
	r := Release{Version: "0.2.1", Assets: []Asset{
		{Name: "envsetup_0.2.0_x64-setup.exe"},
		{Name: "envsetup_0.2.1_x64_en-US.msi"}, {Name: "envsetup_0.2.1_amd64.deb"}, {Name: "install.ps1"},
		{Name: "envsetup_0.2.1_x64-setup.exe"},
	}}
	if a, ok := r.Installer("windows"); !ok || a.Name != "envsetup_0.2.1_x64-setup.exe" {
		t.Errorf("windows: %v %v", a, ok)
	}
	if _, ok := r.Installer("linux"); ok {
		t.Error("Linux installs from its packages, the app does not run them")
	}
	if _, ok := (Release{Assets: []Asset{{Name: "other-setup.exe"}}}).Installer("windows"); ok {
		t.Error("only envsetup's own installer is taken")
	}
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// A server that plays GitHub: the API says what the latest release is and the
// installer is under /dl/. The release names the server's own URL.
func fakeGitHub(t *testing.T, body []byte) Client {
	t.Helper()
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+Repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "GitHub wants a user agent", http.StatusForbidden)
			return
		}
		fmt.Fprintf(w, `{"tag_name":"v0.2.1","body":"notes ","html_url":"https://example/r","draft":false,"prerelease":false,
		"assets":[{"name":"envsetup_0.2.1_x64-setup.exe","size":%d,"browser_download_url":"%s/dl/envsetup_0.2.1_x64-setup.exe","digest":"%s"}]}`,
			len(body), base, sum(body))
	})
	mux.HandleFunc("/dl/envsetup_0.2.1_x64-setup.exe", func(w http.ResponseWriter, r *http.Request) { w.Write(body) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	base = srv.URL
	return Client{HTTP: srv.Client(), API: srv.URL, Downloads: srv.URL + "/dl/"}
}

func TestLatestAndDownload(t *testing.T) {
	body := []byte("MZ pretend installer")
	cl := fakeGitHub(t, body)

	r, err := cl.Latest(context.Background())
	if err != nil || r.Tag != "v0.2.1" || r.Version != "0.2.1" || r.Notes != "notes" {
		t.Fatalf("Latest: %+v %v", r, err)
	}
	a, ok := r.Installer("windows")
	if !ok {
		t.Fatal("no installer in the release")
	}
	dir := filepath.Join(t.TempDir(), "up")
	path, err := cl.Download(context.Background(), a, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(body) || filepath.Dir(path) != dir {
		t.Errorf("downloaded %q to %s", got, path)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.part")); len(left) != 0 {
		t.Errorf("a partial file was left: %v", left)
	}
}

func TestDownloadRefusesWhatDoesNotCheckOut(t *testing.T) {
	body := []byte("the real installer")
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/envsetup_0.2.1_x64-setup.exe", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("a tampered one!!!!")) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cl := Client{HTTP: srv.Client(), Downloads: srv.URL + "/dl/"}
	asset := func(mod func(*Asset)) Asset {
		a := Asset{Name: "envsetup_0.2.1_x64-setup.exe", Size: int64(len(body)), URL: srv.URL + "/dl/envsetup_0.2.1_x64-setup.exe", Digest: sum(body)}
		mod(&a)
		return a
	}
	for name, a := range map[string]Asset{
		"a file that is not the one the checksum is of": asset(func(*Asset) {}),
		"no checksum":                asset(func(a *Asset) { a.Digest = "" }),
		"a checksum of another kind": asset(func(a *Asset) { a.Digest = "md5:abc" }),
		"another host":               asset(func(a *Asset) { a.URL = "https://evil.example/envsetup_0.2.1_x64-setup.exe" }),
		"a path in the name":         asset(func(a *Asset) { a.Name = `..\x-setup.exe` }),
	} {
		dir := filepath.Join(t.TempDir(), "up")
		if p, err := cl.Download(context.Background(), a, dir); err == nil {
			t.Errorf("%s: downloaded %s", name, p)
		} else if files, _ := os.ReadDir(dir); len(files) != 0 {
			t.Errorf("%s: left %d file(s) behind", name, len(files))
		}
	}
	// the right size and checksum of the wrong bytes is the tampered case above;
	// a size that disagrees is refused too
	right := []byte("a tampered one!!!!")
	a := asset(func(a *Asset) { a.Digest = sum(right); a.Size = 3 })
	if _, err := cl.Download(context.Background(), a, filepath.Join(t.TempDir(), "up")); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Errorf("a wrong size must be refused: %v", err)
	}
}

func TestLatestRefusesDraftsAndErrors(t *testing.T) {
	for name, c := range map[string]struct {
		status int
		body   string
	}{
		"no release":   {404, ""},
		"rate limited": {403, ""},
		"draft":        {200, `{"tag_name":"v9.9.9","draft":true}`},
		"pre-release":  {200, `{"tag_name":"v9.9.9","prerelease":true}`},
		"garbage":      {200, `<html>`},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			fmt.Fprint(w, c.body)
		}))
		cl := Client{HTTP: srv.Client(), API: srv.URL}
		if r, err := cl.Latest(context.Background()); err == nil {
			t.Errorf("%s: got %+v", name, r)
		}
		srv.Close()
	}
}
