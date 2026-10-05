package conductor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/line"
)

// abandonGroup is the one group a test repo holds: a done task and an open one.
var abandonGroup = backlog.GroupFile{ID: "TG-1", Title: "A group", Priority: "C", Status: "READY", Type: "feat", Version: "1.0.0",
	Tasks: []backlog.GroupTask{
		{ID: "TSK-1.1", Title: "Done already", Done: true, Files: []string{"a.go"}},
		{ID: "TSK-1.2", Title: "Still open", Files: []string{"b.go"}},
	}}

// abandonGroupFile is the group's the group index file under root, where its blocker note lands.
func abandonGroupFile(root string) string {
	return filepath.Join(backlog.GroupDirPath(root, "TG-1"), backlog.GroupFileName)
}

// abandonRepo builds a repo whose one group was cut into its own worktree and branch, with a run record.
func abandonRepo(t *testing.T) (root, worktree string) {
	t.Helper()
	root = t.TempDir()
	gitIn := func(dir string, args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	gitIn(root, "init", "-b", "main")
	gitIn(root, "config", "user.email", "a@example.com")
	gitIn(root, "config", "user.name", "a")
	backlogtest.Seed(t, root, abandonGroup)
	gitIn(root, "add", "-A")
	gitIn(root, "commit", "-m", "seed")
	worktree = filepath.Join(root, line.StateDir, "wt", "TG-1")
	gitIn(root, "worktree", "add", "--detach", worktree)
	gitIn(root, "update-ref", line.TipRef("feat/a-group"), "HEAD")
	gitIn(root, "branch", "feat/a-group")
	run := line.RunState{Run: "run-1", Group: "TG-1", Base: "main", Branch: "feat/a-group", Worktree: worktree}
	if err := line.SaveRun(root, run); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(StatePath(root, "TG-1"), State{Group: "TG-1", Current: Blocked}); err != nil {
		t.Fatal(err)
	}
	return root, worktree
}

func TestAbandonRemovesTheWorktreeAndTipAndBlocksTheGroup(t *testing.T) {
	root, worktree := abandonRepo(t)
	at := time.Date(2026, 9, 27, 10, 30, 0, 0, time.UTC)
	if err := Abandon(root, "TG-1", at); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("worktree stat = %v, want it removed", err)
	}
	if err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", line.TipRef("feat/a-group")).Run(); err == nil {
		t.Fatal("the group's tip ref survives; abandon must drop it")
	}
	if out, err := exec.Command("git", "-C", root, "branch", "--list", "feat/a-group").Output(); err != nil ||
		strings.TrimSpace(string(out)) == "" {
		t.Fatalf("branch list = %q, %v; abandon must never delete a person's local branch", out, err)
	}
	if _, err := os.Stat(line.RunDir(root, "TG-1")); !os.IsNotExist(err) {
		t.Fatalf("run dir stat = %v, want the run record removed", err)
	}
	data, err := os.ReadFile(abandonGroupFile(root))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"> **Blocked** 2026-09-27 10:30, run run-1, at Blocked.",
		"abandoned on purpose with `komodo abandon`",
		"[TG-1] A group [P: C] [BLOCKED]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("backlog =\n%s\nwant %q", text, want)
		}
	}
	done, err := os.ReadFile(filepath.Join(backlog.GroupDirPath(root, "TG-1"), backlog.TaskFileName("TSK-1.1")))
	if err != nil || !strings.Contains(string(done), "- [x] **TSK-1.1**") {
		t.Fatalf("done task = %q, %v; abandon must leave a finished task ticked", done, err)
	}
}

func TestAbandonRefusesAGroupWithNoRunAndChangesNothing(t *testing.T) {
	root, _ := abandonRepo(t)
	before, err := os.ReadFile(abandonGroupFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := Abandon(root, "TG-9", time.Now()); err == nil {
		t.Fatal("abandon = nil, want a group never cut refused")
	}
	data, err := os.ReadFile(abandonGroupFile(root))
	if err != nil || string(data) != string(before) {
		t.Fatalf("backlog = %q, %v; want it untouched", data, err)
	}
}

func TestAbandonFindsTheDefaultWorktreeOfARunWithNoSavedState(t *testing.T) {
	root, worktree := abandonRepo(t)
	run := line.RunState{Run: "run-1", Group: "TG-1", Base: "main", Branch: "feat/a-group"}
	if err := line.SaveRun(root, run); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(StatePath(root, "TG-1")); err != nil {
		t.Fatal(err)
	}
	if err := Abandon(root, "TG-1", time.Date(2026, 9, 27, 10, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("worktree stat = %v, want the default worktree removed", err)
	}
	data, err := os.ReadFile(abandonGroupFile(root))
	if err != nil || !strings.Contains(string(data), "run run-1, at "+abandonedState+".") {
		t.Fatalf("backlog = %s, %v; want the note at %s", data, err, abandonedState)
	}
}

// TestAbandonNeverRemovesTheWorktreeWhenItCannotWriteTheNote proves the note is written before any
// destructive git step, so a failed write leaves the worktree, branch and run record all in place.
func TestAbandonNeverRemovesTheWorktreeWhenItCannotWriteTheNote(t *testing.T) {
	root, worktree := abandonRepo(t)
	path := abandonGroupFile(root)
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if err := Abandon(root, "TG-1", time.Now()); err == nil {
		t.Fatal("abandon = nil, want the note write's failure")
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("worktree stat = %v, want it kept since nothing was written yet", err)
	}
	if out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", line.TipRef("feat/a-group")).
		CombinedOutput(); err != nil {
		t.Fatalf("tip ref = %v, %s; want it kept since nothing was written yet", err, out)
	}
	if _, err := os.Stat(line.RunDir(root, "TG-1")); err != nil {
		t.Fatalf("run dir stat = %v, want the run record kept since nothing finished", err)
	}
}

func TestAbandonRefusesWhatItCannotNoteAndRemovesNothing(t *testing.T) {
	cases := []struct {
		name  string
		group string
		spoil func(t *testing.T, root string)
	}{
		{"a live run holds the group", "TG-1", func(t *testing.T, root string) {
			t.Setenv(line.LockEnv, "")
			lock := fmt.Sprintf(`{"pid":%d,"run":"run-2"}`, os.Getppid())
			if err := os.WriteFile(line.LockPath(root, "TG-1"), []byte(lock), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"the repo has no backlog", "TG-1", func(t *testing.T, root string) {
			if err := os.RemoveAll(filepath.Join(root, "docs", "backlog")); err != nil {
				t.Fatal(err)
			}
		}},
		{"the backlog holds no such group", "TG-2", func(t *testing.T, root string) {
			if err := line.SaveRun(root, line.RunState{Run: "run-1", Group: "TG-2", Branch: "feat/a-group"}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, worktree := abandonRepo(t)
			tc.spoil(t, root)
			if err := Abandon(root, tc.group, time.Now()); err == nil {
				t.Fatal("abandon = nil, want the refusal")
			}
			if _, err := os.Stat(worktree); err != nil {
				t.Fatalf("worktree stat = %v, want it kept", err)
			}
		})
	}
}

func TestAbandonRefusesAGroupOnACriticalBranch(t *testing.T) {
	root, worktree := abandonRepo(t)
	run := line.RunState{Run: "run-1", Group: "TG-1", Base: "main", Branch: "main", Worktree: worktree}
	if err := line.SaveRun(root, run); err != nil {
		t.Fatal(err)
	}
	if err := Abandon(root, "TG-1", time.Now()); err == nil {
		t.Fatal("abandon = nil, want a critical branch refused")
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("worktree stat = %v, want it kept", err)
	}
}
