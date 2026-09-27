package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckEpicsFlagsEpicWithReadyGroupsButMissingBranchAndDraftPR(t *testing.T) {
	root := t.TempDir()
	backlogText := "# Backlog\n\n" +
		"## [EPIC-01] Phase 1\n\n" +
		"### [TG-01.1] First group\n" +
		"```yaml\n" +
		"type: feat\n" +
		"version: 1.0.0-alpha.1\n" +
		"```\n\n" +
		"#### [TSK-01.1.1] Task [P: C] [READY]\n" +
		"```yaml\n" +
		"files: [a.go]\n" +
		"done_when: [true]\n" +
		"```\n"

	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlogText), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mock pr.Runner that returns no draft PR
	mockPRRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
			return `[]`, nil
		}
		return "", nil
	}

	// Mock git.Run for branch check - epic branch doesn't exist
	mockGitRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "branch" && args[1] == "-r" {
			return "", nil // branch doesn't exist
		}
		return "", nil
	}

	problems := checkEpicsWithRunners(root, "main", mockGitRun, mockPRRun)
	if len(problems) != 1 || problems[0].Check != "epic" || !strings.Contains(problems[0].Detail, "READY groups") {
		t.Fatalf("expected epic branch and draft PR missing problem, got %+v", problems)
	}
}

func TestCheckEpicsIgnoresEpicWithoutReadyGroups(t *testing.T) {
	root := t.TempDir()
	backlogText := "# Backlog\n\n" +
		"## [EPIC-01] Phase 1\n\n" +
		"### [TG-01.1] First group\n" +
		"```yaml\n" +
		"type: feat\n" +
		"version: 1.0.0-alpha.1\n" +
		"```\n\n" +
		"#### [TSK-01.1.1] Task [P: C] [REFINEMENT]\n" +
		"```yaml\n" +
		"files: [a.go]\n" +
		"done_when: [true]\n" +
		"```\n"

	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlogText), 0o644); err != nil {
		t.Fatal(err)
	}

	mockPRRun := func(_ string, args ...string) (string, error) {
		return `[]`, nil
	}
	mockGitRun := func(_ string, args ...string) (string, error) {
		return "", nil
	}

	problems := checkEpicsWithRunners(root, "main", mockGitRun, mockPRRun)
	if len(problems) != 0 {
		t.Fatalf("expected no problems for epic without READY groups, got %+v", problems)
	}
}

func TestCheckEpicsIgnoresEpicWithExistingBranchOrDraftPR(t *testing.T) {
	root := t.TempDir()
	backlogText := "# Backlog\n\n" +
		"## [EPIC-01] Phase 1\n\n" +
		"### [TG-01.1] First group\n" +
		"```yaml\n" +
		"type: feat\n" +
		"version: 1.0.0-alpha.1\n" +
		"```\n\n" +
		"#### [TSK-01.1.1] Task [P: C] [READY]\n" +
		"```yaml\n" +
		"files: [a.go]\n" +
		"done_when: [true]\n" +
		"```\n"

	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlogText), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mock pr.Runner that returns a draft PR
	mockPRRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
			return `[{"number":1,"isDraft":true}]`, nil
		}
		return "", nil
	}

	mockGitRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "branch" && args[1] == "-r" {
			return "", nil // branch doesn't exist
		}
		return "", nil
	}

	problems := checkEpicsWithRunners(root, "main", mockGitRun, mockPRRun)
	if len(problems) != 0 {
		t.Fatalf("expected no problems when draft PR exists, got %+v", problems)
	}
}

func TestCheckEpicsIgnoresEpicWithExistingBranch(t *testing.T) {
	root := t.TempDir()
	backlogText := "# Backlog\n\n" +
		"## [EPIC-01] Phase 1\n\n" +
		"### [TG-01.1] First group\n" +
		"```yaml\n" +
		"type: feat\n" +
		"version: 1.0.0-alpha.1\n" +
		"```\n\n" +
		"#### [TSK-01.1.1] Task [P: C] [READY]\n" +
		"```yaml\n" +
		"files: [a.go]\n" +
		"done_when: [true]\n" +
		"```\n"

	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlogText), 0o644); err != nil {
		t.Fatal(err)
	}

	mockPRRun := func(_ string, args ...string) (string, error) {
		return `[]`, nil
	}

	mockGitRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "branch" && args[1] == "-r" {
			return "  origin/feat/1.0.0-alpha.1", nil // branch exists
		}
		return "", nil
	}

	problems := checkEpicsWithRunners(root, "main", mockGitRun, mockPRRun)
	if len(problems) != 0 {
		t.Fatalf("expected no problems when epic branch exists, got %+v", problems)
	}
}

func TestCheckGroupPRFlagsWrongBaseWhenEpicBranchExists(t *testing.T) {
	root := t.TempDir()
	backlogText := "# Backlog\n\n" +
		"## [EPIC-01] Phase 1\n\n" +
		"### [TG-01.1] First group\n" +
		"```yaml\n" +
		"type: feat\n" +
		"version: 1.0.0-alpha.1\n" +
		"```\n\n" +
		"#### [TSK-01.1.1] Task [P: C] [READY]\n" +
		"```yaml\n" +
		"files: [a.go]\n" +
		"done_when: [true]\n" +
		"```\n"

	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlogText), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mock pr.Runner that returns a PR with base=main
	mockPRRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
			return `[{"number":1,"headRefName":"feat/TG-01.1-first-group","baseRefName":"main","title":"First group"}]`, nil
		}
		return "", nil
	}

	mockGitRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "branch" && args[1] == "-r" {
			return "  origin/feat/1.0.0-alpha.1", nil // epic branch exists
		}
		return "", nil
	}

	problems := checkEpicsWithRunners(root, "main", mockGitRun, mockPRRun)
	if len(problems) != 1 || !strings.Contains(problems[0].Detail, "decision 0028") {
		t.Fatalf("expected wrong base problem, got %+v", problems)
	}
}

func TestCheckGroupPRIgnoresCorrectBase(t *testing.T) {
	root := t.TempDir()
	backlogText := "# Backlog\n\n" +
		"## [EPIC-01] Phase 1\n\n" +
		"### [TG-01.1] First group\n" +
		"```yaml\n" +
		"type: feat\n" +
		"version: 1.0.0-alpha.1\n" +
		"```\n\n" +
		"#### [TSK-01.1.1] Task [P: C] [READY]\n" +
		"```yaml\n" +
		"files: [a.go]\n" +
		"done_when: [true]\n" +
		"```\n"

	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlogText), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mock pr.Runner that returns a PR with correct base (epic branch)
	mockPRRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
			return `[{"number":1,"headRefName":"feat/TG-01.1-first-group","baseRefName":"feat/1.0.0-alpha.1","title":"First group"}]`, nil
		}
		return "", nil
	}

	mockGitRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "branch" && args[1] == "-r" {
			return "  origin/feat/1.0.0-alpha.1", nil
		}
		return "", nil
	}

	problems := checkEpicsWithRunners(root, "main", mockGitRun, mockPRRun)
	if len(problems) != 0 {
		t.Fatalf("expected no problems when PR targets correct base, got %+v", problems)
	}
}

func TestCheckGroupPRIgnoresNonGroupBranches(t *testing.T) {
	root := t.TempDir()
	backlogText := "# Backlog\n\n" +
		"## [EPIC-01] Phase 1\n\n" +
		"### [TG-01.1] First group\n" +
		"```yaml\n" +
		"type: feat\n" +
		"version: 1.0.0-alpha.1\n" +
		"```\n\n" +
		"#### [TSK-01.1.1] Task [P: C] [READY]\n" +
		"```yaml\n" +
		"files: [a.go]\n" +
		"done_when: [true]\n" +
		"```\n"

	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlogText), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mock pr.Runner that distinguishes between the draft PR check and the group PR check
	mockPRRun := func(_ string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
			// Check if this is a draft PR check (has --head) or a group PR check
			hasHeadFlag := false
			for i := range args {
				if args[i] == "--head" {
					hasHeadFlag = true
					break
				}
			}

			if hasHeadFlag {
				// For the draft PR check for the epic, return a draft PR
				return `[{"number":2,"isDraft":true}]`, nil
			} else {
				// For the group PR check, return a PR with unknown branch
				return `[{"number":1,"headRefName":"fix/something-random","baseRefName":"main","title":"Random fix"}]`, nil
			}
		}
		return "", nil
	}

	mockGitRun := func(_ string, args ...string) (string, error) {
		return "", nil
	}

	problems := checkEpicsWithRunners(root, "main", mockGitRun, mockPRRun)
	if len(problems) != 0 {
		t.Fatalf("expected no problems for non-group branches, got %+v", problems)
	}
}
