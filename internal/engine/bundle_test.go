package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/EriikGabriel/dev-environment-files/internal/testutil"
)

// The installer carries config/, dotfiles/ and windows/ (bundle.resources
// in app/src-tauri/tauri.conf.json) and the app only runs the subcommands of
// ALLOWED in lib.rs. These tests keep a release from shipping without a
// script the engine calls or with a button the app is not allowed to run.

func TestInstallerCarriesTheScripts(t *testing.T) {
	root := testutil.RepoRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, "app", "src-tauri", "tauri.conf.json"))
	must(t, err)
	var conf struct {
		Bundle struct {
			Resources map[string]string `json:"resources"`
		} `json:"bundle"`
	}
	must(t, json.Unmarshal(raw, &conf))
	for _, dir := range []string{"config", "dotfiles", "windows"} {
		if conf.Bundle.Resources["../../"+dir+"/"] != "data/"+dir+"/" {
			t.Errorf("the installer does not carry %s/ (bundle.resources: %v)", dir, conf.Bundle.Resources)
		}
	}

	// every windows\*.ps1 named by the engine or the catalogue exists
	scripts := map[string]string{}
	goRe := regexp.MustCompile(`"windows",\s*"([A-Za-z0-9._-]+\.ps1)"`)
	files, _ := filepath.Glob(filepath.Join(root, "internal", "engine", "*.go"))
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range goRe.FindAllSubmatch(b, -1) {
			scripts[string(m[1])] = filepath.Base(f)
		}
	}
	yamlRe := regexp.MustCompile(`windows\\([A-Za-z0-9._-]+\.ps1)`)
	b, err := os.ReadFile(filepath.Join(root, "config", "apps.yaml"))
	must(t, err)
	for _, m := range yamlRe.FindAllSubmatch(b, -1) {
		scripts[string(m[1])] = "config/apps.yaml"
	}
	if len(scripts) < 5 {
		t.Fatalf("too few scripts found, the patterns are out of date: %v", scripts)
	}
	for name, from := range scripts {
		if _, err := os.Stat(filepath.Join(root, "windows", name)); err != nil {
			t.Errorf("%s names windows/%s, which does not exist", from, name)
		}
	}
}

// Windows PowerShell 5.1 reads a .ps1 without a BOM as ANSI: a UTF-8 accent
// or Chinese character turns into other characters, some of them quotes,
// and the script fails to parse before it runs a single line.
func TestWindowsScriptsReadInWindowsPowerShell(t *testing.T) {
	root := testutil.RepoRoot(t)
	n := 0
	err := filepath.WalkDir(filepath.Join(root, "windows"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".ps1") {
			return err
		}
		n++
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(string(b), "\xEF\xBB\xBF") {
			return nil
		}
		for i, c := range b {
			if c > 0x7F {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s has a non-ASCII byte at %d and no UTF-8 BOM: Windows PowerShell misreads it", rel, i)
				break
			}
		}
		return nil
	})
	must(t, err)
	if n < 5 {
		t.Fatalf("too few scripts found under windows/: %d", n)
	}
}

func TestAppRunsOnlyAllowedCommands(t *testing.T) {
	root := testutil.RepoRoot(t)
	lib, err := os.ReadFile(filepath.Join(root, "app", "src-tauri", "src", "lib.rs"))
	must(t, err)
	start := strings.Index(string(lib), "ALLOWED")
	if start < 0 {
		t.Fatal("ALLOWED not found in lib.rs")
	}
	end := strings.Index(string(lib)[start:], "];")
	allowed := map[string]bool{}
	for _, m := range regexp.MustCompile(`"([a-z][a-z0-9-]*)"`).FindAllStringSubmatch(string(lib)[start:start+end], -1) {
		allowed[m[1]] = true
	}
	api, err := os.ReadFile(filepath.Join(root, "app", "src", "api.ts"))
	must(t, err)
	// run<T>(root, ["subcommand", ...])
	calls := regexp.MustCompile(`run<[^(]*\(root,\s*\[\s*"([a-z][a-z0-9-]*)"`).FindAllStringSubmatch(string(api), -1)
	if len(calls) < 30 || len(allowed) < 30 {
		t.Fatalf("too few commands found (%d calls, %d allowed): the patterns are out of date", len(calls), len(allowed))
	}
	for _, m := range calls {
		if !allowed[m[1]] {
			t.Errorf("api.ts runs %q, which is not in ALLOWED (lib.rs): the app would refuse it", m[1])
		}
	}
}
