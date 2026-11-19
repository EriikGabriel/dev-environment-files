package snapshot

// The personal part of a portable program (its settings, saves, playlists)
// is small and changes by hand, so it goes into git, in windows/<program>.
// Export copies it there with the secrets (passwords, API keys, tokens)
// blanked; the values removed are returned and kept in the store, next to
// the manifest, so a restore puts them back. Without a store a restore works
// too, and the program asks for those logins again.

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MaxProfileFile is the largest file Export takes into git.
const MaxProfileFile = 5 << 20

// Secret is a value Export left out of git.
type Secret struct {
	// File is relative to the program folder, with forward slashes.
	File string `json:"file"`
	// Key is the setting the value belongs to; Index is the position of the
	// value among the blanked ones of the file (a key can repeat in JSON).
	Key   string `json:"key"`
	Index int    `json:"index"`
	// Value is kept exactly as it was written in the file (JSON escapes
	// included).
	Value string `json:"value"`
}

// Exported tells what Export did.
type Exported struct {
	Files   int      `json:"files"`
	Bytes   int64    `json:"bytes"`
	Changed []string `json:"changed"`
	Removed []string `json:"removed"`
	// Skipped are files over MaxProfileFile, left out of git.
	Skipped []string `json:"skipped"`
	Secrets []Secret `json:"-"`
}

// Imported tells what Import did.
type Imported struct {
	Files   int `json:"files"`
	Changed int `json:"changed"`
	// Secrets is how many values came back from the store.
	Secrets int `json:"secrets"`
}

// splitPattern turns a pattern ("config", "ExtensionsData/*/config.json")
// into its segments.
func splitPattern(p string) []string {
	p = strings.Trim(strings.ReplaceAll(p, `\`, "/"), "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// MatchProfile tells whether rel (forward slashes) is one of the patterns
// or inside a folder one of them names. Case does not matter (Windows).
// A pattern that starts with "!" takes back what the others match
// ("library/*.db", "!library/games.db").
func MatchProfile(patterns []string, rel string) bool {
	segs := strings.Split(strings.ToLower(rel), "/")
	match := func(p string) bool {
		ps := splitPattern(strings.ToLower(p))
		if len(ps) == 0 || len(ps) > len(segs) {
			return false
		}
		for i, s := range ps {
			if m, _ := path.Match(s, segs[i]); !m {
				return false
			}
		}
		return true
	}
	found := false
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			if match(p[1:]) {
				return false
			}
		} else if match(p) {
			found = true
		}
	}
	return found
}

// Without leaves out of patterns the ones under the folders given
// ("library"): Import uses it to skip what the store already brought.
func Without(patterns []string, folders ...string) []string {
	var out []string
	for _, p := range patterns {
		ps := splitPattern(strings.TrimPrefix(p, "!"))
		skip := false
		for _, f := range folders {
			if len(ps) > 1 && strings.EqualFold(ps[0], f) {
				skip = true
			}
		}
		if !skip {
			out = append(out, p)
		}
	}
	return out
}

// TopLevel are the first segments of the patterns that have no wildcard:
// what a store copy can leave out because git carries it whole.
func TopLevel(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			continue
		}
		ps := splitPattern(p)
		if len(ps) == 1 && !strings.ContainsAny(ps[0], "*?[") {
			out = append(out, ps[0])
		}
	}
	return out
}

// ProfileFiles lists the files of dir the patterns take, relative, with
// forward slashes, sorted.
func ProfileFiles(dir string, patterns []string) []string {
	var out []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if MatchProfile(patterns, rel) {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// Export copies the files of src the patterns take to dst, blanking the
// secrets, and deletes from dst the ones src no longer has. Files whose
// content is the same are not written again.
func Export(src, dst string, patterns []string) (Exported, error) {
	res := Exported{Changed: []string{}, Removed: []string{}, Skipped: []string{}, Secrets: []Secret{}}
	keep := map[string]bool{}
	for _, rel := range ProfileFiles(src, patterns) {
		from := filepath.Join(src, filepath.FromSlash(rel))
		info, err := os.Stat(from)
		if err != nil {
			continue
		}
		if info.Size() > MaxProfileFile {
			res.Skipped = append(res.Skipped, rel)
			continue
		}
		raw, err := os.ReadFile(from)
		if err != nil {
			return res, err
		}
		clean, secrets := Scrub(rel, raw)
		res.Secrets = append(res.Secrets, secrets...)
		keep[strings.ToLower(rel)] = true
		res.Files++
		res.Bytes += int64(len(clean))
		to := filepath.Join(dst, filepath.FromSlash(rel))
		if old, err := os.ReadFile(to); err == nil && bytes.Equal(old, clean) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(to, clean, 0o644); err != nil {
			return res, err
		}
		res.Changed = append(res.Changed, rel)
	}
	for _, rel := range ProfileFiles(dst, patterns) {
		if keep[strings.ToLower(rel)] {
			continue
		}
		if err := os.Remove(filepath.Join(dst, filepath.FromSlash(rel))); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return res, err
		}
		res.Removed = append(res.Removed, rel)
	}
	removeEmptyDirs(dst)
	return res, nil
}

// Import copies the files of the git folder src the patterns take to the
// program folder dst and puts the secrets back in them.
func Import(src, dst string, patterns []string, secrets []Secret) (Imported, error) {
	return ImportWith(src, dst, patterns, secrets, false)
}

// ImportWith is Import; anonymous also blanks the account ids of the owner
// (see Anonymize), for someone else using the same settings.
func ImportWith(src, dst string, patterns []string, secrets []Secret, anonymous bool) (Imported, error) {
	var res Imported
	byFile := map[string][]Secret{}
	for _, s := range secrets {
		k := strings.ToLower(s.File)
		byFile[k] = append(byFile[k], s)
	}
	for _, rel := range ProfileFiles(src, patterns) {
		raw, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
		if err != nil {
			return res, err
		}
		n := 0
		if anonymous {
			raw = Anonymize(rel, raw)
		} else {
			raw, n = Unscrub(rel, raw, byFile[strings.ToLower(rel)])
		}
		res.Secrets += n
		res.Files++
		to := filepath.Join(dst, filepath.FromSlash(rel))
		if old, err := os.ReadFile(to); err == nil && bytes.Equal(old, raw) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(to, raw, 0o644); err != nil {
			return res, err
		}
		res.Changed++
	}
	return res, nil
}

func removeEmptyDirs(dir string) {
	var dirs []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && p != dir {
			dirs = append(dirs, p)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i]) // fails, and stays, when not empty
	}
}

// SecretsFile is where a store keeps the secrets of a program.
func SecretsFile(store, program string) string {
	return filepath.Join(store, "envsetup-"+program+"-secrets.json")
}

// ReadSecrets reads the secrets kept in a store; none when there is no file.
func ReadSecrets(file string) ([]Secret, error) {
	var s []Secret
	if _, err := ReadJSON(file, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(file), err)
	}
	return s, nil
}

// secretKey names settings that hold a password, a token or a key; notSecret
// takes back the ones that only describe them (expiry dates, types, paths,
// hotkeys), whose blank value a program could fail to read.
var (
	secretKey = regexp.MustCompile(`(?i)(api_?keys?|access_?token|refresh_?token|(^|_)token$|secret|passw(or)?d$|cookies?$|session_?(id|token)$)`)
	notSecret = regexp.MustCompile(`(?i)(expir[a-z]*|utc|type|path|mode|hotkey|enabled|date|time)$`)
)

// IsSecretKey tells whether a setting holds a secret.
func IsSecretKey(k string) bool { return secretKey.MatchString(k) && !notSecret.MatchString(k) }

// Scrub blanks the secrets of a settings file: JSON (.json) and
// key = "value" files (.cfg, .opt). Other files are returned as they are.
func Scrub(rel string, raw []byte) ([]byte, []Secret) {
	switch strings.ToLower(path.Ext(rel)) {
	case ".json":
		return scrubJSON(rel, raw)
	case ".cfg", ".opt":
		return scrubCfg(rel, raw)
	}
	return raw, nil
}

// Unscrub puts the secrets back in a file Scrub blanked; n is how many.
// A value is only put where the file still has it blank and the key is the
// same, so a file edited since keeps the edits.
func Unscrub(rel string, raw []byte, secrets []Secret) ([]byte, int) {
	if len(secrets) == 0 {
		return raw, 0
	}
	byIndex := map[int]Secret{}
	for _, s := range secrets {
		byIndex[s.Index] = s
	}
	switch strings.ToLower(path.Ext(rel)) {
	case ".json":
		sites := jsonSecretSites(raw)
		var b bytes.Buffer
		last, n := 0, 0
		for i, s := range sites {
			sec, ok := byIndex[i]
			if !ok || sec.Key != s.key || s.end != s.start {
				continue
			}
			b.Write(raw[last:s.start])
			b.WriteString(sec.Value)
			last = s.end
			n++
		}
		b.Write(raw[last:])
		return b.Bytes(), n
	case ".cfg", ".opt":
		lines := bytes.SplitAfter(raw, []byte("\n"))
		n := 0
		for i, line := range lines {
			k, v, ok := cfgLine(line)
			if !ok || v != "" {
				continue
			}
			for _, s := range secrets {
				if s.Key == k {
					lines[i] = []byte(strings.Replace(string(line), `""`, `"`+s.Value+`"`, 1))
					n++
					break
				}
			}
		}
		return bytes.Join(lines, nil), n
	}
	return raw, 0
}

var cfgRE = regexp.MustCompile(`^\s*([A-Za-z0-9_.\-]+)\s*=\s*"(.*)"\s*$`)

func cfgLine(line []byte) (key, value string, ok bool) {
	m := cfgRE.FindSubmatch(bytes.TrimRight(line, "\r\n"))
	if m == nil {
		return "", "", false
	}
	return string(m[1]), string(m[2]), true
}

func scrubCfg(rel string, raw []byte) ([]byte, []Secret) {
	var secrets []Secret
	lines := bytes.SplitAfter(raw, []byte("\n"))
	for i, line := range lines {
		k, v, ok := cfgLine(line)
		if !ok || v == "" || !IsSecretKey(k) {
			continue
		}
		secrets = append(secrets, Secret{File: rel, Key: k, Value: v})
		lines[i] = []byte(strings.Replace(string(line), `"`+v+`"`, `""`, 1))
	}
	return bytes.Join(lines, nil), secrets
}

// site is a string value of a JSON file that holds a secret: raw[start:end]
// is its content between the quotes.
type site struct {
	start, end int
	key        string
}

func scrubJSON(rel string, raw []byte) ([]byte, []Secret) {
	var secrets []Secret
	var b bytes.Buffer
	last := 0
	for i, s := range jsonSecretSites(raw) {
		if s.end == s.start {
			continue
		}
		secrets = append(secrets, Secret{File: rel, Key: s.key, Index: i, Value: string(raw[s.start:s.end])})
		b.Write(raw[last:s.start])
		last = s.end
	}
	if secrets == nil {
		return raw, nil
	}
	b.Write(raw[last:])
	return b.Bytes(), secrets
}

// jsonSecretSites scans JSON text (a BOM and invalid parts are tolerated)
// and returns, in order, the string values under a secret key: the value of
// the key itself, or every string inside it when it is an object or array.
// Working on the text keeps the rest of the file byte for byte.
func jsonSecretSites(raw []byte) []site { return jsonSites(raw, IsSecretKey) }

// jsonSites is jsonSecretSites with the keys chosen by isKey.
func jsonSites(raw []byte, isKey func(string) bool) []site {
	type frame struct {
		object bool
		secret bool
		key    string
		// wantKey is true in an object between "{" or "," and the key.
		wantKey bool
	}
	var sites []site
	stack := []frame{{}}
	top := func() *frame { return &stack[len(stack)-1] }
	valueSecret := func() (bool, string) {
		f := top()
		if f.object {
			return f.secret || isKey(f.key), f.key
		}
		return f.secret, f.key
	}
	i := 0
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		i = 3
	}
	for i < len(raw) {
		c := raw[i]
		switch c {
		case '{', '[':
			sec, key := valueSecret()
			stack = append(stack, frame{object: c == '{', secret: sec, key: key, wantKey: c == '{'})
			if c == '{' {
				top().key = ""
				top().secret = sec
			}
			i++
		case '}', ']':
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			i++
		case ',':
			if top().object {
				top().wantKey = true
			}
			i++
		case ':':
			top().wantKey = false
			i++
		case '"':
			start := i + 1
			j := start
			for j < len(raw) && raw[j] != '"' {
				if raw[j] == '\\' {
					j++
				}
				j++
			}
			if j > len(raw) {
				j = len(raw)
			}
			f := top()
			if f.object && f.wantKey {
				f.key = string(raw[start:j])
			} else if sec, key := valueSecret(); sec {
				sites = append(sites, site{start: start, end: j, key: key})
			}
			i = j + 1
		default:
			i++
		}
	}
	return sites
}

// Pending counts the files Export would write or delete in dst: the
// changes not in git yet.
func Pending(src, dst string, patterns []string) int {
	n := 0
	have := map[string]bool{}
	for _, rel := range ProfileFiles(src, patterns) {
		have[strings.ToLower(rel)] = true
		from := filepath.Join(src, filepath.FromSlash(rel))
		if info, err := os.Stat(from); err != nil || info.Size() > MaxProfileFile {
			continue
		}
		raw, err := os.ReadFile(from)
		if err != nil {
			continue
		}
		clean, _ := Scrub(rel, raw)
		if old, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel))); err != nil || !bytes.Equal(old, clean) {
			n++
		}
	}
	for _, rel := range ProfileFiles(dst, patterns) {
		if !have[strings.ToLower(rel)] {
			n++
		}
	}
	return n
}

// Git describes the personal part of a program kept in the repo.
type Git struct {
	// Dir is relative to the repo, with forward slashes.
	Dir   string `json:"dir"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
	// Pending counts the files of the program folder that differ from git
	// (a save copies them; then they need a commit).
	Pending int `json:"pending"`
}

// DescribeGit fills Git for the folder dir of the repo root and the
// program folder src.
func DescribeGit(root, dir, src string, patterns []string) Git {
	g := Git{Dir: filepath.ToSlash(dir)}
	full := filepath.Join(root, dir)
	for _, rel := range ProfileFiles(full, patterns) {
		if info, err := os.Stat(filepath.Join(full, filepath.FromSlash(rel))); err == nil {
			g.Files++
			g.Bytes += info.Size()
		}
	}
	if _, err := os.Stat(src); err == nil {
		g.Pending = Pending(src, full, patterns)
	}
	return g
}

// identityKey names settings that hold who the owner is on a service: user
// names and ids, account ids, nicknames, e-mail.
var identityKey = regexp.MustCompile(`(?i)(user_?name|user_?id|account_?(id|name)|steam_?id(64)?|nick_?name|e_?mail|_user)$`)

// IsIdentityKey tells whether a setting holds an account of the owner.
func IsIdentityKey(k string) bool { return identityKey.MatchString(k) }

// Anonymize blanks the account ids of a settings file (JSON, .cfg, .opt):
// the text values of IsIdentityKey keys (not true or false), so someone else gets the settings without the owner's accounts.
func Anonymize(rel string, raw []byte) []byte {
	// switches like settings_show_user = "true" are settings, not accounts
	keep := func(v string) bool { return v == "" || v == "true" || v == "false" }
	switch strings.ToLower(path.Ext(rel)) {
	case ".json":
		var b bytes.Buffer
		last := 0
		for _, s := range jsonSites(raw, IsIdentityKey) {
			if keep(string(raw[s.start:s.end])) {
				continue
			}
			b.Write(raw[last:s.start])
			last = s.end
		}
		b.Write(raw[last:])
		return b.Bytes()
	case ".cfg", ".opt":
		lines := bytes.SplitAfter(raw, []byte("\n"))
		for i, line := range lines {
			k, v, ok := cfgLine(line)
			if ok && IsIdentityKey(k) && !keep(v) {
				lines[i] = []byte(strings.Replace(string(line), `"`+v+`"`, `""`, 1))
			}
		}
		return bytes.Join(lines, nil)
	}
	return raw
}
