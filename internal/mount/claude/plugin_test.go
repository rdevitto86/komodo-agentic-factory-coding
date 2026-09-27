package claude

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/detect"
	"komodo/internal/hooks"
	"komodo/internal/install"
	"komodo/internal/mount"
)

func TestBuilderPluginSkillsIncludeBuildAndLanguageStandards(t *testing.T) {
	detected := detect.Profile{
		Languages: []string{"Go", "TypeScript", "Python"},
	}
	skills := []mount.Skill{
		{Name: "build", Body: "# Build\n"},
		{Name: "standards-go", Body: "# Go\n"},
		{Name: "standards-typescript", Body: "# TS\n"},
		{Name: "standards-python", Body: "# Python\n"},
		{Name: "standards-java", Body: "# Java\n"},
		{Name: "review-correctness", Body: "# Review\n"},
	}
	got := BuilderPluginSkills(detected, skills)
	if len(got) != 4 {
		t.Fatalf("got %d skills, want 4: %v", len(got), got)
	}
	wantSkills := []string{"build", "standards-go", "standards-python", "standards-typescript"}
	for i, want := range wantSkills {
		if got[i] != want {
			t.Fatalf("skill[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestBuilderPluginSkillsExcludesReviewSkills(t *testing.T) {
	detected := detect.Profile{
		Languages: []string{"Go"},
	}
	skills := []mount.Skill{
		{Name: "build", Body: "# Build\n"},
		{Name: "standards-go", Body: "# Go\n"},
		{Name: "review-correctness", Body: "# Review\n"},
		{Name: "review-security", Body: "# Security\n"},
		{Name: "respond", Body: "# Respond\n"},
		{Name: "run", Body: "# Run\n"},
		{Name: "plan", Body: "# Plan\n"},
	}
	got := BuilderPluginSkills(detected, skills)
	for _, skill := range got {
		if strings.Contains(skill, "review") || strings.Contains(skill, "respond") ||
			strings.Contains(skill, "run") || strings.Contains(skill, "plan") {
			t.Fatalf("builder plugin includes non-builder skill: %q", skill)
		}
	}
}

func TestBuilderPluginSkillsIsSorted(t *testing.T) {
	detected := detect.Profile{
		Languages: []string{"C++", "Go", "Rust"},
	}
	skills := []mount.Skill{
		{Name: "build", Body: "# Build\n"},
		{Name: "standards-cpp", Body: "# C++\n"},
		{Name: "standards-go", Body: "# Go\n"},
		{Name: "standards-rust", Body: "# Rust\n"},
	}
	got := BuilderPluginSkills(detected, skills)
	for i := 0; i < len(got)-1; i++ {
		if got[i] > got[i+1] {
			t.Fatalf("skills not sorted: %v", got)
		}
	}
}

func TestBuilderPluginSkillsWithNoDetectedLanguages(t *testing.T) {
	detected := detect.Profile{
		Languages: []string{},
	}
	skills := []mount.Skill{
		{Name: "build", Body: "# Build\n"},
		{Name: "standards-go", Body: "# Go\n"},
	}
	got := BuilderPluginSkills(detected, skills)
	if len(got) != 1 || got[0] != "build" {
		t.Fatalf("got %v, want only build skill", got)
	}
}

func TestBuilderPluginSkillsOmitsMissingStandards(t *testing.T) {
	detected := detect.Profile{
		Languages: []string{"Go", "Java"},
	}
	skills := []mount.Skill{
		{Name: "build", Body: "# Build\n"},
		{Name: "standards-go", Body: "# Go\n"},
		// standards-java is not in the skills list
	}
	got := BuilderPluginSkills(detected, skills)
	want := []string{"build", "standards-go"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("skill[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// TestRenderBuilderPluginListsExactlyThoseSkills verifies (REQ-16) that the rendered builder plugin
// directory holds exactly the skills BuilderPluginSkills names, and no others.
func TestRenderBuilderPluginListsExactlyThoseSkills(t *testing.T) {
	root := "/repo"
	detected := detect.Profile{Languages: []string{"Go"}}
	skills := []mount.Skill{
		{Name: "build", Body: "# Build\n"},
		{Name: "standards-go", Body: "# Go\n"},
		{Name: "review-correctness", Body: "# Review\n"},
		{Name: "plan", Body: "# Plan\n"},
	}

	var plan install.Plan
	owned := RenderBuilderPlugin(&plan, root, detected, skills)

	if len(owned) != 2 || !owned["build"] || !owned["standards-go"] {
		t.Fatalf("owned = %+v, want exactly build and standards-go", owned)
	}

	dir := filepath.Join(root, Dir, "plugins", "builder", "skills")
	for _, name := range []string{"build", "standards-go"} {
		want := filepath.Join(dir, name, "SKILL.md")
		found := false
		for _, change := range plan.Changes {
			if change.Path == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("builder plugin must render %q", want)
		}
	}
	for _, name := range []string{"review-correctness", "plan"} {
		for _, change := range plan.Changes {
			if strings.Contains(change.Path, name) {
				t.Fatalf("builder plugin must not render %q: %s", name, change.Path)
			}
		}
	}
}

func TestBuilderPluginSkillsNoDuplicates(t *testing.T) {
	detected := detect.Profile{
		Languages: []string{"Go"},
	}
	skills := []mount.Skill{
		{Name: "build", Body: "# Build\n"},
		{Name: "standards-go", Body: "# Go\n"},
		{Name: "standards-go", Body: "# Go again (shouldn't happen)\n"},
	}
	got := BuilderPluginSkills(detected, skills)
	if len(got) != 2 {
		t.Fatalf("got %d skills, want 2 (no duplicates): %v", len(got), got)
	}
	seen := map[string]bool{}
	for _, skill := range got {
		if seen[skill] {
			t.Fatalf("duplicate skill: %q", skill)
		}
		seen[skill] = true
	}
}

// pluginHooksFile is one rendered plugin hooks file, decoded.
type pluginHooksFile struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func TestEachRolePluginCarriesOnlyItsOwnHooks(t *testing.T) {
	t.Parallel()
	root := "/repo"
	var plan install.Plan
	RenderPluginHooks(&plan, root, "/bin/komodo")
	cases := []struct {
		role string
		want map[string][]string
	}{
		{"builder", map[string][]string{"PostToolUse": {"format", "timewarn"}, "Stop": {"taskchecks"}}},
		{"reviewer", map[string][]string{"PostToolUse": {"timewarn"}}},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			t.Parallel()
			var rendered pluginHooksFile
			path := filepath.Join(root, Dir, "plugins", tc.role, "hooks", "hooks.json")
			if err := json.Unmarshal(planBody(t, plan, path), &rendered); err != nil {
				t.Fatal(err)
			}
			if len(rendered.Hooks) != len(tc.want) {
				t.Fatalf("%s stages = %v, want %v", tc.role, rendered.Hooks, tc.want)
			}
			for event, names := range tc.want {
				entries := rendered.Hooks[event]
				if len(entries) != len(names) {
					t.Fatalf("%s %s = %+v, want %v", tc.role, event, entries, names)
				}
				for i, name := range names {
					command := entries[i].Hooks[0].Command
					if !strings.HasPrefix(command, "/bin/komodo hook "+name+" ") || entries[i].Hooks[0].Timeout <= 0 {
						t.Fatalf("%s %s hook %d = %+v, want %s with a timeout", tc.role, event, i, entries[i], name)
					}
					if strings.Contains(command, " guard") {
						t.Fatalf("the guard belongs to every session, not the %s plugin: %q", tc.role, command)
					}
				}
			}
		})
	}
}

func TestTheFormatHookMatchesOnlyEdits(t *testing.T) {
	t.Parallel()
	var plan install.Plan
	RenderPluginHooks(&plan, "/repo", "/bin/komodo")
	var rendered pluginHooksFile
	path := filepath.Join("/repo", Dir, "plugins", "builder", "hooks", "hooks.json")
	if err := json.Unmarshal(planBody(t, plan, path), &rendered); err != nil {
		t.Fatal(err)
	}
	format := rendered.Hooks["PostToolUse"][0]
	for _, tool := range strings.Split(format.Matcher, "|") {
		if !guardWriteTools[tool] {
			t.Fatalf("format matcher %q names %q, not an edit tool", format.Matcher, tool)
		}
	}
}

func TestHookOutcomeRendersWhatThisHostReads(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		event hooks.Event
		out   hooks.Outcome
		want  string
	}{
		{"a refused stop blocks", hooks.Stop, hooks.Outcome{Verdict: hooks.Refuse, Message: "fix it"},
			`{"decision":"block","reason":"fix it"}`},
		{"context after a tool", hooks.PostToolUse, hooks.Outcome{Verdict: hooks.Inform, Message: "lint"},
			`{"hookSpecificOutput":{"additionalContext":"lint","hookEventName":"PostToolUse"}}`},
		{"anything else falls back", hooks.PreToolUse, hooks.Outcome{Verdict: hooks.Refuse, Message: "no"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := string(hookOutcome(tc.event, tc.out)); got != tc.want {
				t.Fatalf("hookOutcome = %q, want %q", got, tc.want)
			}
		})
	}
}

// planBody returns the content the plan writes at path.
func planBody(t *testing.T, plan install.Plan, path string) []byte {
	t.Helper()
	for _, change := range plan.Changes {
		if change.Path == path {
			return change.Body
		}
	}
	t.Fatalf("the plan does not write %s", path)
	return nil
}

func TestPluginHooksRunARelativeBinaryFromTheMainCheckout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var plan install.Plan
	RenderPluginHooks(&plan, root, filepath.Join("bin", "komodo"))
	var rendered pluginHooksFile
	path := filepath.Join(root, Dir, "plugins", "reviewer", "hooks", "hooks.json")
	if err := json.Unmarshal(planBody(t, plan, path), &rendered); err != nil {
		t.Fatal(err)
	}
	command := rendered.Hooks["PostToolUse"][0].Hooks[0].Command
	binary := strings.Fields(command)[0]
	if !filepath.IsAbs(binary) || !strings.HasSuffix(binary, filepath.Join("bin", "komodo")) {
		t.Fatalf("hook command %q does not run an absolute binary", command)
	}
}

func TestRenderWritesEachRolePluginsHooksFile(t *testing.T) {
	root := toolkitRepo(t)
	plan, err := Render(root, "/opt/komodo/bin/komodo")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"builder", "reviewer"} {
		var rendered pluginHooksFile
		path := filepath.Join(root, Dir, "plugins", role, "hooks", "hooks.json")
		if err := json.Unmarshal(planBody(t, plan, path), &rendered); err != nil {
			t.Fatalf("%s hooks: %v", role, err)
		}
		if len(rendered.Hooks) == 0 {
			t.Fatalf("the %s plugin's hooks file mounts no hook", role)
		}
	}
}
