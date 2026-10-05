package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/conductor"
	"komodo/internal/line"
)

// statusRoot records three runs: one building, one escalated to the orchestrator, one no conductor has driven.
func statusRoot(t *testing.T) string {
	t.Helper()
	root := clockRoot(t)
	started := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	saved := map[string]*conductor.State{
		"TG-1.1": {Group: "TG-1.1", Current: conductor.Building, TimeUsed: 90 * time.Second},
		"TG-1.2": {Group: "TG-1.2", Current: conductor.Escalated, TimeUsed: 5 * time.Minute, Left: conductor.Reviewing},
		"TG-1.3": nil,
	}
	for index, group := range []string{"TG-1.1", "TG-1.2", "TG-1.3"} {
		run := line.RunState{Run: "run-1", Group: group, Started: started.Add(time.Duration(index) * time.Minute)}
		if err := line.SaveRun(root, run); err != nil {
			t.Fatal(err)
		}
		if s := saved[group]; s != nil {
			if err := conductor.SaveState(conductor.StatePath(root, group), *s); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestRunStatusShowsEachGroupsStateTimeAndBlocker(t *testing.T) {
	t.Parallel()
	got := RunStatus(statusRoot(t))
	want := []GroupStatus{
		{Group: "TG-1.1", State: "Building", TimeUsed: 90 * time.Second},
		{Group: "TG-1.2", State: "Escalated", TimeUsed: 5 * time.Minute},
		{Group: "TG-1.3", State: stateOpen},
	}
	if len(got) != len(want) {
		t.Fatalf("RunStatus = %+v, want %d groups", got, len(want))
	}
	for i := range want {
		if got[i].Group != want[i].Group || got[i].State != want[i].State || got[i].TimeUsed != want[i].TimeUsed {
			t.Fatalf("group %d = %+v, want %+v", i, got[i], want[i])
		}
		if blocked := got[i].Blocker != ""; blocked != (want[i].State == "Escalated") {
			t.Fatalf("group %s blocker = %q; only the escalated group carries one", got[i].Group, got[i].Blocker)
		}
	}
	text := StatusText(got)
	for _, line := range []string{"- TG-1.1: Building, 1m30s used", "- TG-1.3: open, 0s used", "Blocked, waiting on you:\n- TG-1.2: "} {
		if !strings.Contains(text, line) {
			t.Fatalf("status text holds no %q:\n%s", line, text)
		}
	}
}

func TestRunStatusCountsALiveStagesTimeSoFar(t *testing.T) {
	t.Parallel()
	root := statusRoot(t)
	blocked := conductor.State{Group: "TG-1.2", Current: conductor.Blocked, TimeUsed: 5 * time.Minute}
	if err := conductor.SaveState(conductor.StatePath(root, "TG-1.2"), blocked); err != nil {
		t.Fatal(err)
	}
	// The test runner's parent is a live process other than this one, so it holds the lock like a run.
	lock, err := json.Marshal(line.RunLock{PID: os.Getppid(), Run: "TG-1.1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"TG-1.1", "TG-1.2"} {
		if err := os.WriteFile(line.LockPath(root, group), lock, 0o644); err != nil {
			t.Fatal(err)
		}
		entered := time.Now().Add(-2 * time.Minute)
		if err := os.Chtimes(conductor.StatePath(root, group), entered, entered); err != nil {
			t.Fatal(err)
		}
	}
	got := RunStatus(root)
	if used := got[0].TimeUsed; used < 3*time.Minute+30*time.Second || used > 4*time.Minute {
		t.Fatalf("live Building group used %s, want its 1m30s plus about 2m in the current stage", used)
	}
	if used := got[1].TimeUsed; used != 5*time.Minute {
		t.Fatalf("blocked group used %s, want its saved 5m0s; a settled group adds no live time", used)
	}
}

// TestGroupStatusJSONNamesTimeUsedInSeconds proves status --json never serialises TimeUsed as
// raw nanoseconds under a key that names no unit: time_used_seconds carries whole seconds.
func TestGroupStatusJSONNamesTimeUsedInSeconds(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(GroupStatus{Group: "TG-1", State: "Building", TimeUsed: 90 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"time_used_seconds":90`) {
		t.Fatalf("json = %s, want time_used_seconds in whole seconds", data)
	}
	if strings.Contains(string(data), `"time_used"`) {
		t.Fatalf("json = %s, must not carry the old unitless key", data)
	}
}

func TestTheStatusHookInformsOnlyWhenARunIsRecorded(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		root    string
		verdict Verdict
	}{
		{"no run", clockRoot(t), Allow},
		{"a run", statusRoot(t), Inform},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := addStatus(context.Background(), Input{Root: tc.root})
			if err != nil || out.Verdict != tc.verdict {
				t.Fatalf("addStatus = %+v, %v; want %s", out, err, tc.verdict)
			}
			if (out.Message != "") != (tc.verdict == Inform) {
				t.Fatalf("message %q; only an inform carries one", out.Message)
			}
		})
	}
}

// staleMachine stamps a toolkit build under a fresh checkout, makes the installed binary report commit, and
// registers one host check that names files; every swap is undone when the test ends.
func staleMachine(t *testing.T, commit string, files ...string) string {
	t.Helper()
	checkout := clockRoot(t)
	if err := os.MkdirAll(filepath.Join(checkout, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "bin", ".built-from"), []byte("0123456789abcdef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := binaryBuild
	binaryBuild = func(string) (string, string) { return "1.0.0-beta.6", commit }
	RegisterStaleCheck("stale-test", func(string) []string { return files })
	t.Cleanup(func() {
		binaryBuild = previous
		staleLock.Lock()
		delete(staleChecks, "stale-test")
		staleLock.Unlock()
	})
	return checkout
}

func TestStaleNamesTheBinaryAndEachGlobalFileOnOneLine(t *testing.T) {
	cases := []struct {
		name   string
		commit string
		files  []string
		want   string
	}{
		{"fresh", "0123456789ab", nil, ""},
		{"a stale global skill", "0123456789ab", []string{"skills/run/SKILL.md"},
			"stale: skills/run/SKILL.md; run komodo sync\n"},
		{"a binary from another commit", "fedcba987654", nil, "the installed binary"},
		{"a dev build", "unknown", []string{"AGENTS.md"}, "the installed binary"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := StaleText(Stale(staleMachine(t, tc.commit, tc.files...)))
			if tc.want == "" {
				if text != "" {
					t.Fatalf("text = %q, want nothing when fresh", text)
				}
				return
			}
			if !strings.Contains(text, tc.want) || strings.Count(text, "\n") != 1 || !strings.HasPrefix(text, "stale: ") ||
				!strings.HasSuffix(text, "; run komodo sync\n") {
				t.Fatalf("text = %q, want one stale line holding %q", text, tc.want)
			}
			for _, file := range tc.files {
				if !strings.Contains(text, file) {
					t.Fatalf("text = %q, want %s named", text, file)
				}
			}
		})
	}
}

func TestTheStatusHookInformsWhenTheMachineLayerIsStale(t *testing.T) {
	root := staleMachine(t, "0123456789ab", "skills/plan/SKILL.md")
	out, err := addStatus(context.Background(), Input{Root: root})
	if err != nil || out.Verdict != Inform || strings.Count(out.Message, "stale: ") != 1 ||
		!strings.Contains(out.Message, "skills/plan/SKILL.md") {
		t.Fatalf("addStatus = %+v, %v; want one stale line naming the skill", out, err)
	}
}

func TestTheStatusHookIsMountedOnlyInThePrimarySession(t *testing.T) {
	hook, ok := Lookup("status")
	if !ok || hook.Event != SessionStart || len(hook.Sessions) != 1 || hook.Sessions[0] != SessionPrimary {
		t.Fatalf("status hook = %+v, %v; want a SessionStart row for the primary session", hook, ok)
	}
	for _, session := range []Session{SessionBuilder, SessionLens} {
		for _, mounted := range ForSession(session) {
			if mounted.Name == "status" {
				t.Fatalf("the %s session mounts the status hook", session)
			}
		}
	}
}
