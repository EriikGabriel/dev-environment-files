// Package testutil has helpers shared by the tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// RepoRoot is the root of the repository (the folder with go.mod), so tests
// can read config/, dotfiles/ and windows/ wherever their package lives.
func RepoRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test's folder")
		}
		dir = parent
	}
}
