package conductor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/line"
)

// stopBacklog is a group with one open task a blocker note can mark BLOCKED.
const stopBacklog = "## [TG-09.1] A group [P: C] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"- [ ] **TSK-09.1.1** Do it\n  - files: `a.go`\n"

// blocking wires the rig's driver to record each note Block publishes, failing with err.
func blocking(r *rig, err error) *[]backlog.BlockerNote {
	notes := &[]backlog.BlockerNote{}
	r.driver.Block = func(_ context.Context, note backlog.BlockerNote) error {
		*notes = append(*notes, note)
		return err
	}
	return notes
}

func TestAStopWritesABlockerNoteAndWaitsForAPerson(t *testing.T) {
	r := newRig(t)
	r.host.builds = []map[string]any{{"result": "BLOCKED", "tasks": []any{
		map[string]any{"task": "TSK-1.2", "result": "BLOCKED", "question": "which clock?"},
	}}}
	escalating(r, nil, map[string]any{"action": ActionStop, "needs": "a decision on the clock"})
	notes := blocking(r, nil)
	final, err := r.drive(t)
	if err != nil || final.Current != Blocked {
		t.Fatalf("drive = %s, %v; want Blocked", final.Current, err)
	}
	if len(*notes) != 1 {
		t.Fatalf("notes = %v, want one", *notes)
	}
	note := (*notes)[0]
	if note.State != "Building" || note.Run != "run-1" || note.Needs != "a decision on the clock" ||
		len(note.Items) != 1 || note.Items[0] != "TSK-1.2: which clock?" {
		t.Fatalf("note = %+v, want the state left, the run, the blocked task's question and what it needs", note)
	}
	if next := Next(final); next.Move != Blocked {
		t.Fatalf("next = %+v, want the group to wait for a person's edit", next)
	}
}

func TestAStalledGroupsNoteSaysItStoppedWithoutProgress(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail"}, {"fail"}, {"fail"}}
	escalating(r, nil, map[string]any{"action": ActionAnswer, "answer": "again"})
	notes := blocking(r, nil)
	if final, _ := r.drive(t); final.Current != Blocked {
		t.Fatalf("final = %s, want Blocked", final.Current)
	}
	if len(*notes) != 1 || !strings.Contains((*notes)[0].Needs, "without progress") {
		t.Fatalf("notes = %+v, want one saying the group stopped without progress", *notes)
	}
}

func TestLineBlockPublishesNothingOnceStoppedAndReturnsAFailedCommit(t *testing.T) {
	stations := &Line{Root: t.TempDir(), Plan: &line.Plan{Group: "TG-1", Worktree: t.TempDir()}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := stations.Block(ctx, backlog.BlockerNote{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("block = %v, want context.Canceled", err)
	}
	if err := stations.Block(context.Background(), backlog.BlockerNote{}); err == nil {
		t.Fatal("block = nil, want the WIP commit's failure in a worktree with no backlog")
	}
}

// TestLineBlockFailsWhenAScrubbedNoteNeverPublishes proves a scrubbed, unpublished blocker fails
// the group instead of looking published when no draft PR exists.
func TestLineBlockFailsWhenAScrubbedNoteNeverPublishes(t *testing.T) {
	worktree := t.TempDir()
	gitIn := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", worktree}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	gitIn("init", "-b", "main")
	gitIn("config", "user.email", "a@example.com")
	gitIn("config", "user.name", "a")
	writeIn(t, worktree, "docs/backlog/TG-09.1-a-group.md", stopBacklog)
	if err := os.WriteFile(filepath.Join(worktree, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn("add", "-A")
	gitIn("commit", "-m", "seed")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_CONFIG_KEY_0", "credential.helper")
	t.Setenv("GIT_CONFIG_VALUE_0", "")
	stations := &Line{Root: t.TempDir(), Plan: &line.Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []line.PlanTask{{ID: "TSK-09.1.1", Files: []string{"a.go"}}},
	}}
	err := stations.Block(context.Background(), backlog.BlockerNote{
		At: time.Now(), Run: "run-1", State: "Building", Items: []string{"stop"}, Needs: "a decision",
	})
	if err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("block = %v, want a failure naming the missing credential", err)
	}
}

// TestAKilledRunKeepsAnAnsweredStopsNeeds proves a run killed right after a stop was answered
// still writes the orchestrator's needs once it resumes into Blocked, not the default.
func TestAKilledRunKeepsAnAnsweredStopsNeeds(t *testing.T) {
	r := newRig(t)
	notes := blocking(r, nil)
	start := State{
		Group: "TG-1", Current: Escalated, Escalate: true, Answered: true, Stop: true,
		Left: Building, Needs: "a decision on the clock",
	}
	final, err := r.driver.Drive(context.Background(), start)
	if err != nil || final.Current != Blocked {
		t.Fatalf("drive = %s, %v; want Blocked", final.Current, err)
	}
	if len(*notes) != 1 || (*notes)[0].Needs != "a decision on the clock" {
		t.Fatalf("notes = %+v, want the saved needs, not the default", *notes)
	}
}

func TestAFailedPublishIsReturnedAndTheGroupStaysBlocked(t *testing.T) {
	r := newRig(t)
	r.host.builds = []map[string]any{{"result": "BLOCKED"}}
	escalating(r, nil, map[string]any{"action": ActionStop})
	notes := blocking(r, errors.New("no forge credential"))
	final, err := r.drive(t)
	if err == nil || !strings.Contains(err.Error(), "no forge credential") || final.Current != Blocked {
		t.Fatalf("drive = %s, %v; want Blocked with the publish failure", final.Current, err)
	}
	if len(*notes) != 1 || (*notes)[0].Needs != needsDefault {
		t.Fatalf("notes = %+v, want one needing a person's decision", *notes)
	}
}
