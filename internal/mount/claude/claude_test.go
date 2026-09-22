package claude

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/profile"
)

// toolkitRepo builds a root with the rules, two roles, a skill, and the policy, with the local machine down.
func toolkitRepo(t *testing.T) string {
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
	write("komodo/roles/summarizer.md", "---\nname: summarizer\ndescription: Compresses text.\ntier: light\n"+
		"tools: []\nsession: false\nreturns: summarizer.schema.json\n---\n\nYou compress text.\n")
	write("komodo/skills/run/SKILL.md", "---\nname: run\n---\n\nCall komodo step.\n")
	write("komodo/policy.json", `{"critical_refs":["main"],"config_paths":["bin/**"],"trailer_patterns":[]}`)
	return root
}

// body returns one planned file's contents.
func body(t *testing.T, root, rel string) string {
	t.Helper()
	plan, err := Render(root, "bin/komodo-darwin-arm64")
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

func TestRenderWritesTheIncludeAndTheRules(t *testing.T) {
	root := toolkitRepo(t)
	if got := body(t, root, "CLAUDE.md"); got != "@AGENTS.md\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	rules := body(t, root, filepath.Join(Dir, "komodo", "AGENTS.md"))
	if strings.Contains(rules, "{{accessibility}}") {
		t.Fatal("the accessibility slot was not filled")
	}
}

func TestOnlySessionRolesBecomeAgents(t *testing.T) {
	root := toolkitRepo(t)
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, change := range plan.Changes {
		if strings.Contains(change.Path, filepath.Join(Dir, "agents")) {
			names = append(names, filepath.Base(change.Path))
		}
	}
	if len(names) != 1 || names[0] != "builder.md" {
		t.Fatalf("agents = %v", names)
	}
}

func TestAnAgentCarriesHostToolNamesAndAModel(t *testing.T) {
	root := toolkitRepo(t)
	agent := body(t, root, filepath.Join(Dir, "agents", "builder.md"))
	for _, want := range []string{"name: builder", "tools: Read, Edit, Write, Bash, Grep, Glob", "model: sonnet"} {
		if !strings.Contains(agent, want) {
			t.Errorf("the agent is missing %q:\n%s", want, agent)
		}
	}
	if strings.Contains(agent, "read, edit") {
		t.Fatal("a Komodo verb reached the host's agent file")
	}
}

// scoutRoot builds a root whose only session role is on the light tier.
func scoutRoot(t *testing.T) string {
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
	write("komodo/roles/scout.md", "---\nname: scout\ndescription: Scouts a file.\ntier: light\n"+
		"tools: [read, search]\nsession: true\nreturns: scout.schema.json\n---\n\nYou scout.\n")
	write("komodo/skills/run/SKILL.md", "---\nname: run\n---\n\nCall komodo step.\n")
	write("komodo/policy.json", `{"critical_refs":[],"config_paths":[],"trailer_patterns":[]}`)
	return root
}

func TestALightTierSessionRoleKeepsTheLightModelWithNoLocalMachine(t *testing.T) {
	root := scoutRoot(t)
	agent := body(t, root, filepath.Join(Dir, "agents", "scout.md"))
	if !strings.Contains(agent, "model: "+models["light"]) {
		t.Fatalf("scout did not keep the light model: %s", agent)
	}
}

func TestALightTierSessionRoleRendersAsStandardWithTheLocalMachineUp(t *testing.T) {
	root := scoutRoot(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv(profile.OllamaEnv, "http://"+listener.Addr().String())
	agent := body(t, root, filepath.Join(Dir, "agents", "scout.md"))
	if !strings.Contains(agent, "model: "+models["standard"]) {
		t.Fatalf("scout did not fall back to the standard model: %s", agent)
	}
	if strings.Contains(agent, "model: "+models["light"]) {
		t.Fatal("scout still names the light model, which the local machine now holds")
	}
}

func TestSettingsRegisterTheGuardOnceAndNoMCP(t *testing.T) {
	root := toolkitRepo(t)
	raw := body(t, root, filepath.Join(Dir, "settings.json"))
	var settings struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("settings are not JSON: %v", err)
	}
	if len(settings.Hooks.PreToolUse) != 1 || len(settings.Hooks.PreToolUse[0].Hooks) != 1 {
		t.Fatalf("the guard is not registered once: %+v", settings.Hooks.PreToolUse)
	}
	if !strings.HasSuffix(settings.Hooks.PreToolUse[0].Hooks[0].Command, " guard") {
		t.Fatalf("command = %q", settings.Hooks.PreToolUse[0].Hooks[0].Command)
	}
	if len(settings.Permissions.Deny) == 0 {
		t.Fatal("the permissions layer is empty")
	}
	for _, entry := range settings.Permissions.Deny {
		if strings.HasPrefix(entry, "Write(") {
			t.Fatalf("deny entry %q is inert; this host matches file rules on Edit only", entry)
		}
	}
	if strings.Contains(raw, "mcpServers") {
		t.Fatal("the render registered an MCP server")
	}
}

func TestThePersonalOverlayIsSeededNotOverwritten(t *testing.T) {
	root := toolkitRepo(t)
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	seeded := 0
	for _, change := range plan.Changes {
		if change.Seed {
			seeded++
		}
	}
	if seeded != 2 {
		t.Fatalf("seeded = %d, want the two personal files", seeded)
	}
}

func TestTheOldLocalServerEntryIsRemovedOnlyWhenItNamesTheServer(t *testing.T) {
	root := toolkitRepo(t)
	plan, _ := Render(root, "komodo")
	for _, change := range plan.Changes {
		if change.Remove && filepath.Base(change.Path) == ".mcp.json" {
			t.Fatal("an absent local server file was scheduled for removal")
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"url":"http://127.0.0.1:8000"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, _ = Render(root, "komodo")
	found := false
	for _, change := range plan.Changes {
		if change.Remove && filepath.Base(change.Path) == ".mcp.json" {
			found = true
		}
	}
	if !found {
		t.Fatal("the old local server entry was not removed")
	}
}

func TestEverySkillIsCopied(t *testing.T) {
	root := toolkitRepo(t)
	if got := body(t, root, filepath.Join(Dir, "skills", "run", "SKILL.md")); !strings.Contains(got, "komodo step") {
		t.Fatalf("skill = %q", got)
	}
}

func TestRepoStandardAppendsToAShippedOne(t *testing.T) {
	root := toolkitRepo(t)
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("komodo/skills/standards-go/SKILL.md", "---\nname: standards-go\nglobs: [\"**/*.go\"]\n---\n\n# Go\n\nWrap with %w.\n")
	write(".komodo/standards/go.md", "This repo also bans naked returns.\n")
	got := body(t, root, filepath.Join(Dir, "skills", "standards-go", "SKILL.md"))
	if !strings.Contains(got, "Wrap with %w.") || !strings.Contains(got, "## Repo overrides") ||
		!strings.Contains(got, "naked returns") {
		t.Fatalf("standard = %q", got)
	}
}

func TestARepoStandardWithFrontmatterStillMergesIntoAShippedOne(t *testing.T) {
	root := toolkitRepo(t)
	write := func(rel, contents string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("komodo/skills/standards-go/SKILL.md", "---\nname: standards-go\n---\n\n# Go\n")
	write(".komodo/standards/go.md", "---\nglobs: [\"**/*.go\"]\n---\n\nThis repo also bans naked returns.\n")
	plan, err := Render(root, "bin/komodo-darwin-arm64")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, Dir, "skills", "standards-go", "SKILL.md")
	var matches int
	var got string
	for _, change := range plan.Changes {
		if change.Path == target {
			matches++
			got = string(change.Body)
		}
	}
	if matches != 1 {
		t.Fatalf("standards-go was scheduled %d times, want 1", matches)
	}
	if !strings.Contains(got, "# Go") || !strings.Contains(got, "## Repo overrides") ||
		!strings.Contains(got, "naked returns") {
		t.Fatalf("standard = %q", got)
	}
	if strings.Contains(got, "globs:") {
		t.Fatalf("standard = %q, the override's own frontmatter must not appear in the body", got)
	}
}

func TestRepoSkillWithFrontmatterIsNew(t *testing.T) {
	root := toolkitRepo(t)
	path := filepath.Join(root, ".komodo", "skills", "deploy", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: deploy\ndescription: this repo's own deploy steps\n---\n\nRun the deploy script.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := body(t, root, filepath.Join(Dir, "skills", "deploy", "SKILL.md"))
	if !strings.Contains(got, "Run the deploy script.") {
		t.Fatalf("skill = %q", got)
	}
}

func TestARepoSkillCannotAppendAProtectedOne(t *testing.T) {
	root := toolkitRepo(t)
	path := filepath.Join(root, ".komodo", "skills", "run", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("Never happens.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := body(t, root, filepath.Join(Dir, "skills", "run", "SKILL.md"))
	if strings.Contains(got, "Never happens.") {
		t.Fatal("a protected skill was appended to")
	}
}

func TestRenderSkipsAFacetNameThatTriesToEscapeTheFacetsRoot(t *testing.T) {
	root := toolkitRepo(t)
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
	plan, err := Render(root, "bin/komodo-darwin-arm64")
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
	root := toolkitRepo(t)
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	narrowed := plan.Project()
	var names []string
	for _, change := range narrowed.Changes {
		names = append(names, change.Path)
	}
	if !contains(names, filepath.Join(root, Dir, "komodo", "AGENTS.md")) {
		t.Fatal("the rules file is not a project change")
	}
	if !contains(names, filepath.Join(root, Dir, "skills", "run", "SKILL.md")) {
		t.Fatal("a skill is not a project change")
	}
	if contains(names, filepath.Join(root, Dir, "settings.json")) {
		t.Fatal("settings is not a project change")
	}
}

// contains reports whether the slice holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func TestPlanNameMapsTheRateLimitTiers(t *testing.T) {
	cases := map[string]string{
		"default_claude_max_5x": "max_5x",
		"default_claude_pro":    "pro",
		"default_claude_max":    "max_20x",
		"something_else":        "",
	}
	for raw, want := range cases {
		var parsed account
		parsed.OAuth.OrganizationRateLimitTier = raw
		if got := planName(parsed); got != want {
			t.Errorf("planName(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestPlanNameFallsDownTheTiers(t *testing.T) {
	var parsed account
	parsed.OAuth.UserRateLimitTier = "max_20x"
	if got := planName(parsed); got != "max_20x" {
		t.Fatalf("planName = %q", got)
	}
}

func TestTheProbeStructReadsNoIdentity(t *testing.T) {
	body := `{"oauthAccount":{"emailAddress":"a@b.c","accountUuid":"u","organizationRateLimitTier":"default_claude_max_5x"},
	          "cachedUsageUtilization":{"utilization":{"five_hour":{"utilization":42,"resets_at":"2026-09-22T08:00:00Z"}}}}`
	var parsed account
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatal(err)
	}
	if planName(parsed) != "max_5x" {
		t.Fatalf("plan = %q", planName(parsed))
	}
	if parsed.Cached.Utilization.FiveHour.Utilization != 42 {
		t.Fatalf("utilization = %v", parsed.Cached.Utilization.FiveHour.Utilization)
	}
	rendered, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"a@b.c", "accountUuid", "emailAddress"} {
		if strings.Contains(string(rendered), forbidden) {
			t.Fatalf("the probe struct carries %q", forbidden)
		}
	}
}

func TestTheProTierDropsTheHeavyCeiling(t *testing.T) {
	if Tiers("pro", false).Heavy.Model != models["standard"] {
		t.Fatal("pro did not lower the heavy tier")
	}
	if Tiers("max_5x", false).Heavy.Model != models["heavy"] {
		t.Fatal("max did not keep the heavy tier")
	}
}

func TestOllamaTakesTheLightTierAndTheReviewer(t *testing.T) {
	tiers := Tiers("max_5x", true)
	if tiers.Light.Provider != "ollama" || tiers.Reviewer.Provider != "ollama" {
		t.Fatalf("tiers = %+v", tiers)
	}
	if tiers.Standard.Provider == "ollama" {
		t.Fatal("the builder was moved to the local machine")
	}
	if tiers.Light.Model != profile.OllamaModel || tiers.Reviewer.Model != profile.OllamaModel {
		t.Fatalf("tiers did not carry the profile's model: %+v", tiers)
	}
}
