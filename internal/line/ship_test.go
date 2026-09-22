package line

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const shipBacklog = "### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-09.1.1] Do it [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - true\n```\n"

// shipRepo builds a group worktree with a remote origin, so ShipGroup can commit and push.
func shipRepo(t *testing.T) (root, group string) {
	t.Helper()
	root = t.TempDir()
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	group = filepath.Join(root, "group")
	if err := os.MkdirAll(group, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(shipBacklog), 0o644); err != nil {
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
