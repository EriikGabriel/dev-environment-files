package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/EriikGabriel/dev-environment-files/internal/backend"
	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

func (e *Env) aptDir() string {
	if e.AptDir != "" {
		return e.AptDir
	}
	return "/etc/apt"
}

func (e *Env) repoList(id string) string {
	return filepath.Join(e.aptDir(), "sources.list.d", "envsetup-"+id+".list")
}

// repoPresent reports whether the app's apt repository was already added.
func (e *Env) repoPresent(id string) bool {
	_, err := os.Stat(e.repoList(id))
	return err == nil
}

// addRepo registers a third-party apt repository: the signing key goes to
// /etc/apt/keyrings and a one-line source, restricted to that key, to
// /etc/apt/sources.list.d. The package index is refreshed afterwards.
func (e *Env) addRepo(ctx context.Context, out io.Writer, id string, repo config.Repo) error {
	key, err := download(ctx, repo.Key, "envsetup-key-*")
	if err != nil {
		return err
	}
	defer os.Remove(key)
	head := make([]byte, 64)
	if f, err := os.Open(key); err == nil {
		n, _ := f.Read(head)
		head = head[:n]
		f.Close()
	}
	// apt tells armored and binary keys apart by the file extension.
	ext := ".gpg"
	if bytes.Contains(head, []byte("-----BEGIN")) {
		ext = ".asc"
	}
	keyring := filepath.Join(e.aptDir(), "keyrings", "envsetup-"+id+ext)

	line := fmt.Sprintf("deb [arch=%s signed-by=%s] %s\n", e.Info.Arch, keyring, repo.Source)
	list, err := os.CreateTemp("", "envsetup-list-*")
	if err != nil {
		return err
	}
	defer os.Remove(list.Name())
	if _, err := list.WriteString(line); err != nil {
		list.Close()
		return err
	}
	list.Close()

	fmt.Fprintf(out, "adding apt repository: %s", line)
	for _, cmd := range [][]string{
		{"install", "-D", "-m", "0644", key, keyring},
		{"install", "-D", "-m", "0644", list.Name(), e.repoList(id)},
	} {
		if err := e.Runner.Run(ctx, out, backend.Priv(cmd...)...); err != nil {
			return err
		}
	}
	delete(e.refreshed, backend.Apt{}.Name()) // the index must see the new source
	return nil
}
