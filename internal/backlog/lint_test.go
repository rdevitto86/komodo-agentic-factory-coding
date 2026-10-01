package backlog

import (
	"os"
	"path/filepath"
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
		if strings.Contains(problem, "EPIC-51") && strings.Contains(problem, "TG-51.1") && strings.Contains(problem, "split the epic per version") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mismatched version should produce an error; got %v", problems)
	}
}

func TestLintReportsOneMismatchPerEpicNamingEveryDifferingGroup(t *testing.T) {
	text := "## [EPIC-59] Phase Ships as `1.0.0`\n\n" +
		"### [TG-59.1] First mismatched group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-59.1.1] Task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n\n" +
		"### [TG-59.2] Second mismatched group\n```yaml\ntype: feat\nversion: 3.0.0\n```\n\n" +
		"#### [TSK-59.2.1] Task [P: C] [READY]\n```yaml\nfiles: [b.go]\ndone_when: [\"go test ./...\"]\n```\n"
	problems := Lint(Parse(text))
	count := 0
	for _, problem := range problems {
		if strings.Contains(problem, "EPIC-59") {
			count++
			if !strings.Contains(problem, "TG-59.1") || !strings.Contains(problem, "TG-59.2") {
				t.Fatalf("epic mismatch problem should name every differing group; got %q", problem)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected one problem for EPIC-59, got %d: %v", count, problems)
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

func TestLintVersionsRejectsAnOpenGroupAtTheNewestTag(t *testing.T) {
	text := "### [TG-70.1] Already tagged [P: C] [REFINEMENT]\n```yaml\ntype: feat\nversion: 1.2.3\n```\n\n"
	problems := LintVersions(Parse(text), []string{"v1.2.3"})
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-70.1") && strings.Contains(problem, "1.2.3") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a problem naming the group and the tag; got %v", problems)
	}
}

func TestLintVersionsRejectsAPrereleaseOfAnAlreadyTaggedVersion(t *testing.T) {
	text := "### [TG-71.1] Prerelease of tagged [P: C] [REFINEMENT]\n```yaml\ntype: feat\nversion: 1.2.3-beta.1\n```\n\n"
	problems := LintVersions(Parse(text), []string{"v1.2.3"})
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-71.1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a problem for a prerelease behind the newest tag; got %v", problems)
	}
}

func TestLintVersionsAcceptsAnOpenGroupAheadOfTheNewestTag(t *testing.T) {
	text := "### [TG-72.1] Ahead of the tag [P: C] [REFINEMENT]\n```yaml\ntype: feat\nversion: 1.3.0\n```\n\n"
	problems := LintVersions(Parse(text), []string{"v1.2.3"})
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestLintVersionsSkipsTheCheckWithNoTags(t *testing.T) {
	text := "### [TG-73.1] No tags yet [P: C] [REFINEMENT]\n```yaml\ntype: feat\nversion: 0.0.1\n```\n\n"
	if problems := LintVersions(Parse(text), nil); len(problems) != 0 {
		t.Fatalf("problems = %v, want none with no tags", problems)
	}
}

func TestLintVersionsSkipsAGroupWhoseTasksAreAllDone(t *testing.T) {
	text := "### [TG-74.1] Already shipped [P: C] [REFINEMENT]\n```yaml\ntype: feat\nversion: 1.2.3\n```\n\n" +
		"#### [TSK-74.1.1] Done task [P: C] [DONE]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	if problems := LintVersions(Parse(text), []string{"v1.2.3"}); len(problems) != 0 {
		t.Fatalf("problems = %v, want none for a group whose tasks are all done", problems)
	}
}

func TestLintGroupFileRejectsAMissingVersion(t *testing.T) {
	file := ParseGroupFile("## [TG-60.1] No version [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-60\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-60.1.1** A task\n  - files: `a.go`\n")
	problems := LintGroupFile(".", file, "", map[string]bool{"TG-60.1": true}, map[string]bool{"TSK-60.1.1": true}, nil)
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "no version") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a no-version problem; got %v", problems)
	}
}

// TestLintGroupFileAcceptsARefinementTaskWithNoFiles proves a group still being planned does not
// demand a task's files, matching the legacy grammar's own exemption.
func TestLintGroupFileAcceptsARefinementTaskWithNoFiles(t *testing.T) {
	file := ParseGroupFile("## [TG-66.1] Still planning [P: H] [REFINEMENT]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-66\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-66.1.1** A task with no files yet\n")
	problems := LintGroupFile(".", file, "", map[string]bool{"TG-66.1": true}, map[string]bool{"TSK-66.1.1": true}, nil)
	for _, problem := range problems {
		if strings.Contains(problem, "no files") {
			t.Fatalf("a REFINEMENT task should not need files; got %v", problems)
		}
	}
}

func TestLintGroupFileRejectsAnOpenTaskWithNoFiles(t *testing.T) {
	file := ParseGroupFile("## [TG-61.1] Task with no files [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-61\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-61.1.1** A task with no files\n")
	problems := LintGroupFile(".", file, "", map[string]bool{"TG-61.1": true}, map[string]bool{"TSK-61.1.1": true}, nil)
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TSK-61.1.1") && strings.Contains(problem, "no files") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want an open task with no files problem; got %v", problems)
	}
}

func TestLintGroupFileAcceptsADoneTaskWithNoFiles(t *testing.T) {
	file := ParseGroupFile("## [TG-62.1] Done task with no files [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-62\ndepends_on: []\n```\n\n" +
		"- [x] **TSK-62.1.1** A finished task\n")
	problems := LintGroupFile(".", file, "", map[string]bool{"TG-62.1": true}, map[string]bool{"TSK-62.1.1": true}, nil)
	for _, problem := range problems {
		if strings.Contains(problem, "no files") {
			t.Fatalf("a done task should not need files; got %v", problems)
		}
	}
}

// TestLintGroupFileRejectsAReadyAgentTaskWithNoDoneWhen proves a READY task needs at least one
// done_when command, matching Lint's check on the legacy grammar.
func TestLintGroupFileRejectsAReadyAgentTaskWithNoDoneWhen(t *testing.T) {
	file := ParseGroupFile("## [TG-64.1] No done_when [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-64\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-64.1.1** A task\n  - files: `a.go`\n")
	problems := LintGroupFile(".", file, "", map[string]bool{"TG-64.1": true}, map[string]bool{"TSK-64.1.1": true}, nil)
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TSK-64.1.1") && strings.Contains(problem, "no done_when") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a no-done_when problem; got %v", problems)
	}
}

// TestLintGroupFileAcceptsAHumanTaskWithNoDoneWhen proves the done_when check exempts a human owner.
func TestLintGroupFileAcceptsAHumanTaskWithNoDoneWhen(t *testing.T) {
	file := ParseGroupFile("## [TG-65.1] Human owner [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-65\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-65.1.1** A task\n  - files: `a.go`\n  - owner: human\n")
	problems := LintGroupFile(".", file, "", map[string]bool{"TG-65.1": true}, map[string]bool{"TSK-65.1.1": true}, nil)
	for _, problem := range problems {
		if strings.Contains(problem, "no done_when") {
			t.Fatalf("a human task should not need done_when; got %v", problems)
		}
	}
}

func TestLintGroupFileRejectsADependsOnNamingNoGroupOrTask(t *testing.T) {
	file := ParseGroupFile("## [TG-63.1] Bad dependency [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-63\ndepends_on: [TG-99.9]\n```\n\n" +
		"- [ ] **TSK-63.1.1** A task\n  - files: `a.go`\n")
	problems := LintGroupFile(".", file, "", map[string]bool{"TG-63.1": true}, map[string]bool{"TSK-63.1.1": true}, nil)
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "depends_on names unknown") && strings.Contains(problem, "TG-99.9") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a problem naming the unknown dependency; got %v", problems)
	}
}

func TestLintGroupFileAcceptsADependsOnNamingAKnownTask(t *testing.T) {
	file := ParseGroupFile("## [TG-64.1] Task dependency [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-64\ndepends_on: [TSK-01.1.1]\n```\n\n" +
		"- [ ] **TSK-64.1.1** A task\n  - files: `a.go`\n")
	problems := LintGroupFile(".", file, "", map[string]bool{"TG-64.1": true}, map[string]bool{"TSK-01.1.1": true, "TSK-64.1.1": true}, nil)
	for _, problem := range problems {
		if strings.Contains(problem, "depends_on names unknown") {
			t.Fatalf("a depends_on naming a known task should not produce an error; got %v", problems)
		}
	}
}

func TestLintGroupFileRejectsADependsOnNamingALaterVersionGroup(t *testing.T) {
	file := ParseGroupFile("## [TG-68.1] Early group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0-alpha.1\nepic: EPIC-68\ndepends_on: [TG-68.2]\n```\n\n" +
		"- [ ] **TSK-68.1.1** A task\n  - files: `a.go`\n")
	groupIDs := map[string]bool{"TG-68.1": true, "TG-68.2": true}
	versions := map[string]string{"TG-68.1": "1.0.0-alpha.1", "TG-68.2": "1.0.0-alpha.2"}
	problems := LintGroupFile(".", file, "", groupIDs, map[string]bool{"TSK-68.1.1": true}, versions)
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-68.2") && strings.Contains(problem, "later version") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no problem names the later-version dependency; got %v", problems)
	}
}

func TestLintGroupFileAcceptsADependsOnNamingAnEarlierVersionGroup(t *testing.T) {
	file := ParseGroupFile("## [TG-69.1] Late group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0-alpha.2\nepic: EPIC-69\ndepends_on: [TG-69.2]\n```\n\n" +
		"- [ ] **TSK-69.1.1** A task\n  - files: `a.go`\n")
	groupIDs := map[string]bool{"TG-69.1": true, "TG-69.2": true}
	versions := map[string]string{"TG-69.1": "1.0.0-alpha.2", "TG-69.2": "1.0.0-alpha.1"}
	problems := LintGroupFile(".", file, "", groupIDs, map[string]bool{"TSK-69.1.1": true}, versions)
	for _, problem := range problems {
		if strings.Contains(problem, "later version") {
			t.Fatalf("depending on an earlier version should not produce an error; got %v", problems)
		}
	}
}

func TestLintGroupFileNamesAContextAnchorWithNoHeading(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "docs.md"), []byte("# Real heading\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text := "## [TG-65.1] Bad anchor [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-65\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-65.1.1** A task\n  - files: `a.go`\n  - context: `docs.md#no-such-heading`\n"
	file := ParseGroupFile(text)
	problems := LintGroupFile(root, file, text, map[string]bool{"TG-65.1": true}, map[string]bool{"TSK-65.1.1": true}, nil)
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "names no heading") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a problem naming the missing heading; got %v", problems)
	}
}

func TestLintGroupFileAcceptsAContextAnchorWithAMatchingHeading(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "docs.md"), []byte("# Real heading\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text := "## [TG-66.1] Good anchor [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-66\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-66.1.1** A task\n  - files: `a.go`\n  - context: `docs.md#real-heading`\n"
	file := ParseGroupFile(text)
	problems := LintGroupFile(root, file, text, map[string]bool{"TG-66.1": true}, map[string]bool{"TSK-66.1.1": true}, nil)
	for _, problem := range problems {
		if strings.Contains(problem, "names no heading") {
			t.Fatalf("a context anchor matching a heading should not produce an error; got %v", problems)
		}
	}
}

func TestLintGroupFileEpicsReportsOneProblemPerEpic(t *testing.T) {
	first := ParseGroupFile("## [TG-67.1] First [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-67\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-67.1.1** A task\n  - files: `a.go`\n")
	second := ParseGroupFile("## [TG-67.2] Second [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 2.0.0\nepic: EPIC-67\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-67.2.1** A task\n  - files: `b.go`\n")
	problems := LintGroupFileEpics([]GroupFile{first, second})
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want exactly one", problems)
	}
	if !strings.Contains(problems[0], "EPIC-67") || !strings.Contains(problems[0], "TG-67.2") {
		t.Fatalf("problem = %q, want it to name EPIC-67 and TG-67.2", problems[0])
	}
}

func TestSlugMatchesGitHubOnANumberedHeading(t *testing.T) {
	if got := Slug("6.1 X"); got != "61-x" {
		t.Fatalf("Slug(%q) = %q, want %q", "6.1 X", got, "61-x")
	}
}

func TestNotesFlagsAReadyTaskWhoseDoneWhenOnlyTestsItsOwnPackage(t *testing.T) {
	text := "### [TG-80.1] No caller\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-80.1.1] Task with no caller [P: C] [READY]\n```yaml\n" +
		"files: [internal/widget/widget.go, internal/widget/widget_test.go]\n" +
		"done_when: [\"go test ./internal/widget/...\"]\n```\n"
	notes := Notes(Parse(text))
	found := false
	for _, note := range notes {
		if strings.Contains(note, "TSK-80.1.1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a note naming TSK-80.1.1; got %v", notes)
	}
}

func TestNotesAcceptsAReadyTaskWhoseFilesNameACaller(t *testing.T) {
	text := "### [TG-81.1] Has a caller\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-81.1.1] Task with a caller [P: C] [READY]\n```yaml\n" +
		"files: [internal/widget/widget.go, cmd/komodo/widget.go]\n" +
		"done_when: [\"go test ./internal/widget/...\"]\n```\n"
	notes := Notes(Parse(text))
	for _, note := range notes {
		if strings.Contains(note, "TSK-81.1.1") {
			t.Fatalf("a task naming a caller in files should not be noted; got %v", notes)
		}
	}
}

func TestNotesGroupFileFlagsAReadyTaskWhoseDoneWhenOnlyTestsItsOwnPackage(t *testing.T) {
	file := ParseGroupFile("## [TG-82.1] No caller [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-82\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-82.1.1** A task with no caller\n  - files: `internal/widget/widget.go`, `internal/widget/widget_test.go`\n" +
		"  - done_when: `go test ./internal/widget/...`\n")
	notes := NotesGroupFile(file)
	found := false
	for _, note := range notes {
		if strings.Contains(note, "TSK-82.1.1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a note naming TSK-82.1.1; got %v", notes)
	}
}

func TestNotesGroupFileAcceptsAReadyTaskWhoseDoneWhenRunsTheRealCommand(t *testing.T) {
	file := ParseGroupFile("## [TG-83.1] Has a caller [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-83\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-83.1.1** A task proved by the real command\n  - files: `internal/widget/widget.go`, `internal/widget/widget_test.go`\n" +
		"  - done_when: `go test ./internal/widget/...`, `go run ./cmd/komodo doctor`\n")
	notes := NotesGroupFile(file)
	for _, note := range notes {
		if strings.Contains(note, "TSK-83.1.1") {
			t.Fatalf("a task proved by the real command should not be noted; got %v", notes)
		}
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
