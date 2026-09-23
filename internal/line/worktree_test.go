package line

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

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
