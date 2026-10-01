package doctor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/line"
)

// openBacklog holds one ready group, so a run recorded for it stays open.
const openBacklog = "### [TG-01.1] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-01.1.1] Do it [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when:\n  - true\n```\n"

// pruneRepo builds a root on main with its group's own file committed, and returns it with a git
// runner that fails the test.
func pruneRepo(t *testing.T, backlog string) (string, func(dir string, args ...string)) {
	t.Helper()
	root := gitRepo(t)
	backlogtest.SeedText(t, root, backlog)
	write(t, root, ".gitignore", "/.komodo/\n")
	commitAll(t, root, "init")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return root, run
}

func TestPruneSettlesASquashMergedGroupWhoseBranchIsGone(t *testing.T) {
	root, run := pruneRepo(t, "# Backlog\n")
	bare := filepath.Join(t.TempDir(), "origin.git")
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")

	// cut builds a clean worktree off main holding one commit of its own.
	cut := func(group, branch string) string {
		worktree := filepath.Join(root, ".komodo", "wt", group)
		run(root, "worktree", "add", "-q", "-b", branch, worktree, "main")
		write(t, worktree, group+".txt", "done\n")
		commitAll(t, worktree, "ship "+group)
		return worktree
	}
	squashed := cut("TG-01.1", "feat/g")
	run(squashed, "push", "-q", "origin", "feat/g")
	run(squashed, "branch", "-q", "--set-upstream-to=origin/feat/g", "feat/g")
	run(root, "push", "-q", "origin", "--delete", "feat/g")
	unpushed := cut("TG-01.2", "feat/h")

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if exists(squashed) {
		t.Fatalf("the squash-merged group's worktree survived though origin deleted its branch; done = %v", got)
	}
	if out, _ := exec.Command("git", "-C", root, "branch", "--list", "feat/g").Output(); strings.TrimSpace(string(out)) != "" {
		t.Fatalf("feat/g survived; done = %v", got)
	}
	if !exists(unpushed) {
		t.Fatalf("a branch never pushed was taken for one origin deleted; done = %v", got)
	}
}

// TestPruneDefaultsToADryRunAndNeverDeletesABranch proves an unconfirmed prune only lists a merged
// branch it would delete, and a confirmed prune then deletes it.
func TestPruneDefaultsToADryRunAndNeverDeletesABranch(t *testing.T) {
	root, run := pruneRepo(t, "# Backlog\n")
	run(root, "branch", "feat/merged")
	run(root, "checkout", "-q", "feat/merged")
	write(t, root, "merged.txt", "done\n")
	commitAll(t, root, "merged work")
	run(root, "checkout", "-q", "main")
	run(root, "merge", "-q", "--ff-only", "feat/merged")

	dryRun, err := Prune(root, "main", false)
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := exec.Command("git", "-C", root, "branch", "--list", "feat/merged").Output(); strings.TrimSpace(string(out)) == "" {
		t.Fatalf("feat/merged was deleted though prune ran unconfirmed; done = %v", dryRun)
	}
	found := false
	for _, item := range dryRun {
		if strings.Contains(item, "feat/merged") && strings.Contains(item, "--confirm") {
			found = true
		}
	}
	if !found {
		t.Fatalf("done = %v, want a would-delete line naming feat/merged and --confirm", dryRun)
	}

	confirmed, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := exec.Command("git", "-C", root, "branch", "--list", "feat/merged").Output(); strings.TrimSpace(string(out)) != "" {
		t.Fatalf("feat/merged survived a confirmed prune; done = %v", confirmed)
	}
}

func TestPruneKeepsOnlyTheNewestRunFolders(t *testing.T) {
	root, _ := pruneRepo(t, openBacklog)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	groups := []string{"TG-01.1"}
	for index := 2; index <= keptRuns+2; index++ {
		groups = append(groups, fmt.Sprintf("TG-02.%d", index))
	}
	for index, group := range groups {
		state := line.RunState{Run: "r", Group: group, Base: "main", Started: start.Add(time.Duration(index) * time.Hour)}
		if err := line.SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(line.RunDir(root, "TG-01.1")) {
		t.Fatalf("the oldest run's folder was removed though its group is still open; done = %v", got)
	}
	if exists(line.RunDir(root, groups[1])) {
		t.Fatalf("%s's folder survived past the newest %d; done = %v", groups[1], keptRuns, got)
	}
	for _, group := range groups[2:] {
		if !exists(line.RunDir(root, group)) {
			t.Fatalf("%s's folder, one of the newest %d, was removed; done = %v", group, keptRuns, got)
		}
	}
}
