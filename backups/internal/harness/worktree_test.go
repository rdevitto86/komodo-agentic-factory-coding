package harness

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"komodo/internal/git"
	"komodo/internal/ledger"
)

// twoGroupBacklog holds two groups on disjoint files and a third that overlaps the first.
const twoGroupBacklog = "### [TG-15.1] First\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-15.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-15.2] Second\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
	"#### [TSK-15.2.1] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-15.3] Third\n```yaml\ntype: feat\nversion: 2.2.0\n```\n\n" +
	"#### [TSK-15.3.1] Three [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n"

// twoOpenRuns cuts the first two groups side by side, each task briefed.
func twoOpenRuns(t *testing.T) string {
	t.Helper()
	root := repo(t, twoGroupBacklog)
	started := time.Now().UTC()
	for index, group := range []string{"TG-15.1", "TG-15.2"} {
		state := RunState{Run: group + "-1", Group: group, Base: "main", Branch: "feat/" + group, Worktree: root,
			Started: started.Add(time.Duration(index) * time.Second)}
		if err := SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// remotedRepo builds a real git repo with one commit on main, remoted at a bare origin in a temp dir.
func remotedRepo(t *testing.T) (root, bare string) {
	t.Helper()
	root = gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	bare = filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "remote", "add", "origin", bare)
	return root, bare
}

func TestAddDetachedRefusesAPushFromATaskWorktree(t *testing.T) {
	root, _ := remotedRepo(t)
	worktree := filepath.Join(root, StateDir, "wt", "TSK-01.1.1")
	if err := AddDetached(root, "task/x", "main", worktree); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "push", "origin", "task/x")
	cmd.Dir = worktree
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("a push from a task worktree must be refused, out = %s", out)
	}
	if !strings.Contains(string(out), "refused") {
		t.Fatalf("out = %s; the refusal must name refused", out)
	}
}

func TestAGlobalWorktreeConfigStillEnablesItInTheRepo(t *testing.T) {
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[extensions]\n\tworktreeConfig = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	root, _ := remotedRepo(t)
	worktree := filepath.Join(root, StateDir, "wt", "TSK-01.1.1")
	if err := AddDetached(root, "task/x", "main", worktree); err != nil {
		t.Fatalf("a global extensions.worktreeConfig must not skip the repo-local write: %v", err)
	}
	if on, err := git.Run(root, "config", "--local", "--get", "extensions.worktreeConfig"); err != nil || on != "true" {
		t.Fatalf("local extensions.worktreeConfig = %q, %v; want true", on, err)
	}
	if url, err := git.Run(worktree, "config", "--worktree", "--get", "remote.origin.pushurl"); err != nil || url != RefusedPushURL {
		t.Fatalf("pushurl = %q, %v; want the refusal", url, err)
	}
}

func TestAddDetachedLeavesAPushFromTheRootWorking(t *testing.T) {
	root, bare := remotedRepo(t)
	worktree := filepath.Join(root, StateDir, "wt", "TSK-01.1.1")
	if err := AddDetached(root, "task/x", "main", worktree); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "push", "origin", "main"); err != nil {
		t.Fatalf("a push from the root must still work: %v", err)
	}
	if _, err := git.Run(root, "config", "--get", "remote.origin.pushurl"); err == nil {
		t.Fatal("the main checkout's own config must never gain a pushurl from a worktree cut")
	}
	out, err := exec.Command("git", "-C", bare, "branch", "--list", "main").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "main") {
		t.Fatalf("branch = %s; the root's push to the bare remote must have landed", out)
	}
}

func TestASecondCutLeavesTheSharedConfigAlone(t *testing.T) {
	root, _ := remotedRepo(t)
	if err := AddDetached(root, "task/x", "main", filepath.Join(root, StateDir, "wt", "TSK-01.1.1")); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, ".git", "config")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(config, old, old); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, StateDir, "wt", "TSK-01.1.2")
	if err := AddDetached(root, "task/y", "main", second); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(config)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Fatalf("config written at %v; a second cut must not rewrite extensions.worktreeConfig", info.ModTime())
	}
	if got, err := git.Run(second, "config", "--worktree", "--get", "remote.origin.pushurl"); err != nil || got != RefusedPushURL {
		t.Fatalf("pushurl = %q, %v; the second cut must still refuse a push", got, err)
	}
}

func TestAFailedPushurlWriteFailsTheCut(t *testing.T) {
	root, _ := remotedRepo(t)
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	fakes := t.TempDir()
	wrapper := "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"--worktree\" ]; then echo 'error: could not lock config file' >&2; exit 255; fi\ndone\nexec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(fakes, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakes+string(os.PathListSeparator)+os.Getenv("PATH"))
	worktree := filepath.Join(root, StateDir, "wt", "TSK-01.1.1")
	err = AddDetached(root, "task/x", "main", worktree)
	if err == nil || !strings.Contains(err.Error(), "refused pushurl") {
		t.Fatalf("err = %v; a pushurl write that fails must fail the cut", err)
	}
	if _, statErr := os.Stat(worktree); statErr == nil {
		t.Fatal("a cut without its push refusal must not be left for a builder")
	}
}

func TestAddDetachedCutsADetachedWorktreeTrackingItsBranch(t *testing.T) {
	root, _ := remotedRepo(t)
	path := filepath.Join(root, StateDir, "wt", "ad-hoc")
	if err := AddDetached(root, "task/x", "main", path); err != nil {
		t.Fatal(err)
	}
	if branch, err := git.Run(path, "symbolic-ref", "--short", "HEAD"); err == nil {
		t.Fatalf("HEAD = %s; a cut worktree must be detached", branch)
	}
	if got := git.Or(path, "config", "--worktree", "--get", "komodo.branch"); got != "task/x" {
		t.Fatalf("komodo.branch = %q, want task/x", got)
	}
	if _, err := git.Run(root, "rev-parse", "--verify", TipRef("task/x")); err != nil {
		t.Fatalf("TipRef must exist after a cut: %v", err)
	}
}

func TestAddDetachedRefusesACriticalBranch(t *testing.T) {
	root, _ := remotedRepo(t)
	path := filepath.Join(root, StateDir, "wt", "ad-hoc")
	err := AddDetached(root, "main", "main", path)
	if err == nil || !strings.Contains(err.Error(), "critical") {
		t.Fatalf("err = %v; a critical branch must be refused", err)
	}
}

func TestAddDetachedRefusesASecondWorktreeTrackingTheSameBranch(t *testing.T) {
	root, _ := remotedRepo(t)
	first := filepath.Join(root, StateDir, "wt", "a")
	if err := AddDetached(root, "task/x", "main", first); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, StateDir, "wt", "b")
	err := AddDetached(root, "task/x", "main", second)
	if err == nil || !strings.Contains(err.Error(), "already tracks") {
		t.Fatalf("err = %v; a second worktree tracking the same branch must be refused", err)
	}
}

func TestAddDetachedResumesFromAnEarlierTipAfterTheFirstWorktreeIsGone(t *testing.T) {
	root, _ := remotedRepo(t)
	first := filepath.Join(root, StateDir, "wt", "a")
	if err := AddDetached(root, "task/x", "main", first); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "work.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, first, "add", "-A")
	runGit(t, first, "commit", "-q", "-m", "work")
	old, err := git.Run(root, "rev-parse", TipRef("task/x"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(root, "task/x", first, old); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "worktree", "remove", "--force", first); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, StateDir, "wt", "b")
	if err := AddDetached(root, "task/x", "main", second); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(second, "work.txt")); err != nil {
		t.Fatalf("a resumed cut must start from the advanced tip, not startRef: %v", err)
	}
}

func TestAdvanceRefusesAStaleOldValue(t *testing.T) {
	root, _ := remotedRepo(t)
	worktree := filepath.Join(root, StateDir, "wt", "a")
	if err := AddDetached(root, "task/x", "main", worktree); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "work.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "-A")
	runGit(t, worktree, "commit", "-q", "-m", "work")
	stale := strings.Repeat("0", 40)
	if err := Advance(root, "task/x", worktree, stale); err == nil {
		t.Fatal("Advance with a stale old value must be refused")
	}
}

func TestResultIsFoundInTheWorktreeTheBuilderIsSandboxedTo(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, StateDir, "wt", "TG-01.1")
	state := RunState{Run: "TG-01.1-1", Group: "TG-01.1", Base: "main", Branch: "feat/a", Worktree: worktree}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	path := ResultPath(worktree, "TSK-01.1.1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"result":"DONE"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasResult(root, "TSK-01.1.1") {
		t.Fatal("a result written inside the worktree was not found")
	}
	parsed, err := ReadResult(root, "TSK-01.1.1")
	if err != nil || parsed["result"] != "DONE" {
		t.Fatalf("parsed = %v, err = %v", parsed, err)
	}
}

func TestResultAtTheRootIsStillFound(t *testing.T) {
	root := t.TempDir()
	path := ResultPath(root, "TSK-01.1.1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"result":"DONE"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasResult(root, "TSK-01.1.1") {
		t.Fatal("a result at the root was not found")
	}
}

func TestResultIsFoundInTheTasksOwnWorktree(t *testing.T) {
	root := t.TempDir()
	state := RunState{
		Run: "TG-01.1-1", Group: "TG-01.1", Base: "main", Branch: "feat/a",
		Worktree: filepath.Join(root, StateDir, "wt", "TG-01.1"),
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	path := ResultPath(filepath.Join(root, StateDir, "wt", "TSK-01.1.1"), "TSK-01.1.1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"result":"DONE"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasResult(root, "TSK-01.1.1") {
		t.Fatal("a result in the task's own worktree was not found")
	}
}

func TestAnAbsoluteWorktreeIsNotJoinedToTheRoot(t *testing.T) {
	root := t.TempDir()
	absolute := filepath.Join(root, StateDir, "wt", "TG-09.1")
	if got := WorktreePath(root, absolute); got != absolute {
		t.Fatalf("path = %s; a run records its worktree absolute and joining it to the root points nowhere", got)
	}
	relative := filepath.Join(StateDir, "wt", "TG-09.1")
	if got := WorktreePath(root, relative); got != absolute {
		t.Fatalf("path = %s, want %s; a plan builds its worktree relative to the root", got, absolute)
	}
	if got := WorktreePath(root, ""); got != root {
		t.Fatalf("path = %s; no worktree means the root", got)
	}
}

func TestAcquireLockTakesAFreshLock(t *testing.T) {
	root := t.TempDir()
	if err := AcquireLock(root, "", "TG-01.1"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(LockPath(root, ""))
	if err != nil {
		t.Fatal(err)
	}
	var held RunLock
	if err := json.Unmarshal(data, &held); err != nil {
		t.Fatal(err)
	}
	if held.PID != os.Getpid() || held.Run != "TG-01.1" {
		t.Fatalf("lock = %+v", held)
	}
}

func TestAcquireLockRefusesALivePid(t *testing.T) {
	root := t.TempDir()
	holder := exec.Command("sleep", "30")
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Process.Kill(); _ = holder.Wait() })
	data, err := json.Marshal(RunLock{PID: holder.Process.Pid, Run: "TG-01.1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(LockPath(root, "")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(root, ""), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LockEnv, "")
	err = AcquireLock(root, "", "TG-02.1")
	if err == nil {
		t.Fatal("a second run must not steal a lock a live pid holds")
	}
	if !strings.Contains(err.Error(), "TG-01.1") {
		t.Fatalf("err = %v, want it to name the holder", err)
	}
}

func TestAcquireLockReclaimsADeadPid(t *testing.T) {
	root := t.TempDir()
	child := exec.Command("true")
	if err := child.Run(); err != nil {
		t.Fatal(err)
	}
	dead := RunLock{PID: child.Process.Pid, Run: "TG-01.1"}
	data, err := json.Marshal(dead)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(LockPath(root, "")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(root, ""), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AcquireLock(root, "", "TG-02.1"); err != nil {
		t.Fatalf("a lock whose pid has exited must be reclaimed: %v", err)
	}
}

func TestTheLaunchersOwnSessionPassesTheLockAndAStrangerDoesNot(t *testing.T) {
	root := t.TempDir()
	holder := exec.Command("sleep", "30")
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Process.Kill(); _ = holder.Wait() })
	data, err := json.Marshal(RunLock{PID: holder.Process.Pid, Run: "TG-01.1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(LockPath(root, "")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(root, ""), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LockEnv, "")
	if err := CheckLock(root, ""); err == nil || !strings.Contains(err.Error(), "TG-01.1") {
		t.Fatalf("err = %v; a live launcher's lock must stop a stranger's next --start", err)
	}
	t.Setenv(LockEnv, strconv.Itoa(holder.Process.Pid))
	if err := CheckLock(root, ""); err != nil {
		t.Fatalf("the launcher's own session must pass its lock: %v", err)
	}
}

func TestReleaseLockFreesOnlyItsOwnLock(t *testing.T) {
	root := t.TempDir()
	if err := AcquireLock(root, "", "TG-01.1"); err != nil {
		t.Fatal(err)
	}
	ReleaseLock(root, "")
	if _, err := os.Stat(LockPath(root, "")); !os.IsNotExist(err) {
		t.Fatalf("stat = %v; the holder's release must remove the lock", err)
	}
}

// holdLock writes group's lock held by a live sleep, and clears LockEnv so this process is a stranger.
func holdLock(t *testing.T, root, group, run string) {
	t.Helper()
	holder := exec.Command("sleep", "30")
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Process.Kill(); _ = holder.Wait() })
	data, err := json.Marshal(RunLock{PID: holder.Process.Pid, Run: run})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(LockPath(root, group)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(root, group), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LockEnv, "")
}

func TestTwoGroupsEachHoldTheirOwnLock(t *testing.T) {
	root := t.TempDir()
	holdLock(t, root, "TG-01.1", "TG-01.1")
	if got := LockPath(root, "TG-01.1"); got != filepath.Join(root, StateDir, "runs", "TG-01.1", "run.lock") {
		t.Fatalf("lock path = %s", got)
	}
	if err := AcquireLock(root, "TG-02.1", "TG-02.1"); err != nil {
		t.Fatalf("a second group must take its own lock beside a live first: %v", err)
	}
	if err := CheckLock(root, "TG-01.1"); err == nil || !strings.Contains(err.Error(), "TG-01.1") {
		t.Fatalf("err = %v; a group's live lock must stop a second launcher on that group", err)
	}
	if err := CheckLock(root, ""); err == nil {
		t.Fatal("a repo-wide drain must not start while a group's launcher is live")
	}
}

func TestTheRepoLockCoversEveryGroup(t *testing.T) {
	root := t.TempDir()
	holdLock(t, root, "", "the open run")
	if err := AcquireLock(root, "TG-02.1", "TG-02.1"); err == nil || !strings.Contains(err.Error(), "the open run") {
		t.Fatalf("err = %v; a live drain must stop a group launcher", err)
	}
}

// stageFixtures copies a staged repo's group files and its builder role into root.
func stageFixtures(t *testing.T, staged, root string) {
	t.Helper()
	copyBacklogTree(t, staged, root)
	role := filepath.Join(RolesDir, "builder.md")
	data, err := os.ReadFile(filepath.Join(staged, role))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, RolesDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, role), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// cutRepo is a real repo pushed to a bare origin, holding text as its backlog and a builder role.
func cutRepo(t *testing.T, text string) string {
	t.Helper()
	root, _ := remotedRepo(t)
	runGit(t, root, "push", "origin", "main")
	// The seeded epic ships a version; its branch on origin keeps a cut from opening the epic's pull request here.
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	stageFixtures(t, repo(t, text), root)
	return root
}

// freshPlan is the plan intake builds for one group, failing the test when there is none.
func freshPlan(t *testing.T, root, group string) *Plan {
	t.Helper()
	plan, err := next(root, group)
	if err != nil || plan == nil {
		t.Fatalf("plan %s = %v, err = %v", group, plan, err)
	}
	return plan
}

func TestTwoConcurrentCutsOfOverlappingGroupsLetExactlyOneThrough(t *testing.T) {
	root := cutRepo(t, twoGroupBacklog)
	plans := []*Plan{freshPlan(t, root, "TG-15.1"), freshPlan(t, root, "TG-15.3")}
	errs := make([]error, len(plans))
	var wait sync.WaitGroup
	for index, plan := range plans {
		wait.Add(1)
		go func(index int, plan *Plan) {
			defer wait.Done()
			_, errs[index] = Start(root, plan, "", false)
		}(index, plan)
	}
	wait.Wait()
	cut := 0
	for _, err := range errs {
		if err == nil {
			cut++
		} else if !strings.Contains(err.Error(), "is open and not shipped") {
			t.Fatalf("err = %v; the losing cut must be refused for the overlap", err)
		}
	}
	if cut != 1 {
		t.Fatalf("errs = %v; exactly one of two overlapping cuts must succeed", errs)
	}
	if _, err := os.Stat(CutLockPath(root)); !os.IsNotExist(err) {
		t.Fatalf("stat = %v; the cut lock must be released on return", err)
	}
}

func TestCuttingASecondGroupKeepsAShippedGroupALiveProcessStillDrives(t *testing.T) {
	root := cutRepo(t, twoGroupBacklog)
	if _, err := Start(root, freshPlan(t, root, "TG-15.1"), "", false); err != nil {
		t.Fatal(err)
	}
	Stamp(root, ledger.Entry{Group: "TG-15.1", Station: "ship", Outcome: "done"})
	held, err := json.Marshal(RunLock{PID: os.Getppid(), Run: "TG-15.1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(root, "TG-15.1"), held, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(root, freshPlan(t, root, "TG-15.2"), "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(RunDir(root, "TG-15.1"), "run.json")); err != nil {
		t.Fatalf("TG-15.1 lost its run directory while a live process held its lock: %v", err)
	}
}

func TestCuttingASecondGroupKeepsTheFirstGroupsRunAndLedger(t *testing.T) {
	root := cutRepo(t, twoGroupBacklog)
	if _, err := Start(root, freshPlan(t, root, "TG-15.1"), "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(root, freshPlan(t, root, "TG-15.2"), "", false); err != nil {
		t.Fatalf("a group on disjoint files must cut beside an open one: %v", err)
	}
	for _, group := range []string{"TG-15.1", "TG-15.2"} {
		if _, err := os.Stat(filepath.Join(RunDir(root, group), "run.json")); err != nil {
			t.Fatalf("%s lost its run directory: %v", group, err)
		}
	}
	entries, err := Book(root).Read(ledger.RunFile)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.Station == "intake" {
			seen[entry.Group] = true
		}
	}
	if !seen["TG-15.1"] || !seen["TG-15.2"] {
		t.Fatalf("ledger = %+v; the second cut must keep the first group's rows", entries)
	}
}

func TestAGroupWithAFilelessTaskOverlapsEveryOpenGroup(t *testing.T) {
	root := twoOpenRuns(t)
	fileless := "\n### [TG-15.4] Fourth\n```yaml\ntype: feat\nversion: 2.3.0\n```\n\n" +
		"#### [TSK-15.4.1] Four [P: C] [READY]\n```yaml\ndone_when: [\"true\"]\n```\n"
	reseed(t, root, twoGroupBacklog+fileless)
	if err := RefuseOpenRun(root, "TG-15.4"); err == nil || !strings.Contains(err.Error(), "is open") {
		t.Fatalf("err = %v; a task declaring no files may touch any, so its group must be refused", err)
	}
}

func TestALegacyStatusIsMergedIntoItsGroupsAndRemoved(t *testing.T) {
	root := t.TempDir()
	state := RunState{Run: "TG-16.1-1", Group: "TG-16.1", Base: "main", Branch: "feat/legacy"}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	legacyStatus := `{"TSK-16.1.1":{"status":"DONE"},"TSK-16.1.2":{"status":"BLOCKED"}}`
	groupStatus := `{"TSK-16.1.2":{"status":"DONE"}}`
	for path, body := range map[string]string{
		filepath.Join(root, StateDir, "run.json"):             string(data),
		filepath.Join(root, StateDir, "status.json"):          legacyStatus,
		filepath.Join(RunDir(root, "TG-16.1"), "status.json"): groupStatus,
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	LoadRuns(root)
	if _, err := os.Stat(filepath.Join(root, StateDir, "status.json")); !os.IsNotExist(err) {
		t.Fatalf("stat = %v; the legacy status must be removed once merged", err)
	}
	merged := loadStatusFile(filepath.Join(RunDir(root, "TG-16.1"), "status.json"))
	if merged["TSK-16.1.1"].Status != "DONE" || merged["TSK-16.1.2"].Status != "DONE" {
		t.Fatalf("status = %+v; the legacy entry must join and the group's own must win", merged)
	}
}

// TestAddDetachedCutsFromBaseNotFromStaleOrigin proves a branch that only a stale origin/feat/a still holds
// is cut fresh from its base, without that origin branch's old commit.
func TestAddDetachedCutsFromBaseNotFromStaleOrigin(t *testing.T) {
	root, _ := remotedRepo(t)
	if _, err := git.Run(root, "push", "origin", "main"); err != nil {
		t.Fatal(err)
	}
	worktree1 := filepath.Join(root, StateDir, "wt", "TSK-20.1.1")
	if err := AddDetached(root, "feat/a", "main", worktree1); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree1, "file.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(worktree1, "add", "file.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(worktree1, "commit", "-m", "add file"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "push", "origin", TipRef("feat/a")+":refs/heads/feat/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "worktree", "remove", worktree1); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "update-ref", "-d", TipRef("feat/a")); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "refs/heads/feat/a"); err == nil {
		t.Fatal("feat/a exists locally; the scenario needs only origin/feat/a")
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "origin/feat/a"); err != nil {
		t.Fatalf("origin/feat/a must exist: %v", err)
	}
	worktree2 := filepath.Join(root, StateDir, "wt", "TSK-20.1.2")
	if err := AddDetached(root, "feat/a", "main", worktree2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree2, "file.txt")); err == nil {
		t.Fatal("new worktree must not inherit stale commits from origin/feat/a")
	}
	if _, err := os.Stat(filepath.Join(worktree2, "a.txt")); err != nil {
		t.Fatalf("new worktree must have base commit from main: %v", err)
	}
}

// TestAPersonOwnsEveryBranchTheLineCut proves a cut branch is free for a person to take: checked
// out, committed to, and deleted in the root, all while the line's own worktree lives.
func TestAPersonOwnsEveryBranchTheLineCut(t *testing.T) {
	root := cutRepo(t, twoGroupBacklog)
	state, err := Start(root, freshPlan(t, root, "TG-15.1"), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "checkout", "-b", state.Branch, TipRef(state.Branch)); err != nil {
		t.Fatalf("a person must still check out the line's branch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "person.txt"), []byte("person\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "a person's own commit")
	runGit(t, root, "checkout", "main")
	if _, err := git.Run(root, "branch", "-D", state.Branch); err != nil {
		t.Fatalf("a person must still delete the line's branch: %v", err)
	}
}

// TestStartCutsAGroupFromTheFetchedOriginMain proves Start's own fetch of the base is not wasted:
// a group cut after a GitHub merge carries origin/main forward, not a stale local main.
func TestStartCutsAGroupFromTheFetchedOriginMain(t *testing.T) {
	root, bare := remotedRepo(t)
	runGit(t, root, "push", "origin", "main")
	stageFixtures(t, repo(t, twoGroupBacklog), root)
	// A GitHub merge lands on origin/main; local main never moves.
	clone := t.TempDir()
	runGit(t, "", "clone", bare, clone)
	if err := os.WriteFile(filepath.Join(clone, "merged.txt"), []byte("merged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, clone, "add", "-A")
	runGit(t, clone, "commit", "-m", "merged upstream")
	runGit(t, clone, "push", "origin", "main")
	// The epic's branch is cut from that merged main on origin; the root holds no copy of it at all.
	runGit(t, clone, "push", "origin", "main:refs/heads/feat/2.0.0")

	state, err := Start(root, freshPlan(t, root, "TG-15.1"), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state.Worktree, "merged.txt")); err != nil {
		t.Fatalf("stat = %v; a group cut must carry the fetched origin/main forward", err)
	}
}

// TestWriteBriefCutsATaskFromTheLocalGroupBranchNotStaleOrigin proves a task worktree is cut from
// its group branch's local tip: an earlier push must not leave a task behind a fresher local commit.
func TestWriteBriefCutsATaskFromTheLocalGroupBranchNotStaleOrigin(t *testing.T) {
	root, _ := remotedRepo(t)
	groupBranch := "feat/g"
	runGit(t, root, "checkout", "-b", groupBranch)
	runGit(t, root, "push", "origin", groupBranch)
	// The group branch gains a local commit after the push, which origin/<groupBranch> never sees.
	if err := os.WriteFile(filepath.Join(root, "local.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "local only")

	worktree := filepath.Join(root, StateDir, "wt", "TSK-20.2.1")
	brief := &Brief{Task: "TSK-20.2.1", Worktree: filepath.Join(StateDir, "wt", "TSK-20.2.1"), Path: "brief.md", Text: "brief"}
	if err := WriteBrief(root, brief, groupBranch); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "local.txt")); err != nil {
		t.Fatalf("stat = %v; a task must be cut from the group branch's local tip", err)
	}
}

// TestMigrateRunKeepsTheLegacyFileWhenItsRenameFails blocks the rename into the group's own run
// directory, so the legacy run.json must survive as the state's one remaining copy.
func TestMigrateRunKeepsTheLegacyFileWhenItsRenameFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory mode does not block a rename on Windows")
	}
	root := t.TempDir()
	state := RunState{Run: "TG-16.2-1", Group: "TG-16.2", Base: "main", Branch: "feat/legacy-keep"}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, StateDir, "run.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, data, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := RunDir(root, state.Group)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	LoadRuns(root)

	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("stat = %v; a failed rename must keep the legacy run.json", err)
	}
}

// TestSaveRunNeverLeavesLoadRunForReadingATornFile races a writer against a reader on the same
// run.json, so the reader always sees a whole state, never a truncated or half-written one.
func TestSaveRunNeverLeavesLoadRunForReadingATornFile(t *testing.T) {
	root := t.TempDir()
	states := []RunState{
		{Run: "TG-17.1-1", Group: "TG-17.1", Base: "main", Branch: "feat/a", Waves: [][]string{{"TSK-17.1.1"}}},
		{Run: "TG-17.1-2", Group: "TG-17.1", Base: "main", Branch: "feat/b"},
	}
	if err := SaveRun(root, states[0]); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if err := SaveRun(root, states[i%2]); err != nil {
				t.Errorf("SaveRun = %v", err)
				return
			}
		}
	}()
	for i := 0; i < 2000; i++ {
		if _, err := LoadRunFor(root, "TG-17.1"); err != nil {
			t.Errorf("LoadRunFor = %v; a concurrent SaveRun must never leave a torn read", err)
			break
		}
	}
	close(stop)
	wait.Wait()
}

// TestStartNamesAndDatesARunFromOneClockReading fixes the cut's clock and proves the run id and
// start time both come from that one reading.
func TestStartNamesAndDatesARunFromOneClockReading(t *testing.T) {
	root := cutRepo(t, twoGroupBacklog)
	fixed := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	saved := now
	t.Cleanup(func() { now = saved })
	now = func() time.Time { return fixed }
	state, err := Start(root, freshPlan(t, root, "TG-15.1"), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "TG-15.1-" + strconv.FormatInt(fixed.Unix(), 10); state.Run != want || !state.Started.Equal(fixed) {
		t.Fatalf("run = %s started %s, want %s started %s", state.Run, state.Started, want, fixed)
	}
}

// TestARunKeepsTheBinaryItWasCutWith records the cutting binary in run.json, then proves a rebuild mid-run
// changes neither run.json nor what the run's next ledger entry carries.
func TestARunKeepsTheBinaryItWasCutWith(t *testing.T) {
	root := cutRepo(t, twoGroupBacklog)
	saved := runningBinary
	t.Cleanup(func() { runningBinary = saved })
	runningBinary = func() string { return "1.0.0-beta.6 (0123456789ab)" }
	if _, err := Start(root, freshPlan(t, root, "TG-15.1"), "", false); err != nil {
		t.Fatal(err)
	}
	runningBinary = func() string { return "1.0.0-beta.7 (fedcba987654)" }
	state, err := LoadRunFor(root, "TG-15.1")
	if err != nil || state.Binary != "1.0.0-beta.6 (0123456789ab)" {
		t.Fatalf("run.json binary = %q, %v; want the cutting binary", state.Binary, err)
	}
	Stamp(root, ledger.Entry{Group: "TG-15.1", Station: "build"})
	entries, err := Book(root).Read(ledger.RunFile)
	if err != nil || len(entries) == 0 {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	for _, entry := range entries {
		if entry.Binary != state.Binary {
			t.Fatalf("%s entry binary = %q, want %q", entry.Station, entry.Binary, state.Binary)
		}
	}
}
