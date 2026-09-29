package line

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/git"
	"komodo/internal/pr"
)

// rephaseBacklog holds one epic with two groups sharing its version.
const rephaseBacklog = "## [EPIC-05] Phase 1: the conductor drives\n" +
	"*Goal: one group runs through the conductor within 60 minutes. Ships as `2.0.0`.*\n\n" +
	"### [TG-05.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-05.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
	"### [TG-05.2] Another group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-05.2.1] Two [P: C] [READY]\n```yaml\nfiles: [a/two.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

// rephaseGroupFile is the first group's own docs/backlog file, carrying the epic's version too.
const rephaseGroupFile = "## [TG-05.1] A group [P: C] [READY]\n\n```yaml\ntype: feat\nversion: 2.0.0\nepic: EPIC-05\n```\n"

// rephaseRepo builds a remoted repo with the epic branch already pushed and one group file.
func rephaseRepo(t *testing.T) string {
	t.Helper()
	root := epicRepo(t, rephaseBacklog)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	if err := os.MkdirAll(filepath.Join(root, "docs", "backlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "backlog", "TG-05.1-a-group.md"), []byte(rephaseGroupFile), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRephaseRewritesVersionsPushesAndRetargetsPulls(t *testing.T) {
	root := rephaseRepo(t)
	var calls []string
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		switch {
		case strings.HasPrefix(joined, "pr list"):
			return `[{"number":15,"url":"https://example.com/pull/15"}]`, nil
		case strings.HasPrefix(joined, "pr edit"):
			return "", nil
		}
		return "", nil
	}}

	result, err := Rephase(root, "EPIC-05", "3.0.0", client)
	if err != nil {
		t.Fatal(err)
	}
	if result.OldVersion != "2.0.0" || result.NewVersion != "3.0.0" {
		t.Fatalf("versions = %+v", result)
	}
	if result.OldBranch != "feat/2.0.0" || result.NewBranch != "feat/3.0.0" {
		t.Fatalf("branches = %+v", result)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("groups = %v, want both TG-05.1 and TG-05.2", result.Groups)
	}
	if len(result.GroupFiles) != 1 {
		t.Fatalf("group files = %v, want the one docs/backlog file rewritten", result.GroupFiles)
	}
	if len(result.Retargeted) != 1 || result.Retargeted[0] != "https://example.com/pull/15" {
		t.Fatalf("retargeted = %v", result.Retargeted)
	}

	data, err := os.ReadFile(filepath.Join(root, "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "version: 2.0.0") {
		t.Fatal("BACKLOG.md still carries the old version")
	}
	if strings.Count(text, "version: 3.0.0") != 2 {
		t.Fatalf("BACKLOG.md = %s, want both groups moved to 3.0.0", text)
	}
	if !strings.Contains(text, "Ships as `3.0.0`") {
		t.Fatalf("BACKLOG.md = %s, want the epic's goal line moved too", text)
	}

	groupFile, err := os.ReadFile(filepath.Join(root, "docs", "backlog", "TG-05.1-a-group.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(groupFile), "version: 3.0.0") {
		t.Fatalf("group file = %s, want 3.0.0", groupFile)
	}

	if _, err := git.Run(root, "ls-remote", "--exit-code", "--heads", "origin", "refs/heads/feat/3.0.0"); err != nil {
		t.Fatalf("feat/3.0.0 never reached origin: %v", err)
	}

	var sawList, sawEdit bool
	for _, call := range calls {
		if strings.Contains(call, "pr list") && strings.Contains(call, "--base feat/2.0.0") {
			sawList = true
		}
		if strings.Contains(call, "pr edit 15") && strings.Contains(call, "--base feat/3.0.0") {
			sawEdit = true
		}
	}
	if !sawList || !sawEdit {
		t.Fatalf("calls = %v, want a pr list on the old branch and a pr edit onto the new one", calls)
	}
}

func TestRephaseSkipsPullRequestWorkWithNoClient(t *testing.T) {
	root := rephaseRepo(t)
	result, err := Rephase(root, "EPIC-05", "3.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Retargeted) != 0 {
		t.Fatalf("retargeted = %v, want none with no client", result.Retargeted)
	}
}

func TestRephaseRefusesTheSameVersion(t *testing.T) {
	root := rephaseRepo(t)
	if _, err := Rephase(root, "EPIC-05", "2.0.0", nil); err == nil {
		t.Fatal("want an error rephasing an epic to the version it already ships")
	}
}

func TestRephaseRefusesAnUnknownEpic(t *testing.T) {
	root := rephaseRepo(t)
	if _, err := Rephase(root, "EPIC-99", "3.0.0", nil); err == nil {
		t.Fatal("want an error for an epic that does not exist")
	}
}

func TestDeleteBranchRefusesACriticalRef(t *testing.T) {
	root := rephaseRepo(t)
	if err := DeleteBranch(root, "main"); err == nil {
		t.Fatal("want a refusal deleting a critical ref")
	}
}

func TestDeleteBranchRemovesTheOldBranch(t *testing.T) {
	root := rephaseRepo(t)
	if _, err := Rephase(root, "EPIC-05", "3.0.0", nil); err != nil {
		t.Fatal(err)
	}
	if err := DeleteBranch(root, "feat/2.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "ls-remote", "--exit-code", "--heads", "origin", "refs/heads/feat/2.0.0"); err == nil {
		t.Fatal("feat/2.0.0 should be gone from origin")
	}
}
