package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// toolkit builds a root with the rules, two roles, a skill, and the policy.
func toolkit(t *testing.T) string {
	t.Helper()
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
	root := toolkit(t)
	if got := body(t, root, "CLAUDE.md"); got != "@AGENTS.md\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	rules := body(t, root, filepath.Join(Dir, "komodo", "AGENTS.md"))
	if strings.Contains(rules, "{{accessibility}}") {
		t.Fatal("the accessibility slot was not filled")
	}
}

func TestOnlySessionRolesBecomeAgents(t *testing.T) {
	root := toolkit(t)
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
	root := toolkit(t)
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

func TestSettingsRegisterTheGuardOnceAndNoMCP(t *testing.T) {
	root := toolkit(t)
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
	root := toolkit(t)
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

func TestTheOldBridgeEntryIsRemovedOnlyWhenItNamesTheBridge(t *testing.T) {
	root := toolkit(t)
	plan, _ := Render(root, "komodo")
	for _, change := range plan.Changes {
		if change.Remove && filepath.Base(change.Path) == ".mcp.json" {
			t.Fatal("an absent bridge file was scheduled for removal")
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
		t.Fatal("the old bridge entry was not removed")
	}
}

func TestEverySkillIsCopied(t *testing.T) {
	root := toolkit(t)
	if got := body(t, root, filepath.Join(Dir, "skills", "run", "SKILL.md")); !strings.Contains(got, "komodo step") {
		t.Fatalf("skill = %q", got)
	}
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
}
