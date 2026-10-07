package claude

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/hooks"
	"komodo/internal/install"
	"komodo/internal/mount"
)

// BuilderPluginSkills returns the build skill and every standard skills already carries, the ones
// mount.SelectStandards kept for this repo, omitting review, planning and orchestrator skills.
func BuilderPluginSkills(skills []mount.Skill) []string {
	out := []string{"build"}
	seen := map[string]bool{"build": true}
	for _, skill := range skills {
		if strings.HasPrefix(skill.Name, "standards-") && !seen[skill.Name] {
			out = append(out, skill.Name)
			seen[skill.Name] = true
		}
	}
	sort.Strings(out)
	return out
}

// RenderBuilderPlugin adds the builder's plugin directory: a manifest and the build skill and its
// language standards. It returns their names, which the caller omits from the shared directory.
func RenderBuilderPlugin(plan *install.Plan, root string, skills []mount.Skill) map[string]bool {
	byName := map[string]mount.Skill{}
	for _, skill := range skills {
		byName[skill.Name] = skill
	}
	dir := filepath.Join(root, Dir, "plugins", "builder")
	plan.AddScoped(filepath.Join(dir, ".claude-plugin", "plugin.json"), pluginManifest("builder"),
		"the builder plugin's manifest")

	owned := map[string]bool{}
	for _, name := range BuilderPluginSkills(skills) {
		skill, ok := byName[name]
		if !ok {
			continue
		}
		plan.AddScoped(filepath.Join(dir, "skills", name, "SKILL.md"), []byte(skill.Body),
			"the "+name+" skill, scoped to the builder plugin")
		owned[name] = true
	}
	return owned
}

// RenderReviewerPlugin adds each lens skill to the reviewer's plugin and returns their names,
// which the caller omits from the shared directory.
func RenderReviewerPlugin(plan *install.Plan, root string, skills []mount.Skill) map[string]bool {
	dir := filepath.Join(root, Dir, "plugins", "reviewer", "skills")
	owned := map[string]bool{}
	for _, skill := range skills {
		if !strings.HasPrefix(skill.Name, "review-") {
			continue
		}
		plan.AddScoped(filepath.Join(dir, skill.Name, "SKILL.md"), []byte(skill.Body),
			"the "+skill.Name+" skill, scoped to the reviewer plugin")
		owned[skill.Name] = true
	}
	return owned
}

// pluginManifest is the minimal manifest a plugin directory needs to name itself.
func pluginManifest(name string) []byte {
	return []byte(fmt.Sprintf("{\n  \"name\": %q\n}\n", name))
}

// pluginRoles are the guard suites whose own plugin carries a hooks.json; the guard row is theirs
// already, through settings, so it is never repeated here.
var pluginRoles = []string{"builder", "reviewer"}

// RenderPluginHooks adds each role plugin's hooks file, carrying only the hooks its own guard suite mounts.
func RenderPluginHooks(plan *install.Plan, root, binary string) {
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(mount.MainCheckout(root), binary)
	}
	binary = mount.Publish(binary)
	for _, role := range pluginRoles {
		dir := filepath.Join(root, Dir, "plugins", role)
		if role != "builder" {
			plan.AddScoped(filepath.Join(dir, ".claude-plugin", "plugin.json"), pluginManifest(role),
				"the "+role+" plugin's manifest")
		}
		plan.AddScoped(filepath.Join(dir, "hooks", "hooks.json"), pluginHooks(binary, ownHooks(role)),
			"the "+role+" suite's own hooks")
	}
}

// ownHooks is a guard suite's own hooks, omitting the guard itself, which settings mounts.
func ownHooks(role string) []hooks.Hook {
	var out []hooks.Hook
	for _, hook := range hooks.ForSuite(role) {
		if hook.Name == "guard" {
			continue
		}
		out = append(out, hook)
	}
	return out
}

// profileMinuteCap bounds a session's wall clock, matching the 2 hours a builder's lease allows.
const profileMinuteCap = 120

// pluginHooks renders hooks as this host's plugin hooks file, grouped by stage in table order.
func pluginHooks(binary string, mounted []hooks.Hook) []byte {
	byEvent := map[string][]any{}
	for _, hook := range mounted {
		command := fmt.Sprintf("%s hook %s --host claude", binary, hook.Name)
		if hook.Budgeted {
			command += fmt.Sprintf(" --minutes %d --turns %d", profileMinuteCap, profileTurnCap)
		}
		entry := map[string]any{
			"hooks": []any{map[string]any{
				"type": "command", "command": command, "timeout": int(hook.Timeout.Seconds()),
			}},
		}
		switch hook.Tools {
		case hooks.EditTools:
			entry["matcher"] = editMatcher()
		case hooks.AnyTool:
			entry["matcher"] = "*"
		}
		byEvent[string(hook.Event)] = append(byEvent[string(hook.Event)], entry)
	}
	body, err := json.MarshalIndent(map[string]any{"hooks": byEvent}, "", "  ")
	if err != nil {
		return nil
	}
	return append(body, '\n')
}

// editMatcher joins the guard's registered write tools, so the format hook keeps no second list.
func editMatcher() string {
	names := make([]string, 0, len(guardWriteTools))
	for name := range guardWriteTools {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}

// hookOutcome renders a hook's outcome as the JSON this host reads: context after a tool, a block at a stop.
func hookOutcome(event hooks.Event, out hooks.Outcome) []byte {
	var payload map[string]any
	switch {
	case event == hooks.Stop && out.Verdict == hooks.Refuse:
		payload = map[string]any{"decision": "block", "reason": out.Message}
	case event == hooks.PostToolUse && out.Verdict == hooks.Inform:
		payload = map[string]any{
			"hookSpecificOutput": map[string]any{"hookEventName": string(event), "additionalContext": out.Message},
		}
	default:
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return data
}

// init registers how this host reads a hook's outcome, so the hooks never name it.
func init() {
	hooks.RegisterEncoder("claude", hookOutcome)
}
