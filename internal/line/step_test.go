package line

import (
	"os"
	"path/filepath"
	"testing"

	"komodo/internal/ledger"
)

const stepBacklog = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

// stepRepo builds a repo with a backlog and one role, ready for the station walk.
func stepRepo(t *testing.T) string {
	t.Helper()
	root := repo(t, stepBacklog)
	role := "---\nname: reviewer\ndescription: Reviews.\ntier: heavy\ntools: [read, search]\nsession: true\nreturns: reviewer.schema.json\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(root, RolesDir, "reviewer.md"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// startRun writes a run state for the group so step walks past intake.
func startRun(t *testing.T, root string) {
	t.Helper()
	state := RunState{Run: "TG-12.1-1", Group: "TG-12.1", Base: "main", Branch: "feat/a-group", Worktree: root}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
}

func TestStepStartsWithIntake(t *testing.T) {
	root := stepRepo(t)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo next --start TG-12.1 --base main" {
		t.Fatalf("action = %+v", next)
	}
}

func TestStepNamesTheBaseTheGroupDeclares(t *testing.T) {
	stacked := "### [TG-12.2] A stacked group\n```yaml\ntype: feat\nversion: 2.0.0\nbase: release/2.0\n```\n\n" +
		"#### [TSK-12.2.1] One [P: C] [READY]\n```yaml\nfiles: [b/one.go]\ndone_when: [\"go test ./b/...\"]\n```\n"
	next, err := Step(repo(t, stacked), "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo next --start TG-12.2 --base release/2.0" {
		t.Fatalf("command = %q; a stacked group must not be cut from the default branch", next.Command)
	}
}

func TestStepAsksForABriefThenASpawn(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo brief TSK-12.1.1" {
		t.Fatalf("action = %+v", next)
	}
	briefPath := filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md")
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(briefPath, []byte("brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err = Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "builder" || next.Task != "TSK-12.1.1" {
		t.Fatalf("action = %+v", next)
	}
}

func TestStepClosesATaskThatHasAResult(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo close TSK-12.1.1" {
		t.Fatalf("action = %+v", next)
	}
}

func TestStepMergesTheWaveOnceEveryTaskIsClosed(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	markDone(t, root, "TSK-12.1.1")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo close --wave 1" {
		t.Fatalf("action = %+v", next)
	}
}

func TestStepReviewsThenShipsThenIsDone(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	markDone(t, root, "TSK-12.1.1")
	book := Book(root)
	if err := book.Stamp(ledger.Entry{Run: "TG-12.1-1", Group: "TG-12.1", Station: "qc", Wave: 1, Outcome: "done"}); err != nil {
		t.Fatal(err)
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "reviewer" {
		t.Fatalf("action = %+v", next)
	}
	writeStepResult(t, root, "TG-12.1-review")
	next, _ = Step(root, "")
	if next.Command != "komodo close --group" {
		t.Fatalf("action = %+v", next)
	}
	if err := book.Stamp(ledger.Entry{Run: "TG-12.1-1", Group: "TG-12.1", Station: "ship", Outcome: "done"}); err != nil {
		t.Fatal(err)
	}
	next, _ = Step(root, "")
	if next.Action != "done" {
		t.Fatalf("action = %+v", next)
	}
}

func TestStepIsDoneWhenNothingIsReady(t *testing.T) {
	root := repo(t, "### [TG-12.1] G\n```yaml\ntype: feat\nversion: 2.0.0\n```\n")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "done" {
		t.Fatalf("action = %+v", next)
	}
}

func TestEveryActionNamesItsResolvedParts(t *testing.T) {
	root := stepRepo(t)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Skills == nil || next.Facets == nil || next.Commands == nil {
		t.Fatalf("an action left a resolved list unset: %+v", next)
	}
}

func TestAnOllamaMachineBecomesACommandNotASpawn(t *testing.T) {
	plan := &Plan{Roles: []Role{{Name: "reviewer", Tier: "heavy", Machine: "ollama"}}, Worktree: "."}
	got := action(t.TempDir(), plan, Action{Action: "spawn", Role: "reviewer", Task: "TSK-12.1.1"})
	if got.Action != "run" || got.Command != "komodo machine TSK-12.1.1" || got.Role != "" {
		t.Fatalf("action = %+v", got)
	}
}

// writeStepResult puts a valid result on disk for one task.
func writeStepResult(t *testing.T, root, taskID string) {
	t.Helper()
	path := ResultPath(root, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"result":"DONE"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// markDone flips a task's status token in the test repo's backlog.
func markDone(t *testing.T, root, taskID string) {
	t.Helper()
	path := filepath.Join(root, "BACKLOG.md")
	if err := writeStatus(path, taskID, "DONE"); err != nil {
		t.Fatal(err)
	}
}

func TestSpawnNamesTheWorktreeTheAgentWorksIn(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	path := filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("a brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(StateDir, "wt", "TSK-12.1.1")
	if next.Action != "spawn" || next.Worktree != want {
		t.Fatalf("worktree = %q, want %q (action %s)", next.Worktree, want, next.Action)
	}
}
