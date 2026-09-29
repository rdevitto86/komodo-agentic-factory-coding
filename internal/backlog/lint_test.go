package backlog

import (
	"strings"
	"testing"
)

func TestLintRejectsAGroupWithMoreThan12Tasks(t *testing.T) {
	text := "### [TG-40.1] Large group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"
	for i := 1; i <= 13; i++ {
		text += "#### [TSK-40.1." + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "] Task " + string(rune('0'+i)) + " [P: C] [READY]\n"
		text += "```yaml\nfiles: [a/t" + string(rune('0'+i)) + ".go]\ndone_when: [\"go test ./...\"]\n```\n\n"
	}
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-40.1") && strings.Contains(problem, "exceeds limit of 12") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no problem mentions exceeding 12 tasks; got %v", problems)
	}
}

func TestLintAcceptsAGroupWith12Tasks(t *testing.T) {
	text := "### [TG-41.1] Exactly 12 tasks\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"
	for i := 1; i <= 12; i++ {
		text += "#### [TSK-41.1." + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "] Task " + string(rune('0'+i)) + " [P: C] [READY]\n"
		text += "```yaml\nfiles: [a/t" + string(rune('0'+i)) + ".go]\ndone_when: [\"go test ./...\"]\n```\n\n"
	}
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "exceeds limit of 12") {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("12 tasks should be allowed; got %v", problems)
	}
}

func TestLintRejectsABuildTaskWithLightTier(t *testing.T) {
	text := "### [TG-42.1] Build test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-42.1.1] Build with light [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"type: build\ntier: light\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TSK-42.1.1") && strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no problem mentions build task with light tier; got %v", problems)
	}
}

func TestLintAcceptsBuildTaskWithStandardTier(t *testing.T) {
	text := "### [TG-43.1] Build test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-43.1.1] Build with standard [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"type: build\ntier: standard\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("build task with standard tier should be allowed; got %v", problems)
	}
}

func TestLintAcceptsBuildTaskWithHeavyTier(t *testing.T) {
	text := "### [TG-44.1] Build test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-44.1.1] Build with heavy [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"type: build\ntier: heavy\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("build task with heavy tier should be allowed; got %v", problems)
	}
}

func TestLintRejectsAFeatTaskWithLightTier(t *testing.T) {
	text := "### [TG-45.1] Feat test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-45.1.1] Feat with light [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"tier: light\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TSK-45.1.1") && strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no problem mentions feat task with light tier; got %v", problems)
	}
}

func TestLintAcceptsBuildTaskWithNoTier(t *testing.T) {
	text := "### [TG-46.1] Build test\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-46.1.1] Build with no tier [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n" +
		"type: build\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "cannot have tier light") {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("build task with no tier should be allowed; got %v", problems)
	}
}

func TestLintLeavesFiledFindingsOutOfTheTaskCap(t *testing.T) {
	text := "### [TG-42.1] Four tasks and twelve filed findings\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"
	for i := 1; i <= 16; i++ {
		status := "DONE"
		if i > 4 {
			status = "REFINEMENT"
		}
		text += "#### [TSK-42.1." + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "] Task " + string(rune('0'+i)) + " [P: L] [" + status + "]\n"
		text += "```yaml\nfiles: [a/t" + string(rune('0'+i)) + ".go]\ndone_when: [\"go test ./...\"]\n```\n\n"
	}
	for _, problem := range Lint(Parse(text)) {
		if strings.Contains(problem, "exceeds limit of 12") {
			t.Fatalf("filed REFINEMENT findings must not count toward the cap; got %q", problem)
		}
	}
}

func TestLintAcceptsAGroupWhoseVersionMatchesItsEpic(t *testing.T) {
	text := "## [EPIC-50] Phase Ships as `1.0.0`\n\n" +
		"### [TG-50.1] Matching version\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-50.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	for _, problem := range problems {
		if strings.Contains(problem, "TG-50.1") && strings.Contains(problem, "differs from epic") {
			t.Fatalf("matching version should not produce an error; got %q", problem)
		}
	}
}

func TestLintRejectsAGroupWhoseVersionDiffersFromItsEpic(t *testing.T) {
	text := "## [EPIC-51] Phase Ships as `1.0.0`\n\n" +
		"### [TG-51.1] Mismatched version\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-51.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-51.1") && strings.Contains(problem, "differs from epic") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mismatched version should produce an error; got %v", problems)
	}
}

func TestLintAcceptsAGroupWhoseBaseIsItsOwnEpicBranch(t *testing.T) {
	text := "### [TG-53.1] Base on own epic branch\n```yaml\ntype: feat\nversion: 1.0.0\nbase: feat/1.0.0\n```\n\n" +
		"#### [TSK-53.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	for _, problem := range problems {
		if strings.Contains(problem, "TG-53.1") && strings.Contains(problem, "neither main") {
			t.Fatalf("base naming the group's own epic branch should not produce an error; got %q", problem)
		}
	}
}

func TestLintRejectsAGroupWhoseBaseNamesAnotherEpicsBranch(t *testing.T) {
	text := "### [TG-54.1] Base on a different epic's branch\n```yaml\ntype: feat\nversion: 1.0.0\nbase: feat/v2.0.0\n```\n\n" +
		"#### [TSK-54.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-54.1") && strings.Contains(problem, "neither main") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("base naming a different epic's branch should produce an error; got %v", problems)
	}
}

func TestLintRejectsAGroupInAnEpicWithNoVersion(t *testing.T) {
	text := "## [EPIC-52] Phase with no version\n\n" +
		"### [TG-52.1] Group in unversioned epic\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-52.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-52.1") && strings.Contains(problem, "epic") && strings.Contains(problem, "has no version") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("group in unversioned epic should produce an error; got %v", problems)
	}
}

func TestLintRejectsAVersionWithAPrereleaseNotOneOfTheFourPhases(t *testing.T) {
	text := "### [TG-55.1] Unknown phase\n```yaml\ntype: feat\nversion: 1.0.0-dev.1\n```\n\n" +
		"#### [TSK-55.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-55.1") && strings.Contains(problem, "alpha") &&
			strings.Contains(problem, "beta") && strings.Contains(problem, "rc") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("prerelease dev.1 should be refused and name the four phases; got %v", problems)
	}
}

func TestLintRejectsABetaVersionWithNoNumber(t *testing.T) {
	text := "### [TG-56.1] Beta without a number\n```yaml\ntype: feat\nversion: 1.0.0-beta\n```\n\n" +
		"#### [TSK-56.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-56.1") && strings.Contains(problem, "alpha") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("beta with no number should be refused; got %v", problems)
	}
}

func TestSlugMatchesGitHubOnANumberedHeading(t *testing.T) {
	if got := Slug("6.1 X"); got != "61-x" {
		t.Fatalf("Slug(%q) = %q, want %q", "6.1 X", got, "61-x")
	}
}

func TestLintAcceptsAStableVersionWhoseEpicHadNoRc(t *testing.T) {
	text := "## [EPIC-57] Beta phase Ships as `1.0.0-beta.2`\n\n" +
		"### [TG-57.1] Beta group\n```yaml\ntype: feat\nversion: 1.0.0-beta.2\n```\n\n" +
		"#### [TSK-57.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n\n" +
		"## [EPIC-58] Stable phase Ships as `1.0.0`\n\n" +
		"### [TG-58.1] Stable group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-58.1.1] Task [P: C] [READY]\n```yaml\nfiles: [b.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	for _, problem := range problems {
		if strings.Contains(problem, "TG-57.1") || strings.Contains(problem, "TG-58.1") {
			t.Fatalf("a beta group followed by a stable group with no rc should lint cleanly; got %v", problems)
		}
	}
}
