package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CheckCacheTTL is how long an "is it installed?" answer is reused.
const CheckCacheTTL = 2 * time.Minute

const checkCacheFile = "check-cache.json"

// CheckCache keeps the answers of the slow install checks (a winget query, a
// PowerShell check script: each one a process) for a short while, across
// runs of envsetup. The app asks for the Configurar steps and then for the
// plan again at every click on an app; without it each of those checked
// everything once more. Only the read-only commands use it: apply always
// checks for real, and clears it (ClearCheckCache) since it changes the answers.
type CheckCache struct {
	path    string
	mu      sync.Mutex
	entries map[string]checkEntry
	dirty   bool
	now     func() time.Time
}

type checkEntry struct {
	Installed bool      `json:"installed"`
	Lines     []string  `json:"lines,omitempty"`
	At        time.Time `json:"at"`
}

// OpenCheckCache reads the cache kept in dir; a missing or unreadable file is
// an empty cache, and answers older than CheckCacheTTL are dropped.
func OpenCheckCache(dir string) *CheckCache {
	c := &CheckCache{path: filepath.Join(dir, checkCacheFile), entries: map[string]checkEntry{}, now: time.Now}
	if raw, err := os.ReadFile(c.path); err == nil {
		var saved map[string]checkEntry
		if json.Unmarshal(raw, &saved) == nil {
			for k, e := range saved {
				if c.fresh(e) {
					c.entries[k] = e
				}
			}
		}
	}
	return c
}

func (c *CheckCache) fresh(e checkEntry) bool {
	age := c.now().Sub(e.At)
	return age >= 0 && age < CheckCacheTTL
}

// answer returns the cached answer for key, or runs check and keeps its answer.
func (c *CheckCache) answer(key string, check func() bool) bool {
	if c == nil {
		return check()
	}
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok && c.fresh(e) {
		return e.Installed
	}
	v := check()
	c.mu.Lock()
	c.entries[key] = checkEntry{Installed: v, At: c.now()}
	c.dirty = true
	c.mu.Unlock()
	return v
}

// lines is answer for a list read from the machine (the graphics cards, the
// connected devices: the device list alone takes seconds to read).
func (c *CheckCache) lines(key string, read func() []string) []string {
	if c == nil {
		return read()
	}
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok && c.fresh(e) && e.Lines != nil {
		return e.Lines
	}
	v := read()
	if v == nil {
		v = []string{}
	}
	c.mu.Lock()
	c.entries[key] = checkEntry{Lines: v, At: c.now()}
	c.dirty = true
	c.mu.Unlock()
	return v
}

// Save writes the cache when something new was checked.
func (c *CheckCache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	raw, err := json.Marshal(c.entries)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	c.dirty = false
	return os.Rename(tmp, c.path)
}

// ClearCheckCache forgets every answer kept in dir.
func ClearCheckCache(dir string) {
	_ = os.Remove(filepath.Join(dir, checkCacheFile))
}

// checkKey names one check: the same app checked the same way in the same
// data folder (scripts see it as $env:ENVSETUP_ROOT).
func (s *appStep) checkKey() string {
	return s.env.Root + "\x00" + s.res.Method + "\x00" + s.res.Name + "\x00" + s.res.Check
}
