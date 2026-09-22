// Package claude is the Claude Code mount: the only place this host's names appear.
package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/facet"
	"komodo/internal/install"
	"komodo/internal/mount"
	"komodo/internal/profile"
	repopkg "komodo/internal/repo"
)

// Dir is the host's project directory, relative to the repo root.
const Dir = ".claude"

// models maps a role's tier to this host's model, which is what a profile row sets.
var models = map[string]string{"light": "haiku", "standard": "sonnet", "heavy": "opus"}

// tools maps the five Komodo verbs to this host's tool names, here and nowhere else.
var tools = map[string][]string{
	"read": {"Read"}, "edit": {"Edit"}, "write": {"Write"},
	"shell": {"Bash"}, "search": {"Grep", "Glob"},
}

// retired are the V1 render and the old bridge entry, removed when present.
var retired = []string{
	filepath.Join(Dir, "hooks"),
	filepath.Join(Dir, "mcp.json"),
	filepath.Join(Dir, "commands"),
	".mcp.json",
}

// Render builds the plan that mounts this repo on Claude Code.
func Render(root string, binary string) (install.Plan, error) {
	plan := install.Plan{Host: "claude", Root: root}
	rules, err := mount.Rules(root)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, "CLAUDE.md"), []byte("@AGENTS.md\n"), "the host reads the repo's own rules")
	plan.AddProject(filepath.Join(root, Dir, "komodo", "AGENTS.md"), []byte(rules), "the universal rules, rendered")

	roles, err := mount.LoadRoles(root)
	if err != nil {
		return plan, err
	}
	ollama := profile.OllamaUp()
	for _, role := range roles {
		if !role.Session {
			continue
		}
		plan.Add(filepath.Join(root, Dir, "agents", role.Name+".md"), []byte(agentFile(role, ollama)), "the "+role.Name+" role as an agent")
	}

	skills, err := mount.LoadSkills(root)
	if err != nil {
		return plan, err
	}
	skills = repoSkills(root, skills)
	for _, skill := range skills {
		plan.AddProject(filepath.Join(root, Dir, "skills", skill.Name, "SKILL.md"), []byte(skill.Body), "the "+skill.Name+" skill")
	}

	for _, name := range facetSkills(root) {
		loaded, err := facet.Load(root, name)
		if err != nil {
			continue
		}
		plan.AddProject(filepath.Join(root, Dir, "skills", "facet-"+loaded.Name, "SKILL.md"),
			[]byte(loaded.Skill), "the "+loaded.Name+" facet's setup skill")
	}

	settings, err := settingsFile(root, binary)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, Dir, "settings.json"), settings, "the guard on PreToolUse and the permissions layer")
	plan.AddSeed(filepath.Join(root, Dir, "settings.local.json"), []byte("{\n  \"permissions\": {\n    \"allow\": []\n  }\n}\n"), "the personal overlay")
	plan.AddSeed(filepath.Join(root, "CLAUDE.local.md"), []byte("# Personal overlay\n\nYours. The install never overwrites this file.\n"), "the personal overlay")

	for _, path := range retired {
		full := filepath.Join(root, path)
		if path == ".mcp.json" && !namesBridge(full) {
			continue
		}
		plan.AddRemoval(full, "the V1 render and the old bridge entry")
	}
	return plan, nil
}

// repoSkills merges the repo's standards and skills overrides into the shipped set.
func repoSkills(root string, skills []mount.Skill) []mount.Skill {
	byName := map[string]int{}
	for i, skill := range skills {
		byName[skill.Name] = i
	}
	standards, _ := repopkg.LoadStandards(root)
	for _, override := range standards {
		mergeOverride(&skills, byName, "standards-"+override.Name, override.New, override.Body)
	}
	overrides, _ := repopkg.LoadSkills(root)
	for _, override := range overrides {
		mergeOverride(&skills, byName, override.Name, override.New, override.Body)
	}
	return skills
}

// facetSkills names the facets the detected profile and the repo's own additions select.
func facetSkills(root string) []string {
	names, err := facet.Select(root, detect.Load(root), nil)
	if err != nil {
		return nil
	}
	return names
}

// mergeOverride appends a new skill, or a "Repo overrides" section onto a shipped one by name.
func mergeOverride(skills *[]mount.Skill, byName map[string]int, name string, isNew bool, body string) {
	if isNew {
		byName[name] = len(*skills)
		*skills = append(*skills, mount.Skill{Name: name, Body: body})
		return
	}
	index, ok := byName[name]
	if !ok {
		return
	}
	(*skills)[index].Body = strings.TrimRight((*skills)[index].Body, "\n") + "\n\n## Repo overrides\n\n" + body + "\n"
}

// agentFile renders one role as this host's agent file; a light tier renders as standard when
// the local machine holds the light tier, since a host spawn cannot reach Ollama.
func agentFile(role mount.Role, ollama bool) string {
	var names []string
	for _, verb := range role.Tools {
		names = append(names, tools[verb]...)
	}
	tier := role.Tier
	if ollama && tier == "light" {
		tier = "standard"
	}
	model := models[tier]
	head := []string{
		"---",
		"name: " + role.Name,
		"description: " + role.Description,
	}
	if len(names) > 0 {
		head = append(head, "tools: "+strings.Join(names, ", "))
	}
	if model != "" {
		head = append(head, "model: "+model)
	}
	head = append(head, "---", "")
	return strings.Join(head, "\n") + "\n" + role.Instructions() + "\n"
}

// settingsFile renders the hook registration and the permissions convenience layer.
func settingsFile(root, binary string) ([]byte, error) {
	policy, err := readPolicy(filepath.Join(root, "komodo", "policy.json"))
	if err != nil {
		return nil, err
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": "Bash|Edit|Write|MultiEdit|NotebookEdit",
				"hooks":   []any{map[string]any{"type": "command", "command": binary + " guard"}},
			}},
		},
		"permissions": map[string]any{"deny": denyList(policy)},
	}
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// denyList turns the policy's critical refs and config paths into this host's permission entries;
// one edit entry covers every file-editing tool this host has.
func denyList(policy policyFile) []string {
	var out []string
	for _, ref := range policy.CriticalRefs {
		out = append(out, fmt.Sprintf("Bash(git push origin %s:*)", ref))
		out = append(out, fmt.Sprintf("Bash(git push %s:*)", ref))
	}
	for _, path := range policy.ConfigPaths {
		out = append(out, fmt.Sprintf("Edit(%s)", path))
	}
	return out
}

// policyFile is the part of the policy this host's permission layer mirrors.
type policyFile struct {
	CriticalRefs []string `json:"critical_refs"`
	ConfigPaths  []string `json:"config_paths"`
}

// readPolicy reads the shipped policy, tolerating its absence.
func readPolicy(path string) (policyFile, error) {
	var policy policyFile
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return policy, nil
	}
	if err != nil {
		return policy, err
	}
	return policy, json.Unmarshal(data, &policy)
}

// namesBridge reports whether a file still points at the retired local bridge.
func namesBridge(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "127.0.0.1:8000")
}

// init registers this mount so the binary never names the host itself.
func init() {
	mount.Register(mount.Host{
		Name:        "claude",
		ConfigPaths: []string{"~/.claude/**", "~/.claude.json"},
		Vendors:     []string{"claude", "anthropic", "sonnet", "opus", "haiku"},
		HybridName:  "hybrid",
		Render:      Render,
		Installed:   Installed,
		Tiers:       Tiers,
		Probe:       Probe,
		Usage:       Usage,
		Headless:    Headless,
	})
}

// Headless returns this host's non-interactive command for one skill and one target.
func Headless(skill, target string) (string, []string) {
	prompt := "/" + skill
	if target != "" {
		prompt += " " + target
	}
	return "claude", []string{"-p", prompt, "--permission-mode", "acceptEdits"}
}
