package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestRunStatusPrintsTheRunOrSaysNoneIsRecorded prints each recorded group's state, as text and as JSON.
func TestRunStatusPrintsTheRunOrSaysNoneIsRecorded(t *testing.T) {
	root := t.TempDir()
	if got := captureStdout(t, func() { runStatus(root, nil) }); got != "no run is recorded\n" {
		t.Fatalf("status with no run = %q", got)
	}
	if err := line.SaveRun(root, line.RunState{Run: "run-1", Group: "TG-1"}); err != nil {
		t.Fatal(err)
	}
	state := conductor.State{Group: "TG-1", Current: conductor.Blocked}
	if err := conductor.SaveState(conductor.StatePath(root, "TG-1"), state); err != nil {
		t.Fatal(err)
	}
	if got := captureStdout(t, func() { runStatus(root, nil) }); !strings.Contains(got, "- TG-1: Blocked") ||
		!strings.Contains(got, "Blocked, waiting on you:") {
		t.Fatalf("status = %q, want the blocked group and its blocker", got)
	}
	if got := captureStdout(t, func() { runStatus(root, []string{"--json"}) }); !strings.Contains(got, `"state":"Blocked"`) {
		t.Fatalf("status --json = %q, want the group as JSON", got)
	}
}

// TestWatchStatusRedrawsInPlaceUntilCancelled draws once per tick after clearing the screen, and stops when cancelled.
func TestWatchStatusRedrawsInPlaceUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	draws := 0
	got := captureStdout(t, func() {
		watchStatus(ctx, time.Millisecond, func() {
			draws++
			if draws == 2 {
				cancel()
			}
		})
	})
	if draws != 2 || strings.Count(got, clearScreen) != 2 {
		t.Fatalf("drew %d times with %d clears, want 2 of each", draws, strings.Count(got, clearScreen))
	}
}

// TestRunStatusWatchRefusesAZeroOrNegativeInterval fails with a usage error instead of letting
// time.NewTicker panic on an interval that cannot tick.
func TestRunStatusWatchRefusesAZeroOrNegativeInterval(t *testing.T) {
	root := t.TempDir()
	for _, interval := range []string{"0", "-1s"} {
		oldExit, code := exit, 0
		exit = func(c int) { code = c; panic("exit") }
		func() {
			defer func() {
				exit = oldExit
				if recover() == nil || code != 1 {
					t.Fatalf("interval %s: exit code = %d, want 1", interval, code)
				}
			}()
			runStatus(root, []string{"--watch", "--interval", interval})
		}()
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
	if err := os.MkdirAll(filepath.Join(root, "docs", "backlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "backlog", "TG-1-a-group.md"), []byte(blockedGroup), 0o644); err != nil {
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
	data, err := os.ReadFile(filepath.Join(root, "docs", "backlog", "TG-1-a-group.md"))
	if err != nil || !strings.Contains(string(data), "[P: C] [BLOCKED]") {
		t.Fatalf("backlog = %s, %v; want the task BLOCKED", data, err)
	}
}
