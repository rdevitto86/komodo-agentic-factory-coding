package claude

import (
	"strings"
	"testing"

	"komodo/internal/detect"
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
