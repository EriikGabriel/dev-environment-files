package platform

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// Details is the host summary shown by `envsetup info` and the desktop app.
// Fields that cannot be read are left empty.
type Details struct {
	OS             string   `json:"os"`
	Arch           string   `json:"arch"`
	ID             string   `json:"id"`
	Like           []string `json:"like"`
	Family         string   `json:"family"`
	Name           string   `json:"name"`
	Supported      bool     `json:"supported"`
	Hostname       string   `json:"hostname"`
	User           string   `json:"user"`
	Kernel         string   `json:"kernel"`
	Desktop        string   `json:"desktop"`
	Session        string   `json:"session"` // wayland or x11
	Shell          string   `json:"shell"`
	CPU            string   `json:"cpu"`
	Cores          int      `json:"cores"`
	MemoryGB       float64  `json:"memory_gb"`
	PackageManager string   `json:"package_manager"`
	AURHelper      string   `json:"aur_helper"`
	Flatpak        bool     `json:"flatpak"`
}

// Describe gathers the details for the detected host.
func Describe(i Info) Details {
	like := i.Like
	if like == nil {
		like = []string{}
	}
	d := Details{
		OS: i.OS, Arch: i.Arch, ID: i.ID, Like: like, Family: i.Family, Name: i.Name,
		Supported: i.Family == "debian" || i.Family == "arch",
		Desktop:   os.Getenv("XDG_CURRENT_DESKTOP"),
		Session:   os.Getenv("XDG_SESSION_TYPE"),
		Shell:     filepath.Base(os.Getenv("SHELL")),
	}
	if d.Shell == "." {
		d.Shell = ""
	}
	d.Hostname, _ = os.Hostname()
	if u, err := user.Current(); err == nil {
		d.User = u.Username
	}
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		d.Kernel = strings.TrimSpace(string(b))
	}
	d.CPU, d.Cores = cpuInfo()
	d.MemoryGB = memoryGB()

	switch i.Family {
	case "debian":
		d.PackageManager = "apt"
	case "arch":
		d.PackageManager = "pacman"
		for _, h := range []string{"yay", "paru"} {
			if has(h) {
				d.AURHelper = h
				break
			}
		}
	case "windows":
		d.PackageManager = "winget"
	}
	d.Flatpak = has("flatpak")
	return d
}

// Pairs renders the details as label/value rows, skipping empty values.
func (d Details) Pairs() [][2]string {
	rows := [][2]string{
		{"system", d.Name}, {"id", d.ID}, {"family", d.Family}, {"arch", d.Arch},
		{"kernel", d.Kernel}, {"host", d.Hostname}, {"user", d.User},
		{"desktop", strings.TrimSpace(d.Desktop + " " + d.Session)}, {"shell", d.Shell},
		{"cpu", d.CPU}, {"packages", strings.TrimSpace(d.PackageManager + " " + d.AURHelper)},
	}
	if d.MemoryGB > 0 {
		rows = append(rows, [2]string{"memory", fmt.Sprintf("%.1f GB", d.MemoryGB)})
	}
	out := rows[:0]
	for _, r := range rows {
		if r[1] != "" {
			out = append(out, r)
		}
	}
	return out
}

func has(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func cpuInfo() (model string, cores int) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "model name":
			if model == "" {
				model = strings.TrimSpace(v)
			}
		case "processor":
			cores++
		}
	}
	return model, cores
}

func memoryGB() float64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "MemTotal:"); ok {
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				if kb, err := strconv.ParseFloat(fields[0], 64); err == nil {
					return kb / 1024 / 1024
				}
			}
		}
	}
	return 0
}
