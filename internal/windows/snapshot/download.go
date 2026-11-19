package snapshot

// Downloads of the programs a restore installs again (RetroArch, Playnite,
// Playnite add-ons) and the zips they come in.

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
		if !Inside(dst, dir) {
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
