package line

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
)

const shipBacklog = "### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-09.1.1] Do it [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - true\n```\n"

// shipRepo builds a group worktree carrying its own backlog and a remote origin, so ShipGroup can push.
func shipRepo(t *testing.T) (root, group string) {
	t.Helper()
	root = t.TempDir()
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	group = filepath.Join(root, "group")
	if err := os.MkdirAll(group, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "init")
	runGit(t, group, "config", "user.email", "a@example.com")
	runGit(t, group, "config", "user.name", "a")
	runGit(t, group, "remote", "add", "origin", bare)
	runGit(t, group, "checkout", "-b", "feat/a-group")
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "BACKLOG.md"), []byte(shipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "seed")
	return root, group
}

// runGit runs one git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestShipGroupRunsTheAfterPublishCommand(t *testing.T) {
	root, group := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(group, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `{"after_publish":"touch published.txt"}`
	if err := os.WriteFile(filepath.Join(group, StateDir, "commands.json"), []byte(commands), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Published == nil || !result.Published.OK() {
		t.Fatalf("published = %+v", result.Published)
	}
	if _, err := os.Stat(filepath.Join(group, "published.txt")); err != nil {
		t.Fatalf("after_publish did not run: %v", err)
	}
}

func TestShipGroupWithNoAfterPublishLeavesPublishedUnset(t *testing.T) {
	root, _ := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Published != nil {
		t.Fatalf("published = %+v, want nil", result.Published)
	}
}

const flipBacklog = "### [TG-11.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-11.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

func TestShipFlipsTheStatusOnTheBranchItPushes(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", flipBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(flipBacklog), 0o644); err != nil {
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

func TestAFailedGateRecordsShipAsFailedNotDone(t *testing.T) {
	root, _ := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	state := RunState{Run: "TG-09.1-1", Group: "TG-09.1", Base: "main", Branch: "feat/a-group"}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	if _, err := ShipGroup(root, plan, "", nil); err == nil || !strings.Contains(err.Error(), "gate") {
		t.Fatalf("err = %v; the group worktree has no cmd/komodo, so the gate must fail", err)
	}
	entries, err := Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Group == "TG-09.1" && entry.Outcome == "done" {
			t.Fatal("a failed gate must not stamp the ship entry as done")
		}
	}
	if shipped(root, plan, backlog.Backlog{}) {
		t.Fatal("a failed gate must not release the line to the next group")
	}
}

func TestShipCommitsTheStatusItWrote(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", flipBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(flipBacklog), 0o644); err != nil {
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
