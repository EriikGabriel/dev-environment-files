package config

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// AppendFile adds a file entry to the `files:` list of a profile YAML,
// keeping the comments and the layout of the rest of the document.
func AppendFile(profilePath string, f File) error {
	raw, err := os.ReadFile(profilePath)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("%s: %w", profilePath, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s: expected a mapping at the top level", profilePath)
	}
	top := doc.Content[0]

	var files *yaml.Node
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "files" {
			files = top.Content[i+1]
		}
	}
	if out, ok := insertIntoEmpty(raw, files, f); ok {
		return writeAtomic(profilePath, out)
	}
	if files == nil {
		files = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		top.Content = append(top.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "files"}, files)
	}

	if out, ok := insertAfterLastEntry(raw, files, f); ok {
		return writeAtomic(profilePath, out)
	}

	entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle}
	add := func(k, v string) {
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
	}
	add("src", f.Src)
	add("dest", f.Dest)
	if f.Mode != "" {
		add("mode", f.Mode)
	}
	files.Content = append(files.Content, entry)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2) // match the repo's hand-written files, to keep git diffs small
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return writeAtomic(profilePath, buf.Bytes())
}

// writeAtomic goes through a temp file so a crash cannot leave a half-written profile.
func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// insertAfterLastEntry adds the entry as one new line after the last item of
// a block sequence whose items are single-line flow mappings (the style the
// profiles use). Editing the text keeps comments, blank lines and spacing
// byte-for-byte, which a YAML re-encode cannot do. It reports false when the
// layout is anything else, and the caller falls back to re-encoding.
func insertAfterLastEntry(raw []byte, files *yaml.Node, f File) ([]byte, bool) {
	if files.Kind != yaml.SequenceNode || files.Style&yaml.FlowStyle != 0 || len(files.Content) == 0 {
		return nil, false
	}
	last := files.Content[len(files.Content)-1]
	lines := strings.Split(string(raw), "\n")
	i := last.Line - 1
	if last.Kind != yaml.MappingNode || last.Style&yaml.FlowStyle == 0 || i < 0 || i >= len(lines) {
		return nil, false
	}
	line := lines[i]
	dash := strings.Index(line, "- ")
	if dash < 0 || !strings.HasSuffix(strings.TrimSpace(line), "}") {
		return nil, false // multi-line entry or an unexpected shape
	}

	entry := entryLine(line[:dash], f)

	lines = append(lines[:i+1], append([]string{entry}, lines[i+1:]...)...)
	return []byte(strings.Join(lines, "\n")), true
}

// insertIntoEmpty handles a profile with no `files` key or with `files: []`,
// again by editing text so the rest of the file is untouched.
func insertIntoEmpty(raw []byte, files *yaml.Node, f File) ([]byte, bool) {
	text := string(raw)
	block := "files:\n" + entryLine("  ", f) + "\n"
	if files == nil {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return []byte(text + "\n" + block), true
	}
	if files.Kind != yaml.SequenceNode || len(files.Content) != 0 {
		return nil, false
	}
	lines := strings.Split(text, "\n")
	i := files.Line - 1
	if i < 0 || i >= len(lines) || strings.TrimSpace(lines[i]) != "files: []" {
		return nil, false
	}
	lines[i] = strings.TrimSuffix(block, "\n")
	return []byte(strings.Join(lines, "\n")), true
}

func entryLine(indent string, f File) string {
	parts := []string{"src: " + scalar(f.Src), "dest: " + scalar(f.Dest)}
	if f.Mode != "" {
		parts = append(parts, "mode: "+scalar(f.Mode))
	}
	return indent + "- { " + strings.Join(parts, ", ") + " }"
}

// scalar renders s as a YAML plain or quoted scalar.
func scalar(s string) string {
	b, err := yaml.Marshal(s)
	if err != nil {
		return strconv.Quote(s)
	}
	return strings.TrimSpace(string(b))
}

// SetDirs replaces the `dirs` list of a step (optionID "") or of one of its
// options. The list must already exist as a one-line flow sequence, which is
// how the profiles write it; only that line is rewritten.
func SetDirs(profilePath, stepID, optionID string, dirs []string) error {
	for _, d := range dirs {
		clean := strings.TrimRight(d, "/")
		if !strings.HasPrefix(clean, "~/") || strings.Contains(clean, "//") {
			return fmt.Errorf("invalid directory %q: it must start with ~/", d)
		}
		for _, part := range strings.Split(clean[2:], "/") {
			if part == "" || part == "." || part == ".." {
				return fmt.Errorf("invalid directory %q", d)
			}
		}
	}
	raw, err := os.ReadFile(profilePath)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("%s: %w", profilePath, err)
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("%s: empty document", profilePath)
	}
	child := func(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
		if m == nil || m.Kind != yaml.MappingNode {
			return nil, nil
		}
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == key {
				return m.Content[i], m.Content[i+1]
			}
		}
		return nil, nil
	}
	byID := func(seq *yaml.Node, id string) *yaml.Node {
		if seq == nil || seq.Kind != yaml.SequenceNode {
			return nil
		}
		for _, n := range seq.Content {
			if _, v := child(n, "id"); v != nil && v.Value == id {
				return n
			}
		}
		return nil
	}
	_, steps := child(doc.Content[0], "steps")
	owner := byID(steps, stepID)
	if owner == nil {
		return fmt.Errorf("step %q not found", stepID)
	}
	if optionID != "" {
		_, opts := child(owner, "options")
		if owner = byID(opts, optionID); owner == nil {
			return fmt.Errorf("step %q has no option %q", stepID, optionID)
		}
	}
	key, val := child(owner, "dirs")
	lines := strings.Split(string(raw), "\n")
	if key == nil || val.Kind != yaml.SequenceNode || val.Style&yaml.FlowStyle == 0 || key.Line != val.Line ||
		key.Line < 1 || key.Line > len(lines) {
		return fmt.Errorf("%s: `dirs` of %s must be written as a one-line list (dirs: [a, b]) to be edited", profilePath, stepID)
	}
	line := lines[key.Line-1]
	if !strings.HasSuffix(strings.TrimSpace(line), "]") {
		return fmt.Errorf("%s: `dirs` spans several lines; write it as dirs: [a, b]", profilePath)
	}
	items := make([]string, len(dirs))
	for i, d := range dirs {
		items[i] = scalar(strings.TrimRight(d, "/"))
	}
	lines[key.Line-1] = line[:key.Column-1] + "dirs: [" + strings.Join(items, ", ") + "]"
	return writeAtomic(profilePath, []byte(strings.Join(lines, "\n")))
}
