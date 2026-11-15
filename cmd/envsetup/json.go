package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/EriikGabriel/dev-environment-files/internal/engine"
	"github.com/EriikGabriel/dev-environment-files/internal/platform"
)

// The JSON shapes below are the contract with the desktop app (app/). Keep
// them stable; the TypeScript types in app/src/types.ts mirror them.

type platformJSON struct {
	OS     string   `json:"os"`
	Arch   string   `json:"arch"`
	ID     string   `json:"id"`
	Like   []string `json:"like"`
	Family string   `json:"family"`
	Name   string   `json:"name"`
}

func toPlatformJSON(i platform.Info) platformJSON {
	like := i.Like
	if like == nil {
		like = []string{}
	}
	return platformJSON{OS: i.OS, Arch: i.Arch, ID: i.ID, Like: like, Family: i.Family, Name: i.Name}
}

type profileJSON struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Environment string   `json:"environment"`
	Match       []string `json:"match"`
	Matches     bool     `json:"matches"` // the profile picked for this machine
}

type wizardJSON struct {
	Profile     string              `json:"profile"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Environment string              `json:"environment"`
	Platform    platformJSON        `json:"platform"`
	Steps       []engine.WizardStep `json:"steps"`
}

type planJSON struct {
	Platform platformJSON      `json:"platform"`
	Root     string            `json:"root"`
	Profile  string            `json:"profile"`
	Choices  map[string]string `json:"choices"`
	Steps    []engine.Line     `json:"steps"`
	Skipped  []engine.Skipped  `json:"skipped"`
}

// streamEvent is one NDJSON line printed by `apply --json`.
type streamEvent struct {
	Type string `json:"type"` // step | log | done
	// step
	Index  int    `json:"index,omitempty"`
	Total  int    `json:"total,omitempty"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status,omitempty"` // running | done | uptodate | failed
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
	Group  string `json:"group,omitempty"` // profile step id
	// DurationMs is set when a step finishes.
	DurationMs int64 `json:"duration_ms,omitempty"`
	// log: Index is the step that printed the line
	Line string `json:"line,omitempty"`
	// done
	Done     *int `json:"done,omitempty"`
	UpToDate *int `json:"up_to_date,omitempty"`
	Failed   *int `json:"failed,omitempty"`
	// LogFile is where the full text log of the run was saved.
	LogFile string `json:"log_file,omitempty"`
}

func statusName(s engine.Status) string {
	switch s {
	case engine.Running:
		return "running"
	case engine.Done:
		return "done"
	case engine.UpToDate:
		return "uptodate"
	}
	return "failed"
}

// ndjson serializes events from any goroutine onto one writer.
type ndjson struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func newNDJSON(w io.Writer) *ndjson { return &ndjson{enc: json.NewEncoder(w)} }

func (n *ndjson) emit(e streamEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enc.Encode(e)
}

// logLines turns command output into "log" events, one per line.
type logLines struct {
	out   *ndjson
	buf   string
	index int       // step currently running
	file  io.Writer // optional plain-text copy
}

func (w *logLines) Write(p []byte) (int, error) {
	w.buf += string(p)
	for {
		i := strings.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		line := w.buf[:i]
		w.buf = w.buf[i+1:]
		if j := strings.LastIndexByte(line, '\r'); j >= 0 { // progress bars
			line = line[j+1:]
		}
		if line = strings.TrimRight(line, " \t"); line != "" {
			w.out.emit(streamEvent{Type: "log", Index: w.index, Line: line})
			if w.file != nil {
				fmt.Fprintf(w.file, "    %s\n", line)
			}
		}
	}
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
