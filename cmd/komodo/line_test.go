package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/conductor"
	"komodo/internal/line"
)

// TestRunResumePrintsTheSavedState reads a group's saved state.json and
// reports it, without starting anything itself.
func TestRunResumePrintsTheSavedState(t *testing.T) {
	root := t.TempDir()
	state := conductor.State{Group: "TG-1", Current: conductor.Building, Sessions: []string{"builder-1"}, Repairs: 1}
	if err := conductor.SaveState(conductor.StatePath(root, "TG-1"), state); err != nil {
		t.Fatal(err)
	}

	got := captureStdout(t, func() { runResume(root, []string{"TG-1"}) })
	if !strings.Contains(got, "TG-1 is at Building with 1 session(s) and 1 repair round(s) recorded") {
		t.Fatalf("resume output = %q, want the saved state summary", got)
	}

	got = captureStdout(t, func() { runResume(root, []string{"TG-1", "--json"}) })
	if !strings.Contains(got, `"group":"TG-1"`) || !strings.Contains(got, `"state":"Building"`) {
		t.Fatalf("resume --json output = %q, want the state as JSON", got)
	}
}

// TestRunResumeFailsWithNoSavedState refuses a group whose run never saved a state.json.
func TestRunResumeFailsWithNoSavedState(t *testing.T) {
	root := t.TempDir()
	oldExit, code := exit, 0
	defer func() { exit = oldExit }()
	exit = func(c int) { code = c; panic("exit") }
	defer func() {
		if recover() == nil || code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	}()
	runResume(root, []string{"TG-1"})
}

// TestRunResumeUsage refuses a call with no group named.
func TestRunResumeUsage(t *testing.T) {
	root := t.TempDir()
	oldExit, code := exit, 0
	defer func() { exit = oldExit }()
	exit = func(c int) { code = c; panic("exit") }
	defer func() {
		if recover() == nil || code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	}()
	runResume(root, nil)
}

// TestRunRefusesToNestInsideASessionItStarted stops a headless session the run itself started
// from calling komodo run again, rather than let it contend for the lock and do nothing.
func TestRunRefusesToNestInsideASessionItStarted(t *testing.T) {
	root := t.TempDir()
	t.Setenv(line.LockEnv, "123")
	oldExit, code := exit, 0
	defer func() { exit = oldExit }()
	exit = func(c int) { code = c; panic("exit") }
	defer func() {
		if recover() == nil || code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	}()
	runRun(root, []string{"--dry-run"})
}

func TestAbandonBlocksAGroupFromTheCommandLine(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(blockedGroup), 0o644); err != nil {
		t.Fatal(err)
	}
	run := line.RunState{Run: "run-1", Group: "TG-1", Worktree: filepath.Join(root, "gone")}
	if err := line.SaveRun(root, run); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"abandon"}, 1, "usage: komodo abandon"},
		{[]string{"abandon", "TG-9"}, 1, "TG-9 has no run to abandon"},
		{[]string{"abandon", "TSK-1.1"}, 0, "TG-1 is abandoned"},
	}
	for _, tc := range cases {
		got := runCLI(t, root, "", tc.args...)
		if got.code != tc.code || !strings.Contains(got.stdout+got.stderr, tc.want) {
			t.Fatalf("%v = exit %d, %q %q; want exit %d naming %q", tc.args, got.code, got.stdout, got.stderr, tc.code, tc.want)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "BACKLOG.md"))
	if err != nil || !strings.Contains(string(data), "[P: C] [BLOCKED]") {
		t.Fatalf("backlog = %s, %v; want the task BLOCKED", data, err)
	}
}
