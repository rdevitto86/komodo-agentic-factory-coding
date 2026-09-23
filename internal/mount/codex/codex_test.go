package codex

import (
	"encoding/json"
	"komodo/internal/mount/ollama"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// toolkit builds a root with the rules, two roles, and a skill, with the local machine down.
func toolkit(t *testing.T) string {
	t.Helper()
	t.Setenv(ollama.Env, "http://127.0.0.1:1")
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("komodo/AGENTS.md", "# Agent Rules\n\n{{accessibility}}\n")
	write("komodo/rules/accessibility.md", "## Writing for a human\n- **Answer first.**\n")
	write("komodo/roles/builder.md", "---\nname: builder\ndescription: Writes code.\ntier: standard\n"+
		"tools: [read, edit, write, shell, search]\nsession: true\nreturns: builder.schema.json\n---\n\nYou are a builder.\n")
	write("komodo/roles/reviewer.md", "---\nname: reviewer\ndescription: Reviews a diff.\ntier: heavy\n"+
		"tools: [read, search]\nsession: true\nreturns: reviewer.schema.json\n---\n\nYou review one diff.\n")
	write("komodo/skills/run/SKILL.md", "---\nname: run\n---\n\nCall komodo step.\n")
	return root
}

// body returns one planned file's contents.
func body(t *testing.T, root, rel string) string {
	t.Helper()
	plan, err := Render(root, "bin/komodo-linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if change.Path == filepath.Join(root, rel) {
			return string(change.Body)
		}
	}
	t.Fatalf("%s is not in the plan", rel)
	return ""
}

func TestAnAgentCarriesEveryTomlKey(t *testing.T) {
	root := toolkit(t)
	agent := body(t, root, filepath.Join(Dir, "agents", "builder.toml"))
	for _, want := range []string{
		`name = "builder"`, `description = "Writes code."`, `model = "gpt-6-sol"`,
		`model_reasoning_effort = "medium"`, `sandbox_mode = "workspace-write"`, "developer_instructions =",
	} {
		if !strings.Contains(agent, want) {
			t.Errorf("the agent is missing %q:\n%s", want, agent)
		}
	}
}

func TestAReadOnlyRoleGetsAReadOnlySandbox(t *testing.T) {
	root := toolkit(t)
	agent := body(t, root, filepath.Join(Dir, "agents", "reviewer.toml"))
	if !strings.Contains(agent, `sandbox_mode = "read-only"`) {
		t.Fatalf("agent = %s", agent)
	}
	if !strings.Contains(agent, `model = "gpt-6-astra"`) || !strings.Contains(agent, `model_reasoning_effort = "high"`) {
		t.Fatalf("the heavy tier did not map: %s", agent)
	}
}

func TestSkillsLandUnderTheHostsSkillDirectory(t *testing.T) {
	root := toolkit(t)
	if got := body(t, root, filepath.Join(SkillsDir, "run", "SKILL.md")); !strings.Contains(got, "komodo step") {
		t.Fatalf("skill = %q", got)
	}
}

func TestTheGuardIsRegisteredAndNoMCP(t *testing.T) {
	root := toolkit(t)
	raw := body(t, root, filepath.Join(Dir, "hooks.json"))
	var hooks struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(raw), &hooks); err != nil {
		t.Fatalf("hooks are not in the host's nested hooks.PreToolUse shape: %v", err)
	}
	entries := hooks.Hooks.PreToolUse
	if len(entries) != 1 || entries[0].Matcher != "Bash" || len(entries[0].Hooks) != 1 || entries[0].Hooks[0].Type != "command" {
		t.Fatalf("PreToolUse = %+v", entries)
	}
	if !strings.HasSuffix(entries[0].Hooks[0].Command, " guard") {
		t.Fatalf("command = %q", entries[0].Hooks[0].Command)
	}
	plan, _ := Render(root, "komodo")
	for _, change := range plan.Changes {
		if strings.Contains(string(change.Body), "mcp") {
			t.Fatalf("%s names MCP", change.Path)
		}
	}
}

func TestTheRulesAreRenderedWithTheContract(t *testing.T) {
	root := toolkit(t)
	rules := body(t, root, filepath.Join(Dir, "komodo", "AGENTS.md"))
	if strings.Contains(rules, "{{accessibility}}") || !strings.Contains(rules, "Answer first") {
		t.Fatalf("rules = %q", rules)
	}
}

func TestRepoSkillAppendsToAShippedOne(t *testing.T) {
	root := toolkit(t)
	overridePath := filepath.Join(root, ".komodo", "skills", "builder", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(overridePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overridePath, []byte("This repo deploys through a Makefile.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shippedPath := filepath.Join(root, "komodo", "skills", "builder", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(shippedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shippedPath, []byte("---\nname: builder\n---\n\nBuild it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := body(t, root, filepath.Join(SkillsDir, "builder", "SKILL.md"))
	if !strings.Contains(got, "Build it.") || !strings.Contains(got, "## Repo overrides") ||
		!strings.Contains(got, "through a Makefile") {
		t.Fatalf("skill = %q", got)
	}
}

func TestARepoSkillWithFrontmatterStillMergesIntoAShippedOne(t *testing.T) {
	root := toolkit(t)
	shippedPath := filepath.Join(root, "komodo", "skills", "builder", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(shippedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shippedPath, []byte("---\nname: builder\n---\n\nBuild it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	overridePath := filepath.Join(root, ".komodo", "skills", "builder", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(overridePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overridePath, []byte("---\nname: builder\n---\n\nThis repo deploys through a Makefile.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, SkillsDir, "builder", "SKILL.md")
	var matches int
	var got string
	for _, change := range plan.Changes {
		if change.Path == target {
			matches++
			got = string(change.Body)
		}
	}
	if matches != 1 {
		t.Fatalf("builder was scheduled %d times, want 1", matches)
	}
	if !strings.Contains(got, "Build it.") || !strings.Contains(got, "## Repo overrides") ||
		!strings.Contains(got, "through a Makefile") {
		t.Fatalf("skill = %q", got)
	}
	if strings.Count(got, "---\nname: builder\n---") > 1 {
		t.Fatalf("skill = %q, the override's own frontmatter must not repeat in the body", got)
	}
}

func TestRenderSkipsAFacetNameThatTriesToEscapeTheFacetsRoot(t *testing.T) {
	root := toolkit(t)
	if err := os.MkdirAll(filepath.Join(root, "komodo", "facets"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A directory one level above komodo/facets, reachable only by a traversal name.
	outside := filepath.Join(root, "komodo", "escaped", "skill")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "komodo", "escaped", "facet.md"), []byte("SECRET_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("SECRET_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
	overridePath := filepath.Join(root, ".komodo", "facets")
	if err := os.MkdirAll(filepath.Dir(overridePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overridePath, []byte("../escaped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if strings.Contains(string(change.Body), "SECRET_MARKER") {
			t.Fatalf("a facet name escaped komodo/facets: %s", change.Path)
		}
	}
}

func TestTheRulesFileAndTheSkillsAreProjectChanges(t *testing.T) {
	root := toolkit(t)
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	narrowed := plan.Project()
	found := map[string]bool{}
	for _, change := range narrowed.Changes {
		found[change.Path] = true
	}
	if !found[filepath.Join(root, Dir, "komodo", "AGENTS.md")] {
		t.Fatal("the rules file is not a project change")
	}
	if !found[filepath.Join(root, SkillsDir, "run", "SKILL.md")] {
		t.Fatal("a skill is not a project change")
	}
	if found[filepath.Join(root, Dir, "hooks.json")] {
		t.Fatal("hooks are not a project change")
	}
}

// withLocalMachine points OLLAMA_BASE_URL at a listener that answers TCP, so the mount sees it up.
func withLocalMachine(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	t.Setenv(ollama.Env, "http://"+listener.Addr().String())
}

func TestTheLocalProfilePointsEveryTierAtTheLocalMachine(t *testing.T) {
	root := toolkit(t)
	withLocalMachine(t)
	agent := body(t, root, filepath.Join(Dir, "agents", "builder.toml"))
	if !strings.Contains(agent, `model = "`+ollama.Model+`"`) {
		t.Fatalf("the builder did not take the local model: %s", agent)
	}
	agent = body(t, root, filepath.Join(Dir, "agents", "reviewer.toml"))
	if !strings.Contains(agent, `model = "`+ollama.Model+`"`) {
		t.Fatalf("the reviewer did not take the local model: %s", agent)
	}
}

func TestTheLocalProfileSetsModelProviderOnTheAgent(t *testing.T) {
	root := toolkit(t)
	withLocalMachine(t)
	agent := body(t, root, filepath.Join(Dir, "agents", "builder.toml"))
	if !strings.Contains(agent, `model_provider = "ollama"`) {
		t.Fatalf("the local agent did not name the ollama provider: %s", agent)
	}
}

func TestTiersPutsEveryTierOnTheLocalMachine(t *testing.T) {
	tiers := Tiers("max_5x", true)
	for _, machine := range []struct {
		name string
		got  string
	}{
		{"light", tiers.Light.Provider}, {"standard", tiers.Standard.Provider},
		{"heavy", tiers.Heavy.Provider}, {"reviewer", tiers.Reviewer.Provider},
	} {
		if machine.got != "ollama" {
			t.Fatalf("%s = %q, want ollama", machine.name, machine.got)
		}
	}
	if tiers.Light.Model != ollama.Model {
		t.Fatalf("model = %q", tiers.Light.Model)
	}
}

func TestConfigTomlIsNeverRendered(t *testing.T) {
	root := toolkit(t)
	withLocalMachine(t)
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if change.Path == filepath.Join(root, Dir, "config.toml") {
			t.Fatal("config.toml was rendered; a project config.toml ignores model_provider and model_providers")
		}
	}
}

func TestHeadlessPassesAWorkspaceWriteSandbox(t *testing.T) {
	_, args := Headless("run", "TSK-01.1.1")
	if !contains(args, "--sandbox") || !contains(args, "workspace-write") {
		t.Fatalf("args = %v, want a workspace-write sandbox", args)
	}
}

// contains reports whether value appears among items.
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func TestTheRulesReachRootAgentsMDDirectlyNotAPointer(t *testing.T) {
	root := toolkit(t)
	agents := body(t, root, "AGENTS.md")
	if strings.Contains(agents, ".codex/komodo/AGENTS.md") {
		t.Fatalf("AGENTS.md points at the rendered rules instead of carrying them: %q", agents)
	}
	if !strings.Contains(agents, "Answer first") {
		t.Fatalf("AGENTS.md does not carry the rendered rules: %q", agents)
	}
}

func TestTheRulesKeepARepoOwnedAgentsMDAndReplaceOnlyTheirOwnBlock(t *testing.T) {
	root := toolkit(t)
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte("# This repo\n\nOwn instructions.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agents := body(t, root, "AGENTS.md")
	if !strings.Contains(agents, "Own instructions.") || !strings.Contains(agents, "Answer first") {
		t.Fatalf("agents.md = %q", agents)
	}
	if strings.Count(agents, "Answer first") != 1 {
		t.Fatalf("a second render duplicated the rules block: %q", agents)
	}
}

func TestAWorktreesHookPointsAtTheMainCheckoutsBinary(t *testing.T) {
	root := toolkit(t)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"config", "user.email", "t@e.st"}, {"config", "user.name", "t"},
		{"add", "-A"}, {"commit", "-q", "-m", "seed"}, {"worktree", "add", "-q", "-b", "task/x", filepath.Join(root, ".komodo", "wt", "x")},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	worktree := filepath.Join(root, ".komodo", "wt", "x")
	raw, err := hooksFile(worktree, "bin/komodo")
	if err != nil {
		t.Fatal(err)
	}
	main, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), filepath.Join(main, "bin", "komodo")+" guard") {
		t.Fatalf("hooks = %s; a worktree's hook must run the main checkout's binary, which a worktree lacks", raw)
	}
}
