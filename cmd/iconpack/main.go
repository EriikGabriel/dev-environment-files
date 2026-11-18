// Command iconpack downloads the MacTahoe icon theme and turns its app icons
// into the icon pack the repo ships in windows/icons/mactahoe: one 256 px PNG
// per app, aliases.json with the other names each icon answers to and the
// licence of the theme.
//
//	go run ./cmd/iconpack
//
// The theme is a Linux one: its icons are SVGs with gradients and filters that
// no Go library draws well, so they are drawn by Microsoft Edge (headless),
// many at a time on a sheet that is then cut into tiles. It only has to run
// again to take a newer version of the theme: the result is committed.
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	repo = "vinceliuice/MacTahoe-icon-theme"
	// commit pins the version of the theme the pack is made from.
	commit = "839848b9a8a38a92a6936e30c4abe35cc6f2546d"
	tile   = 256
	cols   = 8 // tiles per row and per column of a sheet
)

// system are the icons of the theme used for the Windows system items, under
// the names the icons package looks for.
var system = map[string]string{
	"src/devices/scalable/computer.svg":       "this-pc",
	"src/places/scalable/user-home.svg":       "user-files",
	"src/places/scalable/user-trash.svg":      "recycle-bin",
	"src/places/scalable/user-trash-full.svg": "recycle-bin-full",
	"src/places/scalable/network-server.svg":  "network",
	"src/places/scalable/folder.svg":          "folder",
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

func main() {
	out := flag.String("out", filepath.Join("windows", "icons", "mactahoe"), "folder of the pack")
	ref := flag.String("ref", commit, "commit of the theme")
	flag.Parse()
	if err := run(*out, *ref); err != nil {
		fmt.Fprintln(os.Stderr, "iconpack:", err)
		os.Exit(1)
	}
}

func run(out, ref string) error {
	edge, err := findEdge()
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "iconpack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	fmt.Println("downloading", repo, "at", ref[:min(12, len(ref))])
	resp, err := http.Get("https://codeload.github.com/" + repo + "/tar.gz/" + ref)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}
	svgs, links, docs, err := extract(resp.Body)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(svgs))
	for n := range svgs {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Printf("%d icons, %d other names\n", len(names), len(links))

	// The files Edge reads have numbers for names: nothing of the archive
	// decides a path on disk.
	for i, n := range names {
		if err := os.WriteFile(filepath.Join(work, fmt.Sprintf("%d.svg", i)), svgs[n], 0o600); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	per := cols * cols
	var empty []string
	for start := 0; start < len(names); start += per {
		end := min(start+per, len(names))
		fmt.Printf("drawing %d-%d of %d\n", start+1, end, len(names))
		sheet, err := drawSheet(edge, work, start, end)
		if err != nil {
			return err
		}
		for i := start; i < end; i++ {
			x, y := (i-start)%cols*tile, (i-start)/cols*tile
			im := image.NewNRGBA(image.Rect(0, 0, tile, tile))
			draw.Draw(im, im.Bounds(), sheet, image.Pt(x, y), draw.Src)
			if blank(im) {
				empty = append(empty, names[i])
				continue
			}
			f, err := os.Create(filepath.Join(out, names[i]+".png"))
			if err != nil {
				return err
			}
			enc := png.Encoder{CompressionLevel: png.BestCompression}
			if err := enc.Encode(f, im); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
	drawn := map[string]bool{}
	for _, n := range names {
		drawn[n] = true
	}
	for _, n := range empty {
		delete(drawn, n)
	}
	aliases := map[string]string{}
	for alias, target := range links {
		if drawn[target] && !drawn[alias] {
			aliases[alias] = target
		}
	}
	body, err := json.MarshalIndent(aliases, "", " ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "aliases.json"), append(body, '\n'), 0o644); err != nil {
		return err
	}
	for name, data := range docs {
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
	}
	readme := fmt.Sprintf(`MacTahoe icon pack
==================

App icons of the MacTahoe icon theme by vinceliuice, drawn as 256 px PNGs:
https://github.com/%s (commit %s).
The theme is under the GNU GPL v3: see COPYING and AUTHORS.

Made by "go run ./cmd/iconpack"; do not edit by hand. aliases.json lists the
other names each icon answers to (the symbolic links of the theme).
this-pc, user-files, recycle-bin, recycle-bin-full and network are the icons
of the Windows system items.
`, repo, ref)
	if err := os.WriteFile(filepath.Join(out, "README.txt"), []byte(readme), 0o644); err != nil {
		return err
	}
	fmt.Printf("%d icons and %d other names in %s\n", len(drawn), len(aliases), out)
	if len(empty) > 0 {
		fmt.Printf("%d came out empty and were left out: %s\n", len(empty), strings.Join(empty, ", "))
	}
	return nil
}

// extract reads the archive of the theme: the app icons (name -> SVG), the
// names that are links to them (alias -> name) and the licence files.
func extract(r io.Reader) (svgs map[string][]byte, links map[string]string, docs map[string][]byte, err error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, nil, nil, err
	}
	svgs, links, docs = map[string][]byte{}, map[string]string{}, map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		// drop the top folder GitHub adds
		_, p, ok := strings.Cut(h.Name, "/")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(path.Base(p), ".svg")
		switch {
		case h.Typeflag == tar.TypeReg && (p == "COPYING" || p == "AUTHORS"):
			if docs[p], err = io.ReadAll(io.LimitReader(tr, 1<<20)); err != nil {
				return nil, nil, nil, err
			}
		case h.Typeflag == tar.TypeReg && system[p] != "":
			if svgs[system[p]], err = io.ReadAll(io.LimitReader(tr, 4<<20)); err != nil {
				return nil, nil, nil, err
			}
		case path.Dir(p) == "src/apps/scalable" && strings.HasSuffix(p, ".svg") && safeName.MatchString(name):
			if h.Typeflag == tar.TypeReg {
				if svgs[name], err = io.ReadAll(io.LimitReader(tr, 4<<20)); err != nil {
					return nil, nil, nil, err
				}
			} else if h.Typeflag == tar.TypeSymlink {
				links[name] = strings.TrimSuffix(path.Base(h.Linkname), ".svg")
			}
		case path.Dir(p) == "links/apps/scalable" && h.Typeflag == tar.TypeSymlink && strings.HasSuffix(p, ".svg") && safeName.MatchString(name):
			links[name] = strings.TrimSuffix(path.Base(h.Linkname), ".svg")
		}
	}
	return svgs, links, docs, nil
}

func findEdge() (string, error) {
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		p := filepath.Join(os.Getenv(env), "Microsoft", "Edge", "Application", "msedge.exe")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("Microsoft Edge was not found: it draws the icons")
}

// drawSheet draws the icons start..end-1 on one transparent sheet. An SVG in
// an <img> runs no script and loads nothing from outside.
func drawSheet(edge, work string, start, end int) (image.Image, error) {
	var b strings.Builder
	b.WriteString(`<!doctype html><html><body style="margin:0;background:transparent;font-size:0;line-height:0;width:` + fmt.Sprint(cols*tile) + `px">`)
	for i := start; i < end; i++ {
		fmt.Fprintf(&b, `<img src="%d.svg" width="%d" height="%d" style="display:inline-block">`, i, tile, tile)
	}
	b.WriteString("</body></html>")
	page := filepath.Join(work, "sheet.html")
	if err := os.WriteFile(page, []byte(b.String()), 0o600); err != nil {
		return nil, err
	}
	shot := filepath.Join(work, "sheet.png")
	os.Remove(shot)
	side := fmt.Sprint(cols * tile)
	u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(page)}
	cmd := exec.Command(edge, "--headless=new", "--disable-gpu", "--no-first-run", "--hide-scrollbars",
		"--force-device-scale-factor=1", "--default-background-color=00000000",
		"--user-data-dir="+filepath.Join(work, "profile"),
		"--window-size="+side+","+side, "--screenshot="+shot, u.String())
	if outp, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("edge: %v: %s", err, outp)
	}
	f, err := os.Open(shot)
	if err != nil {
		return nil, fmt.Errorf("edge wrote no screenshot: %w", err)
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	if b := im.Bounds(); b.Dx() < cols*tile || b.Dy() < (end-start+cols-1)/cols*tile {
		return nil, fmt.Errorf("the screenshot is %dx%d, smaller than the sheet", b.Dx(), b.Dy())
	}
	return im, nil
}

func blank(im *image.NRGBA) bool {
	for i := 3; i < len(im.Pix); i += 4 {
		if im.Pix[i] != 0 {
			return false
		}
	}
	return true
}
