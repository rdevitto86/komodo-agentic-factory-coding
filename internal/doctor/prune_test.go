package doctor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/git"
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

// refExists reports whether ref resolves in root.
func refExists(root, ref string) bool {
	return exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", ref).Run() == nil
}

func TestPruneRemovesALandedDetachedWorktreeAndItsTipButNotAFreshCut(t *testing.T) {
	root, run := pruneRepo(t, "# Backlog\n")
	bare := filepath.Join(t.TempDir(), "origin.git")
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")

	// cut builds a clean detached worktree off main tracking branch, its tip at refs/komodo/<branch>.
	cut := func(group, branch string) string {
		worktree := filepath.Join(root, ".komodo", "wt", group)
		if err := line.AddDetached(root, branch, "main", worktree); err != nil {
			t.Fatal(err)
		}
		write(t, worktree, group+".txt", "done\n")
		commitAll(t, worktree, "ship "+group)
		if err := line.Advance(root, branch, worktree, git.Or(root, "rev-parse", line.TipRef(branch))); err != nil {
			t.Fatal(err)
		}
		return worktree
	}
	landedTree := cut("TG-01.1", "feat/g")
	run(root, "push", "-q", "origin", line.TipRef("feat/g")+":refs/heads/feat/g")
	run(root, "push", "-q", "origin", line.TipRef("feat/g")+":refs/heads/main")
	run(root, "fetch", "-q", "origin")
	run(root, "branch", "feat/person", "main")
	fresh := cut("TG-01.2", "feat/h")
	pushedOnly := cut("TG-01.3", "feat/i")
	run(root, "push", "-q", "origin", line.TipRef("feat/i")+":refs/heads/feat/i")
	run(root, "fetch", "-q", "origin")

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if exists(landedTree) || refExists(root, line.TipRef("feat/g")) {
		t.Fatalf("the landed worktree or its tip survived; done = %v", got)
	}
	if !exists(fresh) || !refExists(root, line.TipRef("feat/h")) {
		t.Fatalf("a fresh cut, never pushed, counted as landed; done = %v", got)
	}
	if !exists(pushedOnly) || !refExists(root, line.TipRef("feat/i")) {
		t.Fatalf("a pushed branch not in main counted as landed; done = %v", got)
	}
	if !refExists(root, "refs/heads/feat/person") {
		t.Fatalf("a person's branch was deleted; done = %v", got)
	}
}

// TestPruneListsAMergedBranchAndNeverDeletesIt proves prune only names a merged local branch, confirmed or not.
func TestPruneListsAMergedBranchAndNeverDeletesIt(t *testing.T) {
	root, run := pruneRepo(t, "# Backlog\n")
	run(root, "branch", "feat/merged")
	run(root, "checkout", "-q", "feat/merged")
	write(t, root, "merged.txt", "done\n")
	commitAll(t, root, "merged work")
	run(root, "checkout", "-q", "main")
	run(root, "merge", "-q", "--ff-only", "feat/merged")

	for _, confirm := range []bool{false, true} {
		got, err := Prune(root, "main", confirm)
		if err != nil {
			t.Fatal(err)
		}
		if !refExists(root, "refs/heads/feat/merged") {
			t.Fatalf("feat/merged was deleted (confirm=%v); done = %v", confirm, got)
		}
		found := false
		for _, item := range got {
			found = found || (strings.Contains(item, "feat/merged") && strings.Contains(item, "git branch -d"))
		}
		if !found {
			t.Fatalf("done = %v, want a line naming feat/merged and git branch -d", got)
		}
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

// noForge keeps every prune test off the real gh unless it names its own forge.
func init() {
	mergedOnForge = func(string, string) bool { return false }
}

// squashRepo is a root on main remoted at a bare origin, with a clean detached worktree cut for branch
// that holds one commit the line pushed through PushFromWorktree.
func squashRepo(t *testing.T, branch string) (root, bare, worktree string, run func(dir string, args ...string)) {
	t.Helper()
	root, run = pruneRepo(t, "# Backlog\n")
	bare = filepath.Join(t.TempDir(), "origin.git")
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")
	worktree = filepath.Join(root, ".komodo", "wt", "TG-01.1")
	if err := line.AddDetached(root, branch, "main", worktree); err != nil {
		t.Fatal(err)
	}
	write(t, worktree, "work.txt", "done\n")
	commitAll(t, worktree, "work")
	if err := line.PushFromWorktree(root, worktree, branch); err != nil {
		t.Fatal(err)
	}
	return root, bare, worktree, run
}

// TestPruneSweepsASquashMergedBranchTheForgeReportsMerged proves a merged pull request headed by the
// branch lands it, though its commits are no ancestor of the base.
func TestPruneSweepsASquashMergedBranchTheForgeReportsMerged(t *testing.T) {
	root, _, worktree, _ := squashRepo(t, "feat/squash")
	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	if !exists(worktree) || !refExists(root, line.TipRef("feat/squash")) {
		t.Fatal("an unmerged pushed branch was swept with the forge silent")
	}
	mergedOnForge = func(_, branch string) bool { return branch == "feat/squash" }
	t.Cleanup(func() { mergedOnForge = func(string, string) bool { return false } })
	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	if exists(worktree) || refExists(root, line.TipRef("feat/squash")) {
		t.Fatal("a branch the forge reports merged kept its worktree or tip")
	}
}

// TestPruneSweepsABranchOriginDroppedAfterTheLinesOwnPush proves the worktree's pushed record, with its
// branch gone from origin, lands it; a branch origin still holds, or one with no record, stays.
func TestPruneSweepsABranchOriginDroppedAfterTheLinesOwnPush(t *testing.T) {
	root, bare, worktree, run := squashRepo(t, "feat/dropped")
	if got := git.Or(worktree, "config", "--worktree", "--get", "komodo.pushed"); got == "" {
		t.Fatal("PushFromWorktree recorded no komodo.pushed sha")
	}
	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	if !exists(worktree) {
		t.Fatal("a branch origin still holds was swept")
	}
	run(root, "-C", bare, "branch", "-D", "feat/dropped")
	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	if exists(worktree) || refExists(root, line.TipRef("feat/dropped")) {
		t.Fatal("a pushed branch origin dropped kept its worktree or tip")
	}
}

// TestPruneKeepsADroppedBranchWithNoPushedRecordOrNewWork proves no forge and no record keeps a worktree,
// and a record is void once the tip moved past it.
func TestPruneKeepsADroppedBranchWithNoPushedRecordOrNewWork(t *testing.T) {
	root, bare, worktree, run := squashRepo(t, "feat/kept")
	run(root, "-C", bare, "branch", "-D", "feat/kept")
	run(worktree, "config", "--worktree", "--unset", "komodo.pushed")
	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	if !exists(worktree) {
		t.Fatal("a branch with no forge and no record was swept")
	}
	run(worktree, "config", "--worktree", "komodo.pushed", git.Or(worktree, "rev-parse", "HEAD"))
	write(t, worktree, "more.txt", "more\n")
	commitAll(t, worktree, "more")
	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	if !exists(worktree) {
		t.Fatal("a worktree with unpushed work past its record was swept")
	}
}
