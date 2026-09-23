package codex

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/profile"
)

// toolkit builds a root with the rules, two roles, and a skill, with the local machine down.
func toolkit(t *testing.T) string {
	t.Helper()
	t.Setenv(profile.OllamaEnv, "http://127.0.0.1:1")
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
		`name = "builder"`, `description = "Writes code."`, `model = "standard"`,
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
	if !strings.Contains(agent, `model = "large"`) || !strings.Contains(agent, `model_reasoning_effort = "high"`) {
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
		Hooks []struct {
			Event   string `json:"event"`
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(raw), &hooks); err != nil {
		t.Fatalf("hooks are not JSON: %v", err)
	}
	if len(hooks.Hooks) != 1 || hooks.Hooks[0].Event != "PreToolUse" {
		t.Fatalf("hooks = %+v", hooks.Hooks)
	}
	if !strings.HasSuffix(hooks.Hooks[0].Command, " guard") {
		t.Fatalf("command = %q", hooks.Hooks[0].Command)
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
	t.Setenv(profile.OllamaEnv, "http://"+listener.Addr().String())
}

func TestTheLocalProfilePointsEveryTierAtTheLocalMachine(t *testing.T) {
	root := toolkit(t)
	withLocalMachine(t)
	agent := body(t, root, filepath.Join(Dir, "agents", "builder.toml"))
	if !strings.Contains(agent, `model = "`+profile.OllamaModel+`"`) {
		t.Fatalf("the builder did not take the local model: %s", agent)
	}
	agent = body(t, root, filepath.Join(Dir, "agents", "reviewer.toml"))
	if !strings.Contains(agent, `model = "`+profile.OllamaModel+`"`) {
		t.Fatalf("the reviewer did not take the local model: %s", agent)
	}
}

func TestTheLocalProfileSetsOssProviderAndTheBaseURL(t *testing.T) {
	root := toolkit(t)
	withLocalMachine(t)
	config := body(t, root, filepath.Join(Dir, "config.toml"))
	if !strings.Contains(config, `oss_provider = "ollama"`) {
		t.Fatalf("config.toml did not set oss_provider: %s", config)
	}
	if !strings.Contains(config, "[model_providers.ollama]") || !strings.Contains(config, "base_url =") {
		t.Fatalf("config.toml did not set the ollama base_url: %s", config)
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
	if tiers.Light.Model != profile.OllamaModel {
		t.Fatalf("model = %q", tiers.Light.Model)
	}
}

func TestConfigTomlIsAbsentWithoutTheLocalMachine(t *testing.T) {
	root := toolkit(t)
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if change.Path == filepath.Join(root, Dir, "config.toml") {
			t.Fatal("config.toml was rendered with the local machine down")
		}
	}
}
