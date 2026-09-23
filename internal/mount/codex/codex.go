// Package codex is the Codex mount: the only place this host's names appear.
package codex

import (
	"encoding/json"
	"fmt"
	"komodo/internal/mount/ollama"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/facet"
	"komodo/internal/install"
	"komodo/internal/mount"
	repopkg "komodo/internal/repo"
)

// Dir is the host's project directory, relative to the repo root.
const Dir = ".codex"

// SkillsDir is where this host reads skills from.
const SkillsDir = ".agents/skills"

// models maps a role's tier to this host's real model id, which is what a profile row sets.
var models = map[string]string{"light": "gpt-6-luna", "standard": "gpt-6-sol", "heavy": "gpt-6-astra"}

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
	local := ollama.Up()
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

	for _, name := range facetSkills(root) {
		if !facet.ValidName(name) {
			continue
		}
		loaded, err := facet.Load(root, name)
		if err != nil {
			continue
		}
		plan.AddProject(filepath.Join(root, SkillsDir, "facet-"+loaded.Name, "SKILL.md"),
			[]byte(loaded.Skill), "the "+loaded.Name+" facet's setup skill")
	}

	hooks, err := hooksFile(root, binary)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, Dir, "hooks.json"), hooks, "the guard before every tool call")
	plan.Add(filepath.Join(root, "AGENTS.md"), rootAgentsMD(root, rules), "the universal rules, reached directly since this host reads AGENTS.md every session")
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
		mount.MergeOverride(&skills, byName, "standards-"+override.Name, override.New, override.Body)
	}
	overrides, _ := repopkg.LoadSkills(root)
	for _, override := range overrides {
		mount.MergeOverride(&skills, byName, override.Name, override.New, override.Body)
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

// agentFile renders one role as this host's TOML agent; the local branch also names the
// built-in "ollama" provider directly, since a project config.toml ignores model_provider.
func agentFile(role mount.Role, local bool) string {
	sandbox := "read-only"
	if role.Writes() {
		sandbox = "workspace-write"
	}
	model := models[role.Tier]
	if local {
		model = ollama.Model
	}
	lines := []string{
		fmt.Sprintf("name = %q", role.Name),
		fmt.Sprintf("description = %q", role.Description),
		fmt.Sprintf("model = %q", model),
		fmt.Sprintf("model_reasoning_effort = %q", efforts[role.Tier]),
	}
	if local {
		lines = append(lines, `model_provider = "ollama"`)
	}
	lines = append(lines,
		fmt.Sprintf("sandbox_mode = %q", sandbox),
		"developer_instructions = \"\"\"",
		role.Instructions(),
		"\"\"\"",
	)
	return strings.Join(lines, "\n") + "\n"
}

// rulesMarker delimits the rendered rules inside AGENTS.md, which this host reads directly.
const rulesMarker = "<!-- komodo:rules -->"

// rootAgentsMD merges the rendered rules into the repo's own AGENTS.md, replacing an earlier
// render's block in place and leaving everything else the repo wrote untouched.
func rootAgentsMD(root, rules string) []byte {
	block := rulesMarker + "\n" + rules
	existing, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return []byte(block)
	}
	text := string(existing)
	if start := strings.Index(text, rulesMarker); start >= 0 {
		return []byte(text[:start] + block)
	}
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + "\n" + block)
}

// hooksFile registers the guard before every tool call; a worktree's binary path resolves
// against the main checkout, since a worktree never holds the built binary itself.
func hooksFile(root, binary string) ([]byte, error) {
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(mount.MainCheckout(root), binary)
	}
	hooks := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": guardShellTool,
				"hooks":   []any{map[string]any{"type": "command", "command": binary + " guard"}},
			}},
		},
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

// Headless returns this host's non-interactive command for one skill and one target; exec
// defaults to a read-only sandbox, so the line and a builder need workspace-write named explicitly.
func Headless(skill, target string) (string, []string) {
	prompt := "/" + skill
	if target != "" {
		prompt += " " + target
	}
	return "codex", []string{"exec", "--json", "--sandbox", "workspace-write", prompt}
}
