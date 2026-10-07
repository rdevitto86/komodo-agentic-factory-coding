package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/conductor"
)

// blockedGroup is the group index file of a group whose task a person set back to READY after it stopped.
const blockedGroup = "## [TG-1] A group [P: C] [READY]\n\n```yaml\ntype: feat\n```\n"

// blockedTask is the group's one task file.
const blockedTask = "- [ ] **TSK-1.1** Do it\n  - files: `a.go`\n"

// writeBlockedGroup writes a group id's folder under worktree with tgText as its the group index file, returning that path.
func writeBlockedGroup(t *testing.T, worktree, tgText string) string {
	t.Helper()
	seedEpic(t, worktree, "EPIC-1", "1.0.0", "feat")
	dir := backlog.GroupDirPath(worktree, "TG-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, backlog.GroupFileName)
	if err := os.WriteFile(path, []byte(tgText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsk-1.1.md"), []byte(blockedTask), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// blockedRun saves a Blocked state whose worktree holds the noted group folder, its heading at status.
func blockedRun(t *testing.T, status string) (root, path string) {
	t.Helper()
	root = t.TempDir()
	worktree := t.TempDir()
	noted, err := backlog.AddGroupFileNote(blockedGroup, backlog.BlockerNote{At: time.Now(), Run: "run-1", State: "Building"})
	if err != nil {
		t.Fatal(err)
	}
	noted = strings.Replace(noted, "[P: C] [BLOCKED]", "[P: C] ["+status+"]", 1)
	path = writeBlockedGroup(t, worktree, noted)
	state := conductor.State{Group: "TG-1", Current: conductor.Blocked, Worktree: worktree}
	if err := conductor.SaveState(conductor.StatePath(root, "TG-1"), state); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func TestResumeClearsTheNoteOfAGroupAPersonSetReady(t *testing.T) {
	root, path := blockedRun(t, "READY")
	runResume(root, []string{"TG-1", "--json"})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != blockedGroup {
		t.Fatalf("backlog =\n%s\nwant the edited group without its note", data)
	}
	state, err := conductor.LoadState(conductor.StatePath(root, "TG-1"))
	if err != nil || !state.Edited {
		t.Fatalf("state = %+v, %v; want it marked edited", state, err)
	}
}

func TestResumeRefusesAGroupStillBlocked(t *testing.T) {
	root, path := blockedRun(t, "BLOCKED")
	oldExit := exit
	defer func() { exit = oldExit }()
	var code int
	exit = func(c int) { code = c; panic("exit") }
	defer func() {
		if recover() == nil || code != 1 {
			t.Fatalf("exit code = %d, want 1 for a group still BLOCKED", code)
		}
		if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "> **Blocked**") {
			t.Fatalf("backlog = %s, %v; want the note kept", data, err)
		}
	}()
	runResume(root, []string{"TG-1"})
}

func TestClearBlockerNamesAMissingBacklogOrGroup(t *testing.T) {
	if err := clearBlocker(conductor.State{Group: "TG-1", Worktree: t.TempDir()}); err == nil {
		t.Fatal("clear = nil, want an error for a worktree with no backlog")
	}
	worktree := t.TempDir()
	writeBlockedGroup(t, worktree, blockedGroup)
	if err := clearBlocker(conductor.State{Group: "TG-9", Worktree: worktree}); err == nil {
		t.Fatal("clear = nil, want an error for a group the backlog lacks")
	}
}
