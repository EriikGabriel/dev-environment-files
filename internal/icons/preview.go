package icons

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Entry is a shortcut with the decision made for it and, for the preview,
// the icon it has now and the one it would get.
type Entry struct {
	Shortcut
	From string `json:"from"`
	File string `json:"file,omitempty"`
	// Before and After are small PNGs as data URLs; After is empty for a
	// shortcut that is skipped or whose icon the preview cannot draw.
	Before string `json:"before"`
	After  string `json:"after"`
	Error  string `json:"error,omitempty"`
}

// PreviewSide is the size of the images in the preview.
const PreviewSide = 64

// Plan decides every shortcut and draws the preview of the result.
func Plan(cfg Config, root string, p *Pack, list []Shortcut) []Entry {
	out := make([]Entry, 0, len(list))
	for _, s := range list {
		c := Decide(cfg, root, p, s)
		e := Entry{Shortcut: s, From: c.From, File: c.File}
		if src, err := decodePNG(s.Src); err == nil {
			e.Before = DataURL(src, PreviewSide)
		}
		if c.From != FromSkip {
			if im, err := Image(c, s.Src, cfg.Style); err != nil {
				e.Error = err.Error()
			} else {
				e.After = DataURL(im, PreviewSide)
			}
		}
		out = append(out, e)
	}
	return out
}

// Written is an .ico made for a shortcut or a system item.
type Written struct {
	ID   string
	Path string
}

// stamp names the content of an icon. Windows caches the picture of an icon
// by its path, at every size it was asked for: an icon that changed under the
// same name shows the old picture at some sizes and the new one at others.
func stamp(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:4])
}

// WriteICOs writes the .ico of every shortcut that is not skipped into dir
// and returns the job for the apply script.
func WriteICOs(dir string, cfg Config, root string, p *Pack, list []Shortcut, manifest string) (Job, []string, error) {
	job := Job{Manifest: manifest}
	var problems []string
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return job, nil, err
	}
	for _, s := range list {
		c := Decide(cfg, root, p, s)
		if c.From == FromSkip {
			continue
		}
		data, err := ICOFile(c, s.Src, cfg.Style)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", s.Name, err))
			continue
		}
		ico := filepath.Join(dir, s.ID+"-"+stamp(data)+".ico")
		if err := os.WriteFile(ico, data, 0o644); err != nil {
			return job, problems, err
		}
		job.Entries = append(job.Entries, JobEntry{Path: s.Path, Ico: ico, Src: s.Src})
	}
	if cfg.SystemIcons {
		for _, si := range SystemIcons(p) {
			data, err := os.ReadFile(si.File)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", si.Item, err))
				continue
			}
			if filepath.Ext(si.File) != ".ico" {
				im, err := decodePNG(si.File)
				if err == nil {
					data, err = ICO(im)
				}
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s: %v", si.Item, err))
					continue
				}
			}
			name := "system-" + si.Item
			if si.Name != "(default)" {
				name += "-" + si.Name
			}
			ico := filepath.Join(dir, name+"-"+stamp(data)+".ico")
			if err := os.WriteFile(ico, data, 0o644); err != nil {
				return job, problems, err
			}
			job.System = append(job.System, JobSystem{Key: si.Key, Name: si.Name, Value: ico + ",0"})
		}
	}
	return job, problems, nil
}
