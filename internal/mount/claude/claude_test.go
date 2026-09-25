package claude

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"komodo/internal/mount"
	"komodo/internal/mount/ollama"
)

// toolkitRepo builds a root with the rules, two roles, a skill, and the policy, with the local machine down.
func toolkitRepo(t *testing.T) string {
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
	if got := body(t, root, "CLAUDE.md"); got != "@AGENTS.md\n"+claudeMDImport {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	rules := body(t, root, filepath.Join(Dir, "komodo", "AGENTS.md"))
	if strings.Contains(rules, "{{accessibility}}") {
		t.Fatal("the accessibility slot was not filled")
	}
}

func TestRenderAddsTheImportToAnExistingCLAUDEmd(t *testing.T) {
	root := toolkitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# My project\n\nOwn notes.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := body(t, root, "CLAUDE.md")
	if !strings.Contains(got, "Own notes.") {
		t.Fatalf("CLAUDE.md lost the repo's own content: %q", got)
	}
	if !strings.Contains(got, claudeMDImport) {
		t.Fatalf("CLAUDE.md is missing the rules import: %q", got)
	}
}

func TestRenderLeavesAnAlreadyImportingCLAUDEmdAlone(t *testing.T) {
	root := toolkitRepo(t)
	original := "# My project\n\n" + claudeMDImport
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := body(t, root, "CLAUDE.md"); got != original {
		t.Fatalf("CLAUDE.md = %q, want %q", got, original)
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
	t.Setenv(ollama.Env, "http://"+listener.Addr().String())
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

func TestSettingsDenyEditsToTheHostsOwnConfig(t *testing.T) {
	root := toolkitRepo(t)
	raw := body(t, root, filepath.Join(Dir, "settings.json"))
	for _, want := range []string{"Edit(~/.claude/**)", "Edit(~/.claude.json)", "Edit(.claude/settings.json)"} {
		if !strings.Contains(raw, `"`+want+`"`) {
			t.Fatalf("settings deny no %s, so the hook's own config is left to the guard alone:\n%s", want, raw)
		}
	}
}

func TestHeadlessSandboxesOnlyWhenTheOverlayOptsIn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, args := Headless("run", "TG-01.1")
	if strings.Contains(strings.Join(args, " "), "--settings") {
		t.Fatalf("args = %v; with no overlay the run is not sandboxed", args)
	}
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := `{"sandbox":true,"sandbox_write":["~/go/pkg/mod"],"sandbox_domains":["proxy.golang.org"]}`
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	_, args = Headless("run", "TG-01.1")
	if len(args) < 2 || args[len(args)-2] != "--settings" {
		t.Fatalf("args = %v; an opted-in run passes the sandbox as inline settings", args)
	}
	var settings struct {
		Sandbox struct {
			Enabled                  bool  `json:"enabled"`
			FailIfUnavailable        bool  `json:"failIfUnavailable"`
			AllowUnsandboxedCommands *bool `json:"allowUnsandboxedCommands"`
			Filesystem               struct {
				AllowWrite []string `json:"allowWrite"`
			} `json:"filesystem"`
			Network struct {
				AllowedDomains []string `json:"allowedDomains"`
			} `json:"network"`
		} `json:"sandbox"`
	}
	if err := json.Unmarshal([]byte(args[len(args)-1]), &settings); err != nil {
		t.Fatal(err)
	}
	s := settings.Sandbox
	if !s.Enabled || !s.FailIfUnavailable || s.AllowUnsandboxedCommands == nil || *s.AllowUnsandboxedCommands ||
		len(s.Filesystem.AllowWrite) != 1 || len(s.Network.AllowedDomains) != 1 {
		t.Fatalf("sandbox = %s", args[len(args)-1])
	}
}

func TestTheHookCommandIsAnAbsolutePath(t *testing.T) {
	root := toolkitRepo(t)
	raw := body(t, root, filepath.Join(Dir, "settings.json"))
	var settings struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("settings are not JSON: %v", err)
	}
	command := settings.Hooks.PreToolUse[0].Hooks[0].Command
	if !filepath.IsAbs(strings.TrimSuffix(command, " guard")) {
		t.Fatalf("command %q is not an absolute path; a cd would lose the hook", command)
	}
	matcher := settings.Hooks.PreToolUse[0].Matcher
	for _, want := range []string{"Bash", "Edit", "Write", "MultiEdit", "NotebookEdit", "Agent", "Task"} {
		if !strings.Contains(matcher, want) {
			t.Fatalf("matcher %q is missing %q", matcher, want)
		}
	}
}

func TestTheHookCommandStaysAbsoluteWhenTheBinaryAlreadyIs(t *testing.T) {
	root := toolkitRepo(t)
	plan, err := Render(root, "/opt/komodo/bin/komodo")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if change.Path == filepath.Join(root, Dir, "settings.json") {
			if !strings.Contains(string(change.Body), "/opt/komodo/bin/komodo guard") {
				t.Fatalf("settings.json = %s", change.Body)
			}
			return
		}
	}
	t.Fatal("settings.json is not in the plan")
}

func TestTheHookCommandIsTheRunningBinaryInAForeignRepo(t *testing.T) {
	name := "komodo-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	toolkitBinary := filepath.Join(t.TempDir(), name)
	saved := mount.Executable
	t.Cleanup(func() { mount.Executable = saved })
	mount.Executable = func() (string, error) { return toolkitBinary, nil }
	root := toolkitRepo(t)
	plan, err := Render(root, mount.BinaryPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if change.Path == filepath.Join(root, Dir, "settings.json") {
			want, _ := json.Marshal(toolkitBinary + " guard")
			if !strings.Contains(string(change.Body), string(want)) {
				t.Fatalf("settings.json = %s, want the hook %s", change.Body, want)
			}
			return
		}
	}
	t.Fatal("settings.json is not in the plan")
}

func TestOldHooksFilesAreRemovedButAUsersOwnHookSurvives(t *testing.T) {
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
	write(filepath.Join(Dir, "hooks", "guard.py"), "old")
	write(filepath.Join(Dir, "hooks", "my-own-hook.sh"), "mine")
	write(filepath.Join(Dir, "commands", "my-command.md"), "mine")
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, Dir, "hooks", "guard.py")); !os.IsNotExist(err) {
		t.Fatal("the old render's guard.py was not removed")
	}
	if _, err := os.Stat(filepath.Join(root, Dir, "hooks", "my-own-hook.sh")); err != nil {
		t.Fatal("a user's own hook script was removed")
	}
	if _, err := os.Stat(filepath.Join(root, Dir, "commands", "my-command.md")); err != nil {
		t.Fatal("a user's own command was removed")
	}
}

func TestOnlyTheDetectedProfilesStandardsRender(t *testing.T) {
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
	write("komodo/skills/standards-go/SKILL.md", "---\nname: standards-go\nglobs: [\"**/*.go\"]\n---\n\nGo rules.\n")
	write("komodo/skills/standards-python/SKILL.md", "---\nname: standards-python\nglobs: [\"**/*.py\"]\n---\n\nPython rules.\n")
	write("komodo/skills/standards-comments/SKILL.md", "---\nname: standards-comments\nglobs: []\n---\n\nComment rules.\n")
	write("main.go", "package main\n")
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, change := range plan.Changes {
		if strings.Contains(change.Path, filepath.Join(Dir, "skills", "standards-")) {
			names = append(names, filepath.Base(filepath.Dir(change.Path)))
		}
	}
	if !contains(names, "standards-go") {
		t.Fatalf("standards-go did not render: %v", names)
	}
	if !contains(names, "standards-comments") {
		t.Fatalf("a glob-less standard did not render: %v", names)
	}
	if contains(names, "standards-python") {
		t.Fatalf("an undetected language's standard rendered: %v", names)
	}
}

func TestARepoOverrideForcesAnUndetectedStandardToRender(t *testing.T) {
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
	write("komodo/skills/standards-python/SKILL.md", "---\nname: standards-python\nglobs: [\"**/*.py\"]\n---\n\nPython rules.\n")
	write(".komodo/standards/python.md", "This repo also bans bare excepts.\n")
	got := body(t, root, filepath.Join(Dir, "skills", "standards-python", "SKILL.md"))
	if !strings.Contains(got, "bare excepts") {
		t.Fatalf("the repo override did not force the undetected standard to render: %q", got)
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

func TestOllamaTakesTheLightTierAndNeverTheReviewerUnasked(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(ollama.ModelEnv, "coder:3b")
	tiers := Tiers("max_5x", true)
	if tiers.Light.Provider != "ollama" || tiers.Light.Model != "coder:3b" {
		t.Fatalf("tiers = %+v", tiers)
	}
	if tiers.Standard.Provider == "ollama" || tiers.Reviewer.Provider == "ollama" {
		t.Fatalf("a write or review tier was moved to the local machine: %+v", tiers)
	}
}

// writeRecall writes ~/.komodo/recall.json for model with the given recall and case count.
func writeRecall(t *testing.T, home, model string, recall float64, cases int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{%q:{"recall":%v,"cases":%d}}`, model, recall, cases)
	if err := os.WriteFile(filepath.Join(home, ".komodo", "recall.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOverlayMovesTheReviewerToOllamaAndRenamesATier(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(ollama.ModelEnv, "")
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := `{"local_model":"coder:7b","local_reviewer":true,"models":{"heavy":"sonnet"}}`
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRecall(t, home, "coder:7b", 0.8, 15)
	tiers := Tiers("max_5x", true)
	if tiers.Reviewer.Provider != "ollama" || tiers.Reviewer.Model != "coder:7b" {
		t.Fatalf("reviewer = %+v", tiers.Reviewer)
	}
	if tiers.Heavy.Model != "sonnet" {
		t.Fatalf("heavy = %+v", tiers.Heavy)
	}
	if got := agentFile(mount.Role{Name: "architect", Tier: "heavy"}, false); !strings.Contains(got, "model: sonnet") {
		t.Fatalf("the agent file did not carry the overlay's model:\n%s", got)
	}
}

func TestNoRecallFileKeepsReviewRemote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(ollama.ModelEnv, "coder:7b")
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := `{"local_reviewer":true}`
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	tiers := Tiers("max_5x", true)
	if tiers.Reviewer.Provider == "ollama" {
		t.Fatalf("reviewer moved local with no recall on record: %+v", tiers.Reviewer)
	}
}

func TestRecallUnderTenCasesKeepsReviewRemote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(ollama.ModelEnv, "coder:7b")
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := `{"local_reviewer":true}`
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRecall(t, home, "coder:7b", 0.8, 5)
	tiers := Tiers("max_5x", true)
	if tiers.Reviewer.Provider == "ollama" {
		t.Fatalf("reviewer moved local on 5 cases, under the 10-case floor: %+v", tiers.Reviewer)
	}
}

func TestAnOverlayBarAboveTheDefaultKeepsReviewRemote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(ollama.ModelEnv, "coder:7b")
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := `{"local_reviewer":true,"local_reviewer_recall":0.9}`
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRecall(t, home, "coder:7b", 0.8, 15)
	tiers := Tiers("max_5x", true)
	if tiers.Reviewer.Provider == "ollama" {
		t.Fatalf("reviewer moved local under the overlay's raised bar: %+v", tiers.Reviewer)
	}
}

func TestReviewerWhyNamesTheGapOrTheMissingRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := ReviewerWhy("coder:7b", "opus"); !strings.Contains(got, "no recall on record for coder:7b") {
		t.Fatalf("why = %q", got)
	}
	writeRecall(t, home, "coder:7b", 0.42, 15)
	if got := ReviewerWhy("coder:7b", "opus"); !strings.Contains(got, "0.42 over 15 cases") || !strings.Contains(got, "stays on opus") {
		t.Fatalf("why = %q", got)
	}
	writeRecall(t, home, "coder:7b", 0.8, 15)
	if got := ReviewerWhy("coder:7b", "opus"); !strings.Contains(got, "moves to coder:7b") {
		t.Fatalf("why = %q", got)
	}
}

func TestAStandardRendersWhenItsFolderGlobMatchesARealFile(t *testing.T) {
	root := toolkitRepo(t)
	for rel, contents := range map[string]string{
		"komodo/skills/standards-specs/SKILL.md": "---\nname: standards-specs\nglobs: [\"**/SDD.md\"]\n---\n\nSpec rules.\n",
		"komodo/skills/standards-api/SKILL.md":   "---\nname: standards-api\nglobs: [\"api/**\"]\n---\n\nAPI rules.\n",
		"docs/spec/SDD.md":                       "# Design\n",
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := Render(root, "komodo")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, change := range plan.Changes {
		if strings.Contains(change.Path, filepath.Join(Dir, "skills", "standards-")) {
			names = append(names, filepath.Base(filepath.Dir(change.Path)))
		}
	}
	if !contains(names, "standards-specs") {
		t.Fatalf("a standard whose glob matches docs/spec/SDD.md did not render: %v", names)
	}
	if contains(names, "standards-api") {
		t.Fatalf("a standard whose glob matches no file rendered: %v", names)
	}
}

func TestAWorktreesHookPointsAtTheMainCheckoutsBinary(t *testing.T) {
	root := toolkitRepo(t)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"config", "user.email", "t@e.st"}, {"config", "user.name", "t"},
		{"add", "-A"}, {"commit", "-q", "-m", "seed"}, {"worktree", "add", "-q", "-b", "task/x", filepath.Join(root, ".komodo", "wt", "x")},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	worktree := filepath.Join(root, ".komodo", "wt", "x")
	raw, err := settingsFile(worktree, "bin/komodo")
	if err != nil {
		t.Fatal(err)
	}
	main, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), filepath.Join(main, "bin", "komodo")+" guard") {
		t.Fatalf("settings = %s; a worktree's hook must run the main checkout's binary, which a worktree lacks", raw)
	}
}

func TestHeadlessBypassesPromptsAndDrivesOnTheStandardTier(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	name, args := Headless("run", "TG-01.1")
	joined := strings.Join(args, " ")
	if name != "claude" || !strings.Contains(joined, "-p /run TG-01.1") {
		t.Fatalf("command = %s %s", name, joined)
	}
	if !strings.Contains(joined, "--permission-mode bypassPermissions") {
		t.Fatalf("a headless run cannot answer a prompt: %s", joined)
	}
	if !strings.Contains(joined, "--model "+models["standard"]) {
		t.Fatalf("the driver did not take the standard tier: %s", joined)
	}
}

func TestLeftoversNamesARetiredHookAndAllowRuleOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	settings := `{
  "hooks": {
    "SessionStart": [{"matcher": "", "hooks": [{"type": "command", "command": "/h/.claude/hooks/komodo-hooks inject"}]}],
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "/repo/bin/komodo-darwin-arm64 guard"}]}]
  },
  "permissions": {"allow": ["Bash(python3 -m komodo:*)", "Bash(go test:*)"]}
}`
	if err := os.WriteFile(path, []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	found := leftoversIn(path)
	if len(found) != 2 || !strings.Contains(found[0], "SessionStart") || !strings.Contains(found[1], "python3 -m komodo") {
		t.Fatalf("found = %q; want the prototype hook and allow rule, never the current guard", found)
	}
	if got := leftoversIn(filepath.Join(t.TempDir(), "missing.json")); got != nil {
		t.Fatalf("a missing file = %q, want nothing", got)
	}
}

// TestSettingsTurnAttributionOff checks the rendered settings hide the trailer, the PR footer, and the session link.
func TestSettingsTurnAttributionOff(t *testing.T) {
	worktree := t.TempDir()
	raw, err := settingsFile(worktree, "bin/komodo")
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Attribution struct {
			Commit     *string `json:"commit"`
			PR         *string `json:"pr"`
			SessionURL *bool   `json:"sessionUrl"`
		} `json:"attribution"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	a := settings.Attribution
	if a.Commit == nil || *a.Commit != "" || a.PR == nil || *a.PR != "" || a.SessionURL == nil || *a.SessionURL {
		t.Fatalf("attribution = %s; commit and pr must be empty and sessionUrl false, since true appends the session link", raw)
	}
}
