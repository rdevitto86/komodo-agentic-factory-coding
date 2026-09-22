package line

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shipBacklog = "### [TG-11.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-11.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

func TestShipFlipsTheStatusOnTheBranchItPushes(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", shipBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(shipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	if _, err := ShipGroup(root, plan, "body", nil); err == nil || !strings.Contains(err.Error(), "push") {
		t.Fatalf("err = %v; the fixture has no remote, so ship must fail at the push and not before", err)
	}
	shipped, err := os.ReadFile(filepath.Join(worktree, "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shipped), "[TSK-11.1.1] One [P: C] [DONE]") {
		t.Fatalf("the branch being pushed still says READY:\n%s", shipped)
	}
	stale, err := os.ReadFile(filepath.Join(root, "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stale), "[DONE]") {
		t.Fatal("ship must write the worktree it commits, not the root it was invoked from")
	}
}

func TestShipCommitsTheStatusItWrote(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", shipBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(shipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	_, _ = ShipGroup(root, plan, "body", nil)
	status, err := git(worktree, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("status = %q; the status flip and the changelog must land in the commit, not after it", status)
	}
}
