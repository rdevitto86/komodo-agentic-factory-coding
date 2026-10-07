// Package plugin loads the harness's plugins: notifiers, tool packs and stage hooks, each disabled until a machine enables it.
package plugin

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"

	"komodo/internal/toolkit"
)

// Type names the plugin point a manifest attaches to.
type Type string

// The three plugin types.
const (
	Notifier  Type = "notifier"
	ToolPack  Type = "tool-pack"
	StageHook Type = "stage-hook"
)

// Types lists every plugin type, in the order doctor reports them.
var Types = []Type{Notifier, ToolPack, StageHook}

// When a stage hook runs, relative to its stage.
const (
	Before = "before"
	After  = "after"
)

// ManifestName is the file naming a plugin, inside its folder under komodo/plugins.
const ManifestName = "plugin.json"

// Manifest is what a plugin's plugin.json declares.
type Manifest struct {
	Name     string            `json:"name"`
	Type     Type              `json:"type"`
	Roles    []string          `json:"roles"`
	Stages   []string          `json:"stages"`
	Settings map[string]string `json:"settings"`
	// Command is the shell command a notifier or stage hook runs.
	Command string `json:"command"`
	// Tools are the commands a tool pack adds to each of its roles' allow lists.
	Tools []string `json:"tools"`
	// When is before or after, for a stage hook.
	When string `json:"when"`
}

// Plugin is one loaded manifest, with whether this machine enabled it.
type Plugin struct {
	Manifest
	Enabled bool
}

// Problem is one manifest that did not load, named by its path under komodo.
type Problem struct {
	Where  string
	Detail string
}

// EnabledPath is the per-machine file naming the enabled plugins, or empty when there is no home.
func EnabledPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".komodo", "plugins.json")
}

// Enabled reads the names this machine enabled; a missing file enables none.
func Enabled() (map[string]bool, error) {
	enabled := map[string]bool{}
	where := EnabledPath()
	if where == "" {
		return enabled, nil
	}
	data, err := os.ReadFile(where)
	if os.IsNotExist(err) {
		return enabled, nil
	}
	if err != nil {
		return enabled, err
	}
	var parsed struct {
		Enabled []string `json:"enabled"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return enabled, fmt.Errorf("%s: %w", where, err)
	}
	for _, name := range parsed.Enabled {
		enabled[name] = true
	}
	return enabled, nil
}

// Load reads every plugin under the root's komodo/plugins, sorted by name, each enabled only when enabled names it.
func Load(root string, enabled map[string]bool) ([]Plugin, []Problem) {
	tree := toolkit.FS(root)
	manifests, _ := fs.Glob(tree, path.Join("plugins", "*", ManifestName))
	var plugins []Plugin
	var problems []Problem
	for _, name := range manifests {
		where := path.Join("komodo", name)
		data, err := fs.ReadFile(tree, name)
		if err != nil {
			problems = append(problems, Problem{where, err.Error()})
			continue
		}
		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			problems = append(problems, Problem{where, "is not JSON: " + err.Error()})
			continue
		}
		if detail := malformed(manifest, path.Base(path.Dir(name))); detail != "" {
			problems = append(problems, Problem{where, detail})
			continue
		}
		plugins = append(plugins, Plugin{Manifest: manifest, Enabled: enabled[manifest.Name]})
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	return plugins, problems
}

// malformed names what a manifest lacks for its type, or returns empty when it is whole.
func malformed(manifest Manifest, folder string) string {
	switch {
	case manifest.Name == "":
		return "names no plugin"
	case manifest.Name != folder:
		return "names " + manifest.Name + ", not its folder " + folder
	}
	switch manifest.Type {
	case Notifier:
		if manifest.Command == "" {
			return "a notifier needs a command"
		}
	case ToolPack:
		if len(manifest.Tools) == 0 || len(manifest.Roles) == 0 {
			return "a tool pack needs tools and roles"
		}
	case StageHook:
		if manifest.Command == "" || len(manifest.Stages) == 0 {
			return "a stage hook needs a command and stages"
		}
		if manifest.When != Before && manifest.When != After {
			return fmt.Sprintf("a stage hook runs before or after, not %q", manifest.When)
		}
	default:
		return fmt.Sprintf("type %q is not notifier, tool-pack or stage-hook", manifest.Type)
	}
	return ""
}
