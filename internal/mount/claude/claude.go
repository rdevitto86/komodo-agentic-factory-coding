// Package claude is the Claude Code mount: the only place this host's names appear.
package claude

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/facet"
	"komodo/internal/install"
	"komodo/internal/mount"
	"komodo/internal/mount/ollama"
	repopkg "komodo/internal/repo"
	"komodo/internal/toolkit"
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

// retired are the exact files an old render wrote; a directory a user keeps files in is never wiped.
var retired = []string{
	filepath.Join(Dir, "hooks", "guard.py"),
	filepath.Join(Dir, "hooks", "context_injector.py"),
	filepath.Join(Dir, "hooks", "komodo-hooks"),
	filepath.Join(Dir, "mcp.json"),
	".mcp.json",
}

// claudeMDImport puts the rendered rules on the file this host always loads into a session.
const claudeMDImport = "@" + Dir + "/komodo/AGENTS.md\n"

// Render builds the plan that mounts this repo on Claude Code.
func Render(root string, binary string) (install.Plan, error) {
	plan := install.Plan{Host: "claude", Root: root}
	rules, err := mount.Rules(root)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, "CLAUDE.md"), claudeMD(root), "the host reads the repo's own rules, plus the rendered ones")
	plan.AddProject(filepath.Join(root, Dir, "komodo", "AGENTS.md"), []byte(rules), "the universal rules, rendered")

	roles, err := mount.LoadRoles(root)
	if err != nil {
		return plan, err
	}
	localUp := ollama.Up()
	for _, role := range roles {
		if !role.Session {
			continue
		}
		plan.Add(filepath.Join(root, Dir, "agents", role.Name+".md"), []byte(agentFile(role, localUp)), "the "+role.Name+" role as an agent")
	}

	detected := detect.Load(root)
	skills, err := mount.LoadSkills(root)
	if err != nil {
		return plan, err
	}
	skills = mount.SelectStandards(root, skills)
	skills = repoSkills(root, skills)
	for _, skill := range skills {
		plan.AddProject(filepath.Join(root, Dir, "skills", skill.Name, "SKILL.md"), []byte(skill.Body), "the "+skill.Name+" skill")
	}

	for _, name := range facetSkills(root, detected) {
		if !facet.ValidName(name) {
			continue
		}
		loaded, err := facet.Load(root, name)
		if err != nil {
			continue
		}
		plan.AddProject(filepath.Join(root, Dir, "skills", "facet-"+loaded.Name, "SKILL.md"),
			[]byte(loaded.Skill), "the "+loaded.Name+" facet's setup skill")
	}

	mount.PruneSkills(&plan, root, filepath.Join(root, Dir, "skills"))

	settings, err := settingsFile(root, binary)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, Dir, "settings.json"), settings, "the guard on PreToolUse and the permissions layer")
	plan.AddSeed(filepath.Join(root, Dir, "settings.local.json"), []byte("{\n  \"permissions\": {\n    \"allow\": []\n  }\n}\n"), "the personal overlay")
	plan.AddSeed(filepath.Join(root, "CLAUDE.local.md"), []byte("# Personal overlay\n\nYours. The install never overwrites this file.\n"), "the personal overlay")

	for _, path := range retired {
		full := filepath.Join(root, path)
		if path == ".mcp.json" && !namesLocalServer(full) {
			continue
		}
		plan.AddRemoval(full, "the prototype's render and the old local server entry")
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
		mount.MergeOverride(&skills, byName, "standards-"+override.Name, override.New, override.Body)
	}
	overrides, _ := repopkg.LoadSkills(root)
	for _, override := range overrides {
		mount.MergeOverride(&skills, byName, override.Name, override.New, override.Body)
	}
	return skills
}

// facetSkills names the facets the detected profile and the repo's own additions select.
func facetSkills(root string, detected detect.Profile) []string {
	names, err := facet.Select(root, detected, nil)
	if err != nil {
		return nil
	}
	return names
}

// claudeMD adds the rules import to an existing CLAUDE.md, or seeds a fresh one that carries it.
func claudeMD(root string) []byte {
	existing, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		return []byte("@AGENTS.md\n" + claudeMDImport)
	}
	if strings.Contains(string(existing), claudeMDImport) {
		return existing
	}
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		existing = append(existing, '\n')
	}
	return append(existing, []byte(claudeMDImport)...)
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
	model := modelFor(tier)
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

// settingsFile renders the hook registration, the permissions convenience layer, and attribution off.
func settingsFile(root, binary string) ([]byte, error) {
	policy, err := readPolicy(toolkit.FS(root))
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(mount.MainCheckout(root), binary)
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": hookMatcher(),
				"hooks":   []any{map[string]any{"type": "command", "command": binary + " guard"}},
			}},
		},
		"permissions": map[string]any{"deny": denyList(policy)},
		// No co-author trailer, no pull request footer, no session link, in every session and subagent.
		"attribution": map[string]any{"commit": "", "pr": "", "sessionUrl": false},
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
	// The hosts' own config, the hook's settings file included, is denied beside the policy's paths.
	seen := map[string]bool{}
	for _, path := range append(append(append([]string{}, policy.ConfigPaths...), mount.ConfigPaths()...), mount.GuardConfigPaths()...) {
		if !seen[path] {
			seen[path] = true
			out = append(out, fmt.Sprintf("Edit(%s)", path))
		}
	}
	return out
}

// policyFile is the part of the policy this host's permission layer mirrors.
type policyFile struct {
	CriticalRefs []string `json:"critical_refs"`
	ConfigPaths  []string `json:"config_paths"`
}

// readPolicy reads the shipped policy, tolerating its absence.
func readPolicy(tree fs.FS) (policyFile, error) {
	var policy policyFile
	data, err := fs.ReadFile(tree, "policy.json")
	if os.IsNotExist(err) {
		return policy, nil
	}
	if err != nil {
		return policy, err
	}
	return policy, json.Unmarshal(data, &policy)
}

// namesLocalServer reports whether a file still points at the retired local server.
func namesLocalServer(path string) bool {
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
		Leftovers:   Leftovers,
		ReviewerWhy: reviewerWhy,
	})
}

// Headless returns this host's non-interactive command for one skill and one target, with prompts
// bypassed since none can be answered, the guard hook as the wall, and the standard tier driving.
func Headless(skill, target string) (string, []string) {
	prompt := "/" + skill
	if target != "" {
		prompt += " " + target
	}
	args := []string{"-p", prompt, "--permission-mode", "bypassPermissions", "--model", modelFor("standard")}
	if settings := sandboxSettings(mount.LoadOverlay()); settings != "" {
		args = append(args, "--settings", settings)
	}
	return "claude", args
}

// sandboxSettings is the inline settings that sandbox every headless shell command when the overlay
// opts in: no unsandboxed retry, and a refusal to start when the sandbox cannot.
func sandboxSettings(overlay mount.Overlay) string {
	if !overlay.Sandbox {
		return ""
	}
	sandbox := map[string]any{"enabled": true, "failIfUnavailable": true, "allowUnsandboxedCommands": false}
	if len(overlay.SandboxWrite) > 0 {
		sandbox["filesystem"] = map[string]any{"allowWrite": overlay.SandboxWrite}
	}
	if len(overlay.SandboxDomains) > 0 {
		sandbox["network"] = map[string]any{"allowedDomains": overlay.SandboxDomains}
	}
	data, _ := json.Marshal(map[string]any{"sandbox": sandbox})
	return string(data)
}

// retiredCommands are the commands no hook or allow rule in this host's user settings may run.
var retiredCommands = []string{"komodo-hooks", "python3 -m komodo"}

// Leftovers names each hook and allow rule in this host's user settings that runs a retired command.
func Leftovers() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return leftoversIn(filepath.Join(home, Dir, "settings.json"))
}

// leftoversIn reads one settings file and names each hook command and allow rule that runs a retired command.
func leftoversIn(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if json.Unmarshal(data, &settings) != nil {
		return nil
	}
	events := make([]string, 0, len(settings.Hooks))
	for event := range settings.Hooks {
		events = append(events, event)
	}
	sort.Strings(events)
	var found []string
	for _, event := range events {
		for _, group := range settings.Hooks[event] {
			for _, hook := range group.Hooks {
				if isRetired(hook.Command) {
					found = append(found, fmt.Sprintf("%s: the %s hook runs the retired %q", path, event, hook.Command))
				}
			}
		}
	}
	for _, rule := range settings.Permissions.Allow {
		if isRetired(rule) {
			found = append(found, fmt.Sprintf("%s: the allow rule %q names a retired command", path, rule))
		}
	}
	return found
}

// isRetired reports whether a command or rule names one of the retired prototype commands.
func isRetired(text string) bool {
	for _, name := range retiredCommands {
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}
