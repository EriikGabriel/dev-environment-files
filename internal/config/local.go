package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Local is local.yaml at the root of the data folder: the choices of this
// machine, never committed (see .gitignore).
//
// Owner tells whether the person using the app is the owner of the
// configuration (the settings, saves and accounts in the repo are theirs).
// Someone else gets the same programs and settings, but not the owner's
// saves, play history and account ids, and no GitHub backup steps.
type Local struct {
	Owner *bool `yaml:"owner,omitempty"`
}

// LocalPath is where the file lives.
func LocalPath(root string) string { return filepath.Join(root, "local.yaml") }

// LoadLocal reads local.yaml; an empty Local when it is missing.
func LoadLocal(root string) (Local, error) {
	var l Local
	raw, err := os.ReadFile(LocalPath(root))
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, err
	}
	return l, yaml.Unmarshal(raw, &l)
}

// SaveLocal writes local.yaml.
func SaveLocal(root string, l Local) error {
	raw, err := yaml.Marshal(l)
	if err != nil {
		return err
	}
	head := []byte("# Choices of this machine (envsetup): never committed.\n# owner: true when the configuration in this folder is yours.\n")
	return os.WriteFile(LocalPath(root), append(head, raw...), 0o644)
}

// IsClone tells whether the data folder is a git clone.
func IsClone(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil
}

// IsOwner tells whether the user owns the configuration: what local.yaml
// says, else true for a git clone (the owner works in one) and false for the
// copy the installer makes.
func IsOwner(root string) (owner, explicit bool) {
	if l, err := LoadLocal(root); err == nil && l.Owner != nil {
		return *l.Owner, true
	}
	return IsClone(root), false
}
