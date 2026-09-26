package claude

import (
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/detect"
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
