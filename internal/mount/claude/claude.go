// Package claude is the Claude Code mount: the only place this host's names appear.
package claude

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/facet"
	"komodo/internal/glob"
	"komodo/internal/install"
	"komodo/internal/mount"
	"komodo/internal/mount/ollama"
	repopkg "komodo/internal/repo"
	"komodo/internal/toolkit"
)

// Dir is the host's project directory, relative to the repo root.
const Dir = ".claude"

// models pins each tier to a full model ID, never an alias, so every machine runs the same model.
var models = map[string]string{"light": "claude-haiku-4-5-20251001", "standard": "claude-sonnet-5", "heavy": "claude-opus-5-5"}

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
	filepath.Join(Dir, "skills", "backlog", "SKILL.md"),
	filepath.Join(Dir, "skills", "review", "SKILL.md"),
	filepath.Join(Dir, "skills", "adhoc", "SKILL.md"),
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
	builderOwned := RenderBuilderPlugin(&plan, root, detected, skills)
	reviewerOwned := RenderReviewerPlugin(&plan, root, skills)
	orchestratorOwned := RenderOrchestratorPlugin(&plan, root, skills)
	RenderPluginHooks(&plan, root, binary)
	for _, skill := range skills {
		// Standards serve every session; any other role skill loads only in its role's plugin.
		if (builderOwned[skill.Name] && !strings.HasPrefix(skill.Name, "standards-")) || reviewerOwned[skill.Name] || orchestratorOwned[skill.Name] {
			continue
		}
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
	mount.PruneSkills(&plan, root, filepath.Join(root, Dir, "plugins", "builder", "skills"))
	mount.PruneSkills(&plan, root, filepath.Join(root, Dir, "plugins", "reviewer", "skills"))
	mount.PruneSkills(&plan, root, filepath.Join(root, Dir, "plugins", "orchestrator", "skills"))

	settings, err := settingsFile(root, binary, false)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, Dir, "settings.json"), settings, "the permissions layer; the global layer runs the guard")
	lineSettings, err := settingsFile(root, binary, true)
	if err != nil {
		return plan, err
	}
	plan.Add(filepath.Join(root, Dir, LineSettings), lineSettings, "a line session's only settings: the guard and the permissions layer")
	plan.AddSeed(filepath.Join(root, Dir, "settings.local.json"), []byte("{\n  \"permissions\": {\n    \"allow\": []\n  }\n}\n"), "the personal overlay")
	plan.AddSeed(filepath.Join(root, "CLAUDE.local.md"), []byte("# Personal overlay\n\nYours. The install never overwrites this file.\n"), "the personal overlay")

	for _, path := range retired {
		full := filepath.Join(root, path)
		if path == ".mcp.json" && !namesLocalServer(full) {
			continue
		}
		plan.AddRemoval(full, "an old render's file, a skill komodo stopped shipping included")
	}
	return plan, nil
}

// orchestratorSkills are the only skills the user-level config carries, so no other role's skill loads there.
var orchestratorSkills = []string{"komodo", "plan", "respond", "run"}

// statusHook is the hook that adds the run's status and any blocked groups to a primary session as it starts.
const statusHook = "status"

// pruneHook is the hook that launches the background worktree sweep as a primary session starts.
const pruneHook = "prune"

// globalSkillsMarker names the file recording the orchestrator skills the last global render wrote.
const globalSkillsMarker = ".komodo-rendered"

// RenderGlobal plans the user-level orchestrator layer under home: guard, status and prune hooks, and skills.
func RenderGlobal(root, home, binary string) (install.Plan, error) {
	plan := install.Plan{Host: "claude", Root: home, Marker: filepath.Join(home, Dir, globalSkillsMarker),
		Fix: "komodo install --global"}
	skills, err := mount.LoadSkills(root)
	if err != nil {
		return plan, err
	}
	byName := map[string]mount.Skill{}
	for _, skill := range skills {
		byName[skill.Name] = skill
	}
	dir := filepath.Join(home, Dir, "skills")
	for _, name := range orchestratorSkills {
		skill, ok := byName[name]
		if !ok {
			return plan, fmt.Errorf("the toolkit ships no %s skill, which the orchestrator layer needs", name)
		}
		plan.Add(filepath.Join(dir, name, "SKILL.md"), []byte(skill.Body), "the orchestrator's "+name+" skill")
	}
	// A marked skill absent from this render's list is removed; an unmarked one, such as the user's own, stays.
	markerPath := filepath.Join(home, Dir, globalSkillsMarker)
	for _, name := range readGlobalSkillsMarker(markerPath) {
		if !slices.Contains(orchestratorSkills, name) {
			plan.AddRemoval(filepath.Join(dir, name, "SKILL.md"), "an orchestrator skill this render drops")
		}
	}
	plan.Add(markerPath, []byte(strings.Join(orchestratorSkills, "\n")+"\n"), "the record of which skills this render wrote")

	if !filepath.IsAbs(binary) {
		binary = filepath.Join(mount.MainCheckout(root), binary)
	}
	path := filepath.Join(home, Dir, "settings.json")
	settings, err := globalSettings(path, binary)
	if err != nil {
		return plan, err
	}
	plan.Add(path, settings, "the guard on PreToolUse, and the run's status and the worktree sweep on SessionStart")
	return plan, nil
}

// readGlobalSkillsMarker returns the skill names an earlier global render wrote, or none.
func readGlobalSkillsMarker(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Fields(string(data))
}

// globalSettings swaps any komodo hook in the user's settings for the guard, status and prune hooks.
func globalSettings(path, binary string) ([]byte, error) {
	settings := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()
		if err := decoder.Decode(&settings); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	events, _ := settings["hooks"].(map[string]any)
	if events == nil {
		events = map[string]any{}
	}
	named := mount.Publish(binary)
	command := func(line string) []any { return []any{map[string]any{"type": "command", "command": line}} }
	own := map[string]map[string]any{
		"PreToolUse": {"matcher": hookMatcher(), "hooks": command(named + " guard")},
		"SessionStart": {"hooks": append(command(fmt.Sprintf("%s hook %s --host claude", named, statusHook)),
			command(fmt.Sprintf("%s hook %s --host claude", named, pruneHook))...)},
	}
	for event, entry := range own {
		groups, _ := events[event].([]any)
		kept := []any{}
		for _, group := range groups {
			if group, ok := withoutKomodoHooks(group); ok {
				kept = append(kept, group)
			}
		}
		events[event] = append(kept, entry)
	}
	settings["hooks"] = events
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// withoutKomodoHooks drops every komodo hook from one matcher group, reporting false when none of its hooks remain.
func withoutKomodoHooks(group any) (any, bool) {
	entry, ok := group.(map[string]any)
	if !ok {
		return group, true
	}
	hooks, ok := entry["hooks"].([]any)
	if !ok {
		return group, true
	}
	kept := []any{}
	for _, hook := range hooks {
		if fields, ok := hook.(map[string]any); ok {
			if line, ok := fields["command"].(string); ok && install.KomodoHook(line) {
				continue
			}
		}
		kept = append(kept, hook)
	}
	if len(kept) == 0 {
		return nil, false
	}
	entry["hooks"] = kept
	return entry, true
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

// LineSettings is the settings file a headless line session loads alone, so its guard runs once.
const LineSettings = "line-settings.json"

// settingsFile renders the permissions layer and attribution off, plus the guard when guarded: a line
// session loads no user settings, so it takes its one guard from here.
func settingsFile(root, binary string, guarded bool) ([]byte, error) {
	policy, err := readPolicy(toolkit.FS(root))
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(mount.MainCheckout(root), binary)
	}
	settings := map[string]any{
		"permissions": map[string]any{"deny": denyList(root, policy)},
		// No co-author trailer, no pull request footer, no session link, in every session and subagent.
		"attribution": map[string]any{"commit": "", "pr": "", "sessionUrl": false},
	}
	if guarded {
		settings["hooks"] = map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": hookMatcher(),
				"hooks":   []any{map[string]any{"type": "command", "command": mount.Publish(binary) + " guard"}},
			}},
		}
	}
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// homeOwnedDirs are the subdirectories this host documents under its home directory.
var homeOwnedDirs = []string{"agents", "commands", "hooks", "output-styles", "plugins", "rules", "skills"}

// denyList turns the policy's critical refs and config paths into this host's permission entries,
// narrowing one that would also cover a registered mount's own write path for root.
func denyList(root string, policy policyFile) []string {
	var out []string
	for _, ref := range policy.CriticalRefs {
		out = append(out, pushRule("git push origin "+ref), pushRule("git push "+ref))
	}
	// The hosts' own config, the hook's settings file included, is denied beside the policy's paths.
	seen := map[string]bool{}
	for _, path := range append(append(append([]string{}, policy.ConfigPaths...), mount.ConfigPaths()...), mount.GuardConfigPaths()...) {
		for _, rule := range denyRulesFor(root, path) {
			if !seen[rule] {
				seen[rule] = true
				out = append(out, rule)
			}
		}
	}
	return out
}

// denyRulesFor renders path as one Edit rule, or as the narrower homeOwnedDirs rules when path
// would also cover a registered mount's own write path for root.
func denyRulesFor(root, path string) []string {
	if !coversAWritePath(root, path) {
		return []string{fmt.Sprintf("Edit(%s)", path)}
	}
	home := strings.TrimSuffix(path, "**")
	rules := []string{fmt.Sprintf("Edit(%s*)", home)}
	for _, dir := range homeOwnedDirs {
		rules = append(rules, fmt.Sprintf("Edit(%s%s/**)", home, dir))
	}
	return rules
}

// coversAWritePath reports whether path, a config path pattern, also matches a registered mount's
// own write path for root, such as a session's project memory directory.
func coversAWritePath(root, path string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	expanded := strings.Replace(path, "~", home, 1)
	for _, writePath := range mount.WritePaths(root) {
		sample := strings.TrimSuffix(writePath, "/**") + "/x"
		if glob.Match(expanded, sample) {
			return true
		}
	}
	return false
}

// pushRule denies command as a prefix; a glob ref drops the :* suffix, which would make its * literal.
func pushRule(command string) string {
	if strings.Contains(command, "*") {
		return "Bash(" + command + ")"
	}
	return "Bash(" + command + ":*)"
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

// projectSlug turns an absolute path into this host's project-directory name under ~/.claude/projects.
func projectSlug(path string) string {
	slug := strings.ReplaceAll(path, "/", "-")
	return strings.ReplaceAll(slug, ".", "-")
}

// WritePaths returns this host's project memory directory for root, the only path outside root its
// session may still write.
func WritePaths(root string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	dir := filepath.ToSlash(filepath.Join(home, Dir, "projects", projectSlug(filepath.ToSlash(abs)), "memory"))
	return []string{dir + "/**"}
}

// namesLocalServer reports whether a file still points at the retired local server.
func namesLocalServer(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "127.0.0.1:8000")
}

// profileTurnCap bounds a session's turns; a group builder works up to 12 tasks, so the clock binds first.
const profileTurnCap = 150

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
		Concurrency: Concurrency,
		LoggedIn:    LoggedIn,
		Usage:       Usage,
		Leftovers:   Leftovers,
		WritePaths:  WritePaths,
		ReviewerWhy: reviewerWhy,
		Contract: func(root, worktree string) mount.Contract {
			return NewMount(root, worktree, profileTurnCap, 0)
		},
	})
	install.RegisterGlobal("claude", RenderGlobal)
}

// retiredCommands are the commands no hook, allow rule, or mcpServers entry may still name.
var retiredCommands = []string{"komodo-hooks", "python3 -m komodo", "/assess-", "komodo-ollama-bridge"}

// Leftovers names each hook, allow rule, and mcpServers entry in this host's user settings that
// still names a retired command or server.
func Leftovers() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	notes := leftoversIn(filepath.Join(home, Dir, "settings.json"))
	return append(notes, mcpLeftoversIn(filepath.Join(home, ".claude.json"))...)
}

// mcpLeftoversIn reads this host's own config and names each retired mcpServers entry it still
// registers, at the top level and under each project entry.
func mcpLeftoversIn(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var config struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
		Projects   map[string]struct {
			McpServers map[string]json.RawMessage `json:"mcpServers"`
		} `json:"projects"`
	}
	if json.Unmarshal(data, &config) != nil {
		return nil
	}
	var found []string
	found = append(found, retiredServerNames(path, config.McpServers)...)
	projects := make([]string, 0, len(config.Projects))
	for project := range config.Projects {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	for _, project := range projects {
		found = append(found, retiredServerNames(path, config.Projects[project].McpServers)...)
	}
	return found
}

// retiredServerNames names each retired server in one mcpServers map, in a stable order.
func retiredServerNames(path string, servers map[string]json.RawMessage) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	var found []string
	for _, name := range names {
		if isRetired(name) {
			found = append(found, fmt.Sprintf("%s: the mcpServers entry %q names a retired server", path, name))
		}
	}
	return found
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
