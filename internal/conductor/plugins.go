package conductor

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"komodo/internal/plugin"
	"komodo/internal/proc"
)

// pluginTimeout is how long one notifier or stage hook command may run.
const pluginTimeout = 2 * time.Minute

// What a notifier is told about.
const (
	EventBlocker = "blocker"
	EventSummary = "summary"
)

// ErrHookStopped is returned when a stage hook stops the group; its message carries the hook's reason.
var ErrHookStopped = errors.New("a stage hook stopped the group")

// Plugins are the plugins this machine enabled, which the conductor calls from the worktree Dir.
type Plugins struct {
	Dir     string
	Enabled []plugin.Plugin
}

// LoadPlugins reads root's plugins and keeps only those this machine enabled.
func LoadPlugins(root, dir string) (Plugins, error) {
	enabled, err := plugin.Enabled()
	if err != nil {
		return Plugins{Dir: dir}, err
	}
	loaded, _ := plugin.Load(root, enabled)
	plugins := Plugins{Dir: dir}
	for _, found := range loaded {
		if found.Enabled {
			plugins.Enabled = append(plugins.Enabled, found)
		}
	}
	return plugins, nil
}

// Notify sends a blocker note or run summary to every notifier, returning each failure as a note that decides nothing.
func (p Plugins) Notify(event, group, text string) []string {
	var failures []string
	for _, found := range p.of(plugin.Notifier) {
		env := pluginEnv(found, map[string]string{"EVENT": event, "GROUP": group, "TEXT": text})
		if result := proc.ShellEnv(p.Dir, found.Command, pluginTimeout, env); !result.OK() {
			failures = append(failures, fmt.Sprintf("notifier %s: %v: %s", found.Name, result.Err(), result.Output))
		}
	}
	return failures
}

// Tools lists the commands every tool pack adds to role's allow list, which the guard still checks.
func (p Plugins) Tools(role string) []string {
	var tools []string
	for _, found := range p.of(plugin.ToolPack) {
		if slices.Contains(found.Roles, role) {
			tools = append(tools, found.Tools...)
		}
	}
	return tools
}

// Hook runs every stage hook for stage at when, before or after, and stops at the first that exits non-zero.
func (p Plugins) Hook(when, stage, group string) error {
	for _, found := range p.of(plugin.StageHook) {
		if found.When != when || !slices.Contains(found.Stages, stage) {
			continue
		}
		env := pluginEnv(found, map[string]string{"WHEN": when, "STAGE": stage, "GROUP": group})
		if result := proc.ShellEnv(p.Dir, found.Command, pluginTimeout, env); !result.OK() {
			reason := result.Output
			if reason == "" {
				reason = result.Err().Error()
			}
			return fmt.Errorf("%w: %s %s %s: %s", ErrHookStopped, found.Name, when, stage, reason)
		}
	}
	return nil
}

// of lists the enabled plugins of one type.
func (p Plugins) of(kind plugin.Type) []plugin.Plugin {
	var out []plugin.Plugin
	for _, found := range p.Enabled {
		if found.Type == kind {
			out = append(out, found)
		}
	}
	return out
}

// pluginEnv is this process's environment plus KOMODO_<KEY> for each value and KOMODO_SETTING_<NAME> for each setting.
func pluginEnv(found plugin.Plugin, values map[string]string) []string {
	env := append(os.Environ(), "KOMODO_PLUGIN="+found.Name)
	for key, value := range values {
		env = append(env, "KOMODO_"+key+"="+value)
	}
	for name, value := range found.Settings {
		env = append(env, "KOMODO_SETTING_"+strings.ToUpper(name)+"="+value)
	}
	return env
}
