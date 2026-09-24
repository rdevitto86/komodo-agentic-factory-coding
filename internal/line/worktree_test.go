package line

import (
	"encoding/json"
	"komodo/internal/git"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

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

func TestAddWorktreeRefusesAPushFromATaskWorktree(t *testing.T) {
	root, _ := remotedRepo(t)
	worktree := filepath.Join(root, StateDir, "wt", "TSK-01.1.1")
	if err := AddWorktree(root, "task/x", "main", worktree); err != nil {
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
	if err := AddWorktree(root, "task/x", "main", worktree); err != nil {
		t.Fatalf("a global extensions.worktreeConfig must not skip the repo-local write: %v", err)
	}
	if on, err := git.Run(root, "config", "--local", "--get", "extensions.worktreeConfig"); err != nil || on != "true" {
		t.Fatalf("local extensions.worktreeConfig = %q, %v; want true", on, err)
	}
	if url, err := git.Run(worktree, "config", "--worktree", "--get", "remote.origin.pushurl"); err != nil || url != RefusedPushURL {
		t.Fatalf("pushurl = %q, %v; want the refusal", url, err)
	}
}

func TestAddWorktreeLeavesAPushFromTheRootWorking(t *testing.T) {
	root, bare := remotedRepo(t)
	worktree := filepath.Join(root, StateDir, "wt", "TSK-01.1.1")
	if err := AddWorktree(root, "task/x", "main", worktree); err != nil {
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

func TestAddWorktreeSkipsTheRefusalWhenCommonConfigHoldsCoreWorktree(t *testing.T) {
	root, _ := remotedRepo(t)
	if _, err := git.Run(root, "config", "core.worktree", "."); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(root, StateDir, "wt", "TSK-01.1.1")
	if err := AddWorktree(root, "task/x", "main", worktree); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(worktree, "config", "--worktree", "--get", "remote.origin.pushurl"); err == nil {
		t.Fatal("a repo whose common config holds core.worktree must skip the pushurl refusal")
	}
}

func TestAddWorktreeStillCutsWhenCommonConfigIsBare(t *testing.T) {
	root, _ := remotedRepo(t)
	if _, err := git.Run(root, "config", "core.bare", "true"); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(root, StateDir, "wt", "TSK-01.1.1")
	if err := AddWorktree(root, "task/x", "main", worktree); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "a.txt")); err != nil {
		t.Fatalf("a bare common config must still cut the worktree: %v", err)
	}
	if _, err := git.Run(worktree, "config", "--worktree", "--get", "remote.origin.pushurl"); err == nil {
		t.Fatal("a repo whose common config is bare must write no worktree pushurl")
	}
}

func TestASecondCutLeavesTheSharedConfigAlone(t *testing.T) {
	root, _ := remotedRepo(t)
	if err := AddWorktree(root, "task/x", "main", filepath.Join(root, StateDir, "wt", "TSK-01.1.1")); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, ".git", "config")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(config, old, old); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, StateDir, "wt", "TSK-01.1.2")
	if err := AddWorktree(root, "task/y", "main", second); err != nil {
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
	err = AddWorktree(root, "task/x", "main", worktree)
	if err == nil || !strings.Contains(err.Error(), "refused pushurl") {
		t.Fatalf("err = %v; a pushurl write that fails must fail the cut", err)
	}
	if _, statErr := os.Stat(worktree); statErr == nil {
		t.Fatal("a cut without its push refusal must not be left for a builder")
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
	if err := AcquireLock(root, "TG-01.1"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(LockPath(root))
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
	if err := os.MkdirAll(filepath.Dir(LockPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LockEnv, "")
	err = AcquireLock(root, "TG-02.1")
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
	if err := os.MkdirAll(filepath.Dir(LockPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AcquireLock(root, "TG-02.1"); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(LockPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LockPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LockEnv, "")
	if err := CheckLock(root); err == nil || !strings.Contains(err.Error(), "TG-01.1") {
		t.Fatalf("err = %v; a live launcher's lock must stop a stranger's next --start", err)
	}
	t.Setenv(LockEnv, strconv.Itoa(holder.Process.Pid))
	if err := CheckLock(root); err != nil {
		t.Fatalf("the launcher's own session must pass its lock: %v", err)
	}
}

func TestReleaseLockFreesOnlyItsOwnLock(t *testing.T) {
	root := t.TempDir()
	if err := AcquireLock(root, "TG-01.1"); err != nil {
		t.Fatal(err)
	}
	ReleaseLock(root)
	if _, err := os.Stat(LockPath(root)); !os.IsNotExist(err) {
		t.Fatalf("stat = %v; the holder's release must remove the lock", err)
	}
}
