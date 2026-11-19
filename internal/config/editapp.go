package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SetAppName replaces the `names` of a catalogue app for one distro key. The
// entry must have its names on a line of their own with that key alone
// (names: { windows: ... }), and only that line is rewritten.
func SetAppName(appsPath, id, key, name string) error {
	if strings.ContainsAny(name, "\n\r") {
		return fmt.Errorf("the name must be a single line")
	}
	raw, err := os.ReadFile(appsPath)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("%s: %w", appsPath, err)
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("%s: empty document", appsPath)
	}
	_, apps := mapChild(doc.Content[0], "apps")
	_, app := mapChild(apps, id)
	if app == nil {
		return fmt.Errorf("app %q is not in the catalogue", id)
	}
	k, v := mapChild(app, "names")
	lines := strings.Split(string(raw), "\n")
	if k == nil || v.Kind != yaml.MappingNode || v.Style&yaml.FlowStyle == 0 || len(v.Content) != 2 ||
		v.Content[0].Value != key || k.Line != v.Line || app.Line == k.Line || k.Line < 1 || k.Line > len(lines) ||
		!strings.HasSuffix(strings.TrimSpace(lines[k.Line-1]), "}") {
		return fmt.Errorf("%s: `names` of %s must be on a line of its own, as names: { %s: ... }, to be edited", appsPath, id, key)
	}
	value := strconv.Quote(name)
	if name != "" && flowScalar(name) == name {
		value = name
	}
	end := ""
	if strings.HasSuffix(lines[k.Line-1], "\r") {
		end = "\r" // a CRLF checkout keeps its line ending
	}
	lines[k.Line-1] = lines[k.Line-1][:k.Column-1] + "names: { " + key + ": " + value + " }" + end
	return writeAtomic(appsPath, []byte(strings.Join(lines, "\n")))
}
