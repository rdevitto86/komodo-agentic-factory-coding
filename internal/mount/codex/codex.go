// Package codex is the Codex mount: the only place this host's names appear.
package codex

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"komodo/internal/install"
	"komodo/internal/mount"
	"komodo/internal/mount/ollama"
	"komodo/internal/profile"
	repopkg "komodo/internal/repo"
)

// Dir is the host's project directory, relative to the repo root.
const Dir = ".codex"

// SkillsDir is where this host reads skills from.
const SkillsDir = ".agents/skills"

// models maps a role's tier to this host's model, which is what a profile row sets.
var models = map[string]string{"light": "small", "standard": "standard", "heavy": "large"}

// efforts maps a role's tier to this host's reasoning effort.
var efforts = map[string]string{"light": "low", "standard": "medium", "heavy": "high"}

// Render builds the plan that mounts this repo on Codex.
func Render(root string, binary string) (install.Plan, error) {
	plan := install.Plan{Host: "codex", Root: root}
	rules, err := mount.Rules(root)
	if err != nil {
		return plan, err
	}
	plan.AddProject(filepath.Join(root, Dir, "komodo", "AGENTS.md"), []byte(rules), "the universal rules, rendered")

	roles, err := mount.LoadRoles(root)
	if err != nil {
		return plan, err
	}
	local := profile.OllamaUp()
	for _, role := range roles {
		if !role.Session {
			continue
		}
		plan.Add(filepath.Join(root, Dir, "agents", role.Name+".toml"), []byte(agentFile(role, local)), "the "+role.Name+" role as an agent")
	}

	skills, err := mount.LoadSkills(root)
	if err != nil {
		return plan, err
	}
	skills = repoSkills(root, skills)
	for _, skill := range skills {
		plan.AddProject(filepath.Join(root, SkillsDir, skill.Name, "SKILL.md"), []byte(skill.Body), "the "+skill.Name+" skill")
	}

	hooks, err := hooksFile(binary)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, Dir, "hooks.json"), hooks, "the guard before every tool call")
	plan.AddSeed(filepath.Join(root, "AGENTS.md"), []byte("# Agent Rules\n\nSee `.codex/komodo/AGENTS.md`.\n"), "the repo's own rules")
	if local {
		plan.Add(filepath.Join(root, Dir, "config.toml"), configFile(), "the local machine as this host's own provider")
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

// agentFile renders one role as this host's TOML agent; every tier's model comes from the
// local machine once it is up, since the local profile runs the builder locally too.
func agentFile(role mount.Role, local bool) string {
	sandbox := "read-only"
	if role.Writes() {
		sandbox = "workspace-write"
	}
	model := models[role.Tier]
	if local {
		model = profile.OllamaModel
	}
	lines := []string{
		fmt.Sprintf("name = %q", role.Name),
		fmt.Sprintf("description = %q", role.Description),
		fmt.Sprintf("model = %q", model),
		fmt.Sprintf("model_reasoning_effort = %q", efforts[role.Tier]),
		fmt.Sprintf("sandbox_mode = %q", sandbox),
		"developer_instructions = \"\"\"",
		role.Instructions(),
		"\"\"\"",
	}
	return strings.Join(lines, "\n") + "\n"
}

// configFile points this host's own provider setting at the local machine.
func configFile() []byte {
	lines := []string{
		`oss_provider = "ollama"`,
		"",
		"[model_providers.ollama]",
		fmt.Sprintf("base_url = %q", ollama.BaseURL()+"/v1"),
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// hooksFile registers the guard before every tool call.
func hooksFile(binary string) ([]byte, error) {
	hooks := map[string]any{
		"hooks": []any{map[string]any{
			"event":   "PreToolUse",
			"command": binary + " guard",
		}},
	}
	body, err := json.MarshalIndent(hooks, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// init registers this mount so the binary never names the host itself.
func init() {
	mount.Register(mount.Host{
		Name:        "codex",
		ConfigPaths: []string{"~/.codex/**"},
		Vendors:     []string{"codex", "openai"},
		HybridName:  "local",
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
	return "codex", []string{"exec", "--json", prompt}
}
