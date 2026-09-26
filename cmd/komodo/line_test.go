package main

import (
	"strings"
	"testing"

	"komodo/internal/conductor"
	"komodo/internal/line"
)

// TestRunResumePrintsTheSavedStateForKomodoRunToContinue reads a group's saved state.json and
// reports it, without starting anything itself.
func TestRunResumePrintsTheSavedStateForKomodoRunToContinue(t *testing.T) {
	root := t.TempDir()
	state := conductor.State{Group: "TG-1", Current: conductor.Building, Sessions: []string{"builder-1"}}
	if err := conductor.SaveState(conductor.StatePath(root, "TG-1"), state); err != nil {
		t.Fatal(err)
	}

	got := captureStdout(t, func() { runResume(root, []string{"TG-1"}) })
	if !strings.Contains(got, "TG-1 is at Building with 1 session(s) recorded") {
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
