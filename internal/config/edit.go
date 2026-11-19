package config

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
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
	items := make([]string, len(dirs))
	for i, d := range dirs {
		items[i] = strings.TrimRight(d, "/")
	}
	return setStepList(profilePath, stepID, optionID, "dirs", items, false)
}

func mapChild(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
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

func seqByID(seq *yaml.Node, id string) *yaml.Node {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	for _, n := range seq.Content {
		if _, v := mapChild(n, "id"); v != nil && v.Value == id {
			return n
		}
	}
	return nil
}

// stepNode parses a profile and returns the mapping of a step, or of one of
// its options when optionID is set.
func stepNode(profilePath, stepID, optionID string) (raw []byte, owner *yaml.Node, err error) {
	raw, err = os.ReadFile(profilePath)
	if err != nil {
		return nil, nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", profilePath, err)
	}
	if len(doc.Content) == 0 {
		return nil, nil, fmt.Errorf("%s: empty document", profilePath)
	}
	_, steps := mapChild(doc.Content[0], "steps")
	if owner = seqByID(steps, stepID); owner == nil {
		return nil, nil, fmt.Errorf("step %q not found", stepID)
	}
	if optionID != "" {
		_, opts := mapChild(owner, "options")
		if owner = seqByID(opts, optionID); owner == nil {
			return nil, nil, fmt.Errorf("step %q has no option %q", stepID, optionID)
		}
	}
	return raw, owner, nil
}

// setStepList replaces a list (dirs, apps, optional) of a step or option. The
// list must be written on one line (key: [a, b]) and only that line changes.
// With create, a missing key is added as a new line below the step's first
// lines; without it, a missing key is an error.
func setStepList(profilePath, stepID, optionID, name string, items []string, create bool) error {
	raw, owner, err := stepNode(profilePath, stepID, optionID)
	if err != nil {
		return err
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = flowScalar(it)
	}
	text := name + ": [" + strings.Join(quoted, ", ") + "]"
	lines := strings.Split(string(raw), "\n")

	key, val := mapChild(owner, name)
	if key == nil {
		if !create {
			return fmt.Errorf("%s: step %s has no `%s` list to edit", profilePath, stepID, name)
		}
		// Anchor on a one-line key of the step, so the new line lands right
		// below it with the same indentation.
		for _, anchor := range []string{"apps", "description", "title", "id"} {
			k, v := mapChild(owner, anchor)
			if k == nil || k.Line != v.Line || k.Line < 1 || k.Line > len(lines) || v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
				continue
			}
			if v.Kind == yaml.SequenceNode && !strings.HasSuffix(strings.TrimSpace(lines[k.Line-1]), "]") {
				continue
			}
			line := strings.Repeat(" ", k.Column-1) + text
			lines = append(lines[:k.Line], append([]string{line}, lines[k.Line:]...)...)
			return writeAtomic(profilePath, []byte(strings.Join(lines, "\n")))
		}
		return fmt.Errorf("%s: could not find where to add `%s` in step %s", profilePath, name, stepID)
	}
	if val.Kind != yaml.SequenceNode || val.Style&yaml.FlowStyle == 0 || key.Line != val.Line ||
		key.Line < 1 || key.Line > len(lines) || !strings.HasSuffix(strings.TrimSpace(lines[key.Line-1]), "]") {
		return fmt.Errorf("%s: `%s` of %s must be written as a one-line list (%s: [a, b]) to be edited", profilePath, name, stepID, name)
	}
	lines[key.Line-1] = lines[key.Line-1][:key.Column-1] + text
	return writeAtomic(profilePath, []byte(strings.Join(lines, "\n")))
}

// flowScalar renders s for use inside a flow collection ([a, b] or {k: v}),
// where commas, brackets and colons would otherwise end a plain scalar.
func flowScalar(s string) string {
	if plainFlow.MatchString(s) && strings.Trim(s, "0123456789.") != "" {
		return s
	}
	return strconv.Quote(s)
}

var plainFlow = regexp.MustCompile(`^[A-Za-z0-9_~/]([A-Za-z0-9_./+~ -]*[A-Za-z0-9_./+~-])?$`)

// stepApps returns the `apps` and `optional` lists of a step.
func stepApps(profilePath, stepID string) (apps, optional []string, err error) {
	_, owner, err := stepNode(profilePath, stepID, "")
	if err != nil {
		return nil, nil, err
	}
	read := func(name string) []string {
		var out []string
		if _, v := mapChild(owner, name); v != nil && v.Kind == yaml.SequenceNode {
			for _, n := range v.Content {
				out = append(out, n.Value)
			}
		}
		return out
	}
	return read("apps"), read("optional"), nil
}

// AddStepApp puts an app in a step: in `apps` (starts selected) or, with
// optional, in `optional` (starts unselected).
func AddStepApp(profilePath, stepID, appID string, optional bool) error {
	apps, opt, err := stepApps(profilePath, stepID)
	if err != nil {
		return err
	}
	for _, id := range append(append([]string{}, apps...), opt...) {
		if id == appID {
			return fmt.Errorf("%q is already in step %q", appID, stepID)
		}
	}
	if optional {
		return setStepList(profilePath, stepID, "", "optional", append(opt, appID), true)
	}
	return setStepList(profilePath, stepID, "", "apps", append(apps, appID), true)
}

// RemoveStepApp takes an app out of a step's `apps` or `optional` list. The
// catalogue entry is kept.
func RemoveStepApp(profilePath, stepID, appID string) error {
	apps, opt, err := stepApps(profilePath, stepID)
	if err != nil {
		return err
	}
	without := func(list []string) ([]string, bool) {
		out, found := []string{}, false
		for _, id := range list {
			if id == appID {
				found = true
				continue
			}
			out = append(out, id)
		}
		return out, found
	}
	if rest, ok := without(apps); ok {
		return setStepList(profilePath, stepID, "", "apps", rest, false)
	}
	if rest, ok := without(opt); ok {
		return setStepList(profilePath, stepID, "", "optional", rest, false)
	}
	return fmt.Errorf("%q is not a toggleable item of step %q", appID, stepID)
}

var appID = regexp.MustCompile(`^[a-z0-9][a-z0-9+._-]*$`)

// AppendApp adds a catalogue entry at the end of apps.yaml, as one line, for a
// single distro key (windows, arch, debian). method "" means the native one.
func AppendApp(appsPath, id, key, method, name, description string) error {
	if !appID.MatchString(id) {
		return fmt.Errorf("invalid app id %q: use lowercase letters, digits, and - . _ +", id)
	}
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name+description, "\n\r") {
		return fmt.Errorf("the package name is required and must be a single line")
	}
	if method != "" && !validMethods[method] {
		return fmt.Errorf("unknown method %q", method)
	}
	raw, err := os.ReadFile(appsPath)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("%s: %w", appsPath, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s: expected a mapping at the top level", appsPath)
	}
	top := doc.Content[0]
	last := len(top.Content) - 2
	if last < 0 || top.Content[last].Value != "apps" || top.Content[last+1].Kind != yaml.MappingNode ||
		top.Content[last+1].Style&yaml.FlowStyle != 0 {
		return fmt.Errorf("%s: `apps` must be the last key, written as a block, to be edited", appsPath)
	}
	if _, exists := mapChild(top.Content[last+1], id); exists != nil {
		return fmt.Errorf("app %q already exists in the catalogue", id)
	}
	var parts []string
	if description != "" {
		parts = append(parts, "description: "+flowScalar(description))
	}
	if method != "" && method != MethodNative {
		parts = append(parts, fmt.Sprintf("method: { %s: %s }", key, method))
	}
	parts = append(parts, fmt.Sprintf("names: { %s: %s }", key, flowScalar(name)))
	text := strings.TrimRight(string(raw), "\n") + "\n  " + id + ": { " + strings.Join(parts, ", ") + " }\n"
	return writeAtomic(appsPath, []byte(text))
}

// SetDockPinned replaces desktop.dock.pinned. Like SetDirs it only rewrites
// that one line, which must be a one-line flow list.
func SetDockPinned(profilePath string, pinned []string) error {
	seen := map[string]bool{}
	for _, id := range pinned {
		if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "\n\r") || strings.HasSuffix(id, ".desktop") {
			return fmt.Errorf("invalid dock entry %q (use the id without .desktop)", id)
		}
		if seen[id] {
			return fmt.Errorf("duplicate dock entry %q", id)
		}
		seen[id] = true
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
	node := doc.Content[0]
	for _, k := range []string{"desktop", "dock"} {
		var next *yaml.Node
		for i := 0; node.Kind == yaml.MappingNode && i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == k {
				next = node.Content[i+1]
			}
		}
		if next == nil {
			return fmt.Errorf("%s: no desktop.dock section to edit", profilePath)
		}
		node = next
	}
	var key, val *yaml.Node
	for i := 0; node.Kind == yaml.MappingNode && i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "pinned" {
			key, val = node.Content[i], node.Content[i+1]
		}
	}
	lines := strings.Split(string(raw), "\n")
	if key == nil || val.Kind != yaml.SequenceNode || val.Style&yaml.FlowStyle == 0 || key.Line != val.Line ||
		key.Line < 1 || key.Line > len(lines) || !strings.HasSuffix(strings.TrimSpace(lines[key.Line-1]), "]") {
		return fmt.Errorf("%s: desktop.dock.pinned must be a one-line list (pinned: [a, b])", profilePath)
	}
	items := make([]string, len(pinned))
	for i, id := range pinned {
		items[i] = scalar(id)
	}
	line := lines[key.Line-1]
	lines[key.Line-1] = line[:key.Column-1] + "pinned: [" + strings.Join(items, ", ") + "]"
	return writeAtomic(profilePath, []byte(strings.Join(lines, "\n")))
}
