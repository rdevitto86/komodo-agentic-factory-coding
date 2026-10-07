package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/git"
	"komodo/internal/harness"
	"komodo/internal/lease"
	"komodo/internal/proc"
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

// TestOriginLacksKillsAHungLsRemoteAndItsChildren proves a ls-remote past toolTimeout is killed,
// process group included, so a hung origin never blocks a sweep.
func TestOriginLacksKillsAHungLsRemoteAndItsChildren(t *testing.T) {
	root := gitRepo(t)
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	fakes := t.TempDir()
	wrapper := "#!/bin/sh\nif [ \"$1\" = ls-remote ]; then sleep 30 & sleep 30; fi\nexec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(fakes, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakes+string(os.PathListSeparator)+os.Getenv("PATH"))
	saved := toolTimeout
	toolTimeout = 200 * time.Millisecond
	t.Cleanup(func() { toolTimeout = saved })
	started := time.Now()
	originLacks(root, "feat/x")
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
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
		if err := harness.AddDetached(root, branch, "main", worktree); err != nil {
			t.Fatal(err)
		}
		write(t, worktree, group+".txt", "done\n")
		commitAll(t, worktree, "ship "+group)
		if err := harness.Advance(root, branch, worktree, git.Or(root, "rev-parse", harness.TipRef(branch))); err != nil {
			t.Fatal(err)
		}
		return worktree
	}
	landedTree := cut("TG-01.1", "feat/g")
	run(root, "push", "-q", "origin", harness.TipRef("feat/g")+":refs/heads/feat/g")
	run(root, "push", "-q", "origin", harness.TipRef("feat/g")+":refs/heads/main")
	run(root, "fetch", "-q", "origin")
	run(root, "branch", "feat/person", "main")
	fresh := cut("TG-01.2", "feat/h")
	pushedOnly := cut("TG-01.3", "feat/i")
	run(root, "push", "-q", "origin", harness.TipRef("feat/i")+":refs/heads/feat/i")
	run(root, "fetch", "-q", "origin")

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if exists(landedTree) || refExists(root, harness.TipRef("feat/g")) {
		t.Fatalf("the landed worktree or its tip survived; done = %v", got)
	}
	if !exists(fresh) || !refExists(root, harness.TipRef("feat/h")) {
		t.Fatalf("a fresh cut, never pushed, counted as landed; done = %v", got)
	}
	if !exists(pushedOnly) || !refExists(root, harness.TipRef("feat/i")) {
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
		state := harness.RunState{Run: "r", Group: group, Base: "main", Started: start.Add(time.Duration(index) * time.Hour)}
		if err := harness.SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(harness.RunDir(root, "TG-01.1")) {
		t.Fatalf("the oldest run's folder was removed though its group is still open; done = %v", got)
	}
	if exists(harness.RunDir(root, groups[1])) {
		t.Fatalf("%s's folder survived past the newest %d; done = %v", groups[1], keptRuns, got)
	}
	for _, group := range groups[2:] {
		if !exists(harness.RunDir(root, group)) {
			t.Fatalf("%s's folder, one of the newest %d, was removed; done = %v", group, keptRuns, got)
		}
	}
}

func TestPruneDryRunLeavesEveryRunFolderInPlace(t *testing.T) {
	root, _ := pruneRepo(t, openBacklog)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var groups []string
	for index := 1; index <= keptRuns+2; index++ {
		group := fmt.Sprintf("TG-02.%d", index)
		groups = append(groups, group)
		state := harness.RunState{Run: "r", Group: group, Base: "main", Started: start.Add(time.Duration(index) * time.Hour)}
		if err := harness.SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Prune(root, "main", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if !exists(harness.RunDir(root, group)) {
			t.Fatalf("a dry run removed %s's folder; done = %v", group, got)
		}
	}
	if !strings.Contains(strings.Join(got, "\n"), "would remove run folder") {
		t.Fatalf("done = %v, want the old run folders listed", got)
	}
}

// TestPruneNeverRemovesOutsideARunsOwnGroupDirectory proves a run.json whose group escapes its own
// directory, such as "..", is skipped rather than handed to os.RemoveAll.
func TestPruneNeverRemovesOutsideARunsOwnGroupDirectory(t *testing.T) {
	root, _ := pruneRepo(t, openBacklog)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	evil := filepath.Join(root, ".komodo", "runs", "evil")
	if err := os.MkdirAll(evil, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"run":"r","group":"..","base":"main","started":%q}`, start.Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(evil, "run.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= keptRuns; index++ {
		group := fmt.Sprintf("TG-02.%d", index)
		state := harness.RunState{Run: "r", Group: group, Base: "main", Started: start.Add(time.Duration(index) * time.Hour)}
		if err := harness.SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}
	sentinel := filepath.Join(root, ".komodo", "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("still here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("a run.json naming %q deleted past its own directory; done = %v", "..", got)
	}
	if _, err := os.Stat(evil); err != nil {
		t.Fatalf("the evil run's own folder should stay, untouched; done = %v", got)
	}
}

// noForge keeps every prune test off the real gh unless it names its own forge.
func init() {
	mergedOnForge = func(string, string) bool { return false }
}

// squashRepo is a root on main remoted at a bare origin, with a clean detached worktree cut for branch
// that holds one commit the harness pushed through PushFromWorktree.
func squashRepo(t *testing.T, branch string) (root, bare, worktree string, run func(dir string, args ...string)) {
	t.Helper()
	root, run = pruneRepo(t, "# Backlog\n")
	bare = filepath.Join(t.TempDir(), "origin.git")
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")
	worktree = filepath.Join(root, ".komodo", "wt", "TG-01.1")
	if err := harness.AddDetached(root, branch, "main", worktree); err != nil {
		t.Fatal(err)
	}
	write(t, worktree, "work.txt", "done\n")
	commitAll(t, worktree, "work")
	if err := harness.PushFromWorktree(root, worktree, branch); err != nil {
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
	if !exists(worktree) || !refExists(root, harness.TipRef("feat/squash")) {
		t.Fatal("an unmerged pushed branch was swept with the forge silent")
	}
	mergedOnForge = func(_, branch string) bool { return branch == "feat/squash" }
	t.Cleanup(func() { mergedOnForge = func(string, string) bool { return false } })
	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	if exists(worktree) || refExists(root, harness.TipRef("feat/squash")) {
		t.Fatal("a branch the forge reports merged kept its worktree or tip")
	}
}

// TestPruneSweepsABranchOriginDroppedAfterTheHarnessOwnPush proves the worktree's pushed record, with its
// branch gone from origin, lands it; a branch origin still holds, or one with no record, stays.
func TestPruneSweepsABranchOriginDroppedAfterTheHarnessOwnPush(t *testing.T) {
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
	if exists(worktree) || refExists(root, harness.TipRef("feat/dropped")) {
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

// TestPruneSweepsAnIdleWorktreeWhoseWorkIsOnOrigin proves an idle worktree with every commit on origin goes,
// freeing its pinned branch, while a recent, unpushed, dirty, leased, open-lane or past-its-tip one stays.
func TestPruneSweepsAnIdleWorktreeWhoseWorkIsOnOrigin(t *testing.T) {
	root, run := pruneRepo(t, openBacklog)
	bare := filepath.Join(t.TempDir(), "origin.git")
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")
	wt := func(name string) string { return filepath.Join(root, ".komodo", "wt", name) }
	// cut builds a detached worktree tracking branch with one commit, pushing it to origin when asked.
	cut := func(name, branch string, push bool) string {
		path := wt(name)
		if err := harness.AddDetached(root, branch, "main", path); err != nil {
			t.Fatal(err)
		}
		write(t, path, name+".txt", "work\n")
		commitAll(t, path, "work "+name)
		if err := harness.Advance(root, branch, path, git.Or(root, "rev-parse", harness.TipRef(branch))); err != nil {
			t.Fatal(err)
		}
		if push {
			run(root, "push", "-q", "origin", git.Or(path, "rev-parse", "HEAD")+":refs/heads/"+branch)
		}
		return path
	}
	group := cut("TG-09.1", "feat/done", true)
	run(root, "fetch", "-q", "origin")
	pinned := wt("TSK-09.1.1")
	run(root, "worktree", "add", "-q", "-b", "task/tsk-09.1.1", pinned, "origin/feat/done")
	unpushed := cut("TG-09.2", "feat/local", false)
	dirty := cut("TG-09.3", "feat/dirty", true)
	write(t, dirty, "draft.txt", "unsaved\n")
	leased := cut("TG-09.4", "feat/leased", true)
	ahead := wt("TG-09.5")
	if err := harness.AddDetached(root, "feat/ahead", "main", ahead); err != nil {
		t.Fatal(err)
	}
	run(root, "push", "-q", "origin", "main:refs/heads/feat/ahead")
	write(t, ahead, "ahead.txt", "past the tip\n")
	commitAll(t, ahead, "work past the tip")
	lane := wt("TSK-01.1.1")
	if err := harness.AddDetached(root, harness.TaskBranch("TSK-01.1.1"), "origin/feat/done", lane); err != nil {
		t.Fatal(err)
	}
	state := harness.RunState{Run: "r", Group: "TG-01.1", Base: "main", Branch: "feat/g", Waves: [][]string{{"TSK-01.1.1"}}}
	if err := harness.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	kept := []string{unpushed, dirty, leased, lane, ahead}

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append([]string{group, pinned}, kept...) {
		if !exists(path) {
			t.Fatalf("%s was swept before it sat idle; done = %v", path, got)
		}
	}

	later := time.Now().Add(idleFor + time.Hour)
	now = func() time.Time { return later }
	t.Cleanup(func() { now = time.Now })
	self, ok := proc.Of(os.Getpid())
	if !ok {
		t.Skip("no process table on this platform")
	}
	if err := lease.Take(root, "TG-09.4", "feat/leased", self, later); err != nil {
		t.Fatal(err)
	}
	got, err = Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if exists(group) || exists(pinned) {
		t.Fatalf("an idle worktree with its work on origin survived; done = %v", got)
	}
	for _, path := range kept {
		if !exists(path) {
			t.Fatalf("%s was swept; done = %v", path, got)
		}
	}
	if !refExists(root, "refs/heads/task/tsk-09.1.1") {
		t.Fatal("prune deleted a branch under refs/heads/")
	}
	run(root, "branch", "-D", "task/tsk-09.1.1")
}

// TestPruneKeepsNewWorkOnABranchTheForgeReportsMerged proves a commit past the harness's push survives a merged pull request.
func TestPruneKeepsNewWorkOnABranchTheForgeReportsMerged(t *testing.T) {
	root, _, worktree, _ := squashRepo(t, "feat/reused")
	old := git.Or(root, "rev-parse", harness.TipRef("feat/reused"))
	write(t, worktree, "next.txt", "after the merge\n")
	commitAll(t, worktree, "work after the merge")
	if err := harness.Advance(root, "feat/reused", worktree, old); err != nil {
		t.Fatal(err)
	}
	mergedOnForge = func(_, branch string) bool { return branch == "feat/reused" }
	t.Cleanup(func() { mergedOnForge = func(string, string) bool { return false } })
	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	if !exists(worktree) || !refExists(root, harness.TipRef("feat/reused")) {
		t.Fatal("unpushed work on a forge-merged branch was swept")
	}
}

func TestGroupOfNamesTheGroupEveryStateFileBelongsTo(t *testing.T) {
	cases := map[string]string{
		"TSK-03.7.11.json":          "TG-03.7",
		"TSK-03.7.11.md":            "TG-03.7",
		"TG-03.10-review.json":      "TG-03.10",
		"TG-07.2-fix.md":            "TG-07.2",
		"TG-03.10-1790218115.jsonl": "TG-03.10",
		"TG-03.6.jsonl":             "TG-03.6",
		"harness.jsonl":             "",
		"notes.md":                  "",
	}
	for name, want := range cases {
		if got := groupOf(name); got != want {
			t.Errorf("groupOf(%s) = %q, want %q", name, got, want)
		}
	}
}

func TestPruneSpentStateKeepsALiveGroupsFilesAndDropsTheRest(t *testing.T) {
	root, _ := pruneRepo(t, openBacklog)
	state := filepath.Join(root, ".komodo")
	live := []string{"briefs/TSK-01.1.1.md", "results/TG-01.1-review.json", "harness.TG-01.1-1790000000.jsonl", "harness.jsonl"}
	spent := []string{"briefs/TSK-03.7.11.md", "results/TSK-03.7.11.json", "harness.TG-03.10-1790218115.jsonl"}
	for _, rel := range append(append([]string{}, live...), spent...) {
		write(t, state, rel, "x\n")
	}
	if dry := pruneSpentState(root, nil, false); len(dry) != len(spent) || !strings.HasPrefix(dry[0], "would remove") {
		t.Fatalf("dry run = %q, want %d would-remove lines", dry, len(spent))
	}
	done := pruneSpentState(root, nil, true)
	if len(done) != len(spent) {
		t.Fatalf("done = %q, want the %d spent files removed", done, len(spent))
	}
	for _, rel := range spent {
		if _, err := os.Stat(filepath.Join(state, rel)); !os.IsNotExist(err) {
			t.Fatalf("%s survived (%v)", rel, err)
		}
	}
	for _, rel := range live {
		if _, err := os.Stat(filepath.Join(state, rel)); err != nil {
			t.Fatalf("%s was removed: %v; a live group's state stays", rel, err)
		}
	}
}

func TestPruneStashesArchivesAWeekOldStashThenDropsIt(t *testing.T) {
	root, run := pruneRepo(t, openBacklog)
	write(t, root, "wip.txt", "old work\n")
	stash := exec.Command("git", "stash", "push", "-u", "-q", "-m", "old work")
	stash.Dir = root
	stash.Env = append(os.Environ(), "GIT_COMMITTER_DATE=2026-09-01T00:00:00Z", "GIT_AUTHOR_DATE=2026-09-01T00:00:00Z")
	if out, err := stash.CombinedOutput(); err != nil {
		t.Fatalf("stash: %v: %s", err, out)
	}
	write(t, root, "fresh.txt", "new work\n")
	run(root, "stash", "push", "-u", "-q", "-m", "fresh work")
	at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	done := pruneStashes(root, at, true)
	if len(done) != 1 || !strings.Contains(done[0], "archived") {
		t.Fatalf("done = %q, want the old stash archived", done)
	}
	out, err := exec.Command("git", "-C", root, "stash", "list").Output()
	if left := strings.TrimSpace(string(out)); err != nil || strings.Count(left, "\n") != 0 || !strings.Contains(left, "fresh work") {
		t.Fatalf("stash list = %q, want only the fresh stash", left)
	}
	patches, _ := filepath.Glob(filepath.Join(root, ".komodo", "stash-archive", "2026-09-01-*.patch"))
	if len(patches) != 1 {
		t.Fatalf("patches = %v, want one archive", patches)
	}
	if body, _ := os.ReadFile(patches[0]); !strings.Contains(string(body), "old work") {
		t.Fatalf("patch = %q, want the stash's own change", body)
	}
}

func TestPruneRemovesTheDeadBranchClaims(t *testing.T) {
	root := gitRepo(t)
	claims := filepath.Join(root, ".git", "komodo-claims")
	if err := os.MkdirAll(claims, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claims, "HEAD.json"), []byte(`{"session":"s","branch":"HEAD"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if dry := pruneClaims(root, false); len(dry) != 1 || !strings.HasPrefix(dry[0], "would remove") {
		t.Fatalf("dry run = %q, want one would-remove line", dry)
	}
	if _, err := os.Stat(claims); err != nil {
		t.Fatalf("a dry run removed the claims: %v", err)
	}
	done := pruneClaims(root, true)
	if _, err := os.Stat(claims); !os.IsNotExist(err) {
		t.Fatalf("claims still present (%v); nothing reads them since the guard dropped claims", err)
	}
	if len(done) != 1 || !strings.Contains(done[0], "removed the dead branch claims") {
		t.Fatalf("done = %q", done)
	}
	if again := pruneClaims(root, true); len(again) != 0 {
		t.Fatalf("a second prune reported %q; with no claims it must say nothing", again)
	}
}
