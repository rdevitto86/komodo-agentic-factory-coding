package line

import (
	"os"
	"path/filepath"
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
