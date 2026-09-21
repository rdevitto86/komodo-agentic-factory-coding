// Package codex is the Codex mount: the only place this host's names appear.
package codex

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"komodo/internal/install"
	"komodo/internal/mount"
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
	plan.Add(filepath.Join(root, Dir, "komodo", "AGENTS.md"), []byte(rules), "the universal rules, rendered")

	roles, err := mount.LoadRoles(root)
	if err != nil {
		return plan, err
	}
	for _, role := range roles {
		if !role.Session {
			continue
		}
		plan.Add(filepath.Join(root, Dir, "agents", role.Name+".toml"), []byte(agentFile(role)), "the "+role.Name+" role as an agent")
	}

	skills, err := mount.LoadSkills(root)
	if err != nil {
		return plan, err
	}
	for _, skill := range skills {
		plan.Add(filepath.Join(root, SkillsDir, skill.Name, "SKILL.md"), []byte(skill.Body), "the "+skill.Name+" skill")
	}

	hooks, err := hooksFile(binary)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, Dir, "hooks.json"), hooks, "the guard before every tool call")
	plan.AddSeed(filepath.Join(root, "AGENTS.md"), []byte("# Agent Rules\n\nSee `.codex/komodo/AGENTS.md`.\n"), "the repo's own rules")
	return plan, nil
}

// agentFile renders one role as this host's TOML agent.
func agentFile(role mount.Role) string {
	sandbox := "read-only"
	if role.Writes() {
		sandbox = "workspace-write"
	}
	lines := []string{
		fmt.Sprintf("name = %q", role.Name),
		fmt.Sprintf("description = %q", role.Description),
		fmt.Sprintf("model = %q", models[role.Tier]),
		fmt.Sprintf("model_reasoning_effort = %q", efforts[role.Tier]),
		fmt.Sprintf("sandbox_mode = %q", sandbox),
		"developer_instructions = \"\"\"",
		role.Instructions(),
		"\"\"\"",
	}
	return strings.Join(lines, "\n") + "\n"
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
	})
}
