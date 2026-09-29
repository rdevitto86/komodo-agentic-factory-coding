package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskChecksAllowAStopWhenEveryCheckPasses(t *testing.T) {
	t.Parallel()
	root := groupRoot(t, "true")
	got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "pass", "cwd": root})
	if got.code != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("a passing group was refused: %+v", got)
	}
}

func TestTaskChecksRefuseAStopWithTheFailingOutput(t *testing.T) {
	t.Parallel()
	root := groupRoot(t, "echo broken-build && false")
	got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "fail", "cwd": root})
	if got.code != ExitRefuse {
		t.Fatalf("code = %d, want %d", got.code, ExitRefuse)
	}
	for _, want := range []string{"broken-build", "Fix it"} {
		if !strings.Contains(got.stderr, want) {
			t.Fatalf("refusal %q lacks %q", got.stderr, want)
		}
	}
}

func TestTaskChecksAllowWhenTheWorktreeNamesNoGroup(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Its own .git stops the worktree walk here; a sandbox's temp dir sits inside the real worktree.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "none", "cwd": root})
	if got.code != 0 || !strings.Contains(got.stderr, "allowing") {
		t.Fatalf("a hook that cannot find its checks did not allow: %+v", got)
	}
}

func TestTaskChecksMarkTheChecksTheyRun(t *testing.T) {
	t.Parallel()
	root := groupRoot(t, `test "$KOMODO_TASK_CHECKS" = 1`)
	got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "marked", "cwd": root})
	if got.code != 0 {
		t.Fatalf("a check the hook ran could not see the nesting marker: %+v", got)
	}
}

func TestTaskChecksAllowInsideACheckTheyStarted(t *testing.T) {
	t.Setenv("KOMODO_TASK_CHECKS", "1")
	root := groupRoot(t, "echo must-not-run && false")
	got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "nested", "cwd": root})
	if got.code != 0 || strings.Contains(got.stderr, "must-not-run") {
		t.Fatalf("a nested hook ran its checks again: %+v", got)
	}
}

func TestTaskChecksRunAWorktreeNamedForOneTask(t *testing.T) {
	t.Parallel()
	group := groupRoot(t, "echo task-check && false")
	root := filepath.Join(filepath.Dir(group), "TSK-90.1.1")
	if err := os.Rename(group, root); err != nil {
		t.Fatal(err)
	}
	got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "task", "cwd": root})
	if got.code != ExitRefuse || !strings.Contains(got.stderr, "task-check") {
		t.Fatalf("the task's check did not refuse: %+v", got)
	}
}

func TestTaskChecksAllowWhenTheBacklogHoldsNoSuchGroup(t *testing.T) {
	t.Parallel()
	group := groupRoot(t, "false")
	root := filepath.Join(filepath.Dir(group), "TG-99.9")
	if err := os.Rename(group, root); err != nil {
		t.Fatal(err)
	}
	got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "missing", "cwd": root})
	if got.code != 0 || !strings.Contains(got.stderr, "no group or task TG-99.9") {
		t.Fatalf("a worktree its backlog does not name was refused: %+v", got)
	}
}

func TestTaskChecksRunACheckTwoTasksShareOnce(t *testing.T) {
	t.Parallel()
	root := groupRoot(t, "echo once >> ran.log")
	path := filepath.Join(root, "docs", "backlog", "TG-90.1-a-group.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second := "\n- [ ] **TSK-90.1.2** Another task\n  - files: `b.go`\n  - done_when: `echo once >> ran.log`\n"
	if err := os.WriteFile(path, append(data, second...), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "shared", "cwd": root}); got.code != 0 {
		t.Fatalf("a passing group was refused: %+v", got)
	}
	ran, err := os.ReadFile(filepath.Join(root, "ran.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(ran) != "once\n" {
		t.Fatalf("the shared check ran %q, want once", ran)
	}
}
