package hooks

import (
	"context"
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
