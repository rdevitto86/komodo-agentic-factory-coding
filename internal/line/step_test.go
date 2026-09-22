package line

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"komodo/internal/ledger"
	"komodo/internal/mount"
	"komodo/internal/profile"
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
	if next.Action != "spawn" || next.Role != "reviewer" || next.Task != "TG-12.1-review" {
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

func TestTheReviewerSpawnCarriesTheBeforeReviewCommand(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	markDone(t, root, "TSK-12.1.1")
	book := Book(root)
	if err := book.Stamp(ledger.Entry{Run: "TG-12.1-1", Group: "TG-12.1", Station: "qc", Wave: 1, Outcome: "done"}); err != nil {
		t.Fatal(err)
	}
	commandsDir := filepath.Join(root, StateDir, "wt", "TG-12.1", StateDir)
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"before_review":"make lint"}`
	if err := os.WriteFile(filepath.Join(commandsDir, "commands.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, command := range next.Commands {
		if command == "make lint" {
			found = true
		}
	}
	if !found {
		t.Fatalf("commands = %v, want before_review", next.Commands)
	}
}

const stepBacklogWithTaskTier = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\ntier: heavy\n```\n"

func TestATaskTierOverridesTheRolesOwnTier(t *testing.T) {
	root := repo(t, stepBacklogWithTaskTier)
	role := "---\nname: reviewer\ndescription: Reviews.\ntier: heavy\ntools: [read, search]\nsession: true\nreturns: reviewer.schema.json\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(root, RolesDir, "reviewer.md"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	startRun(t, root)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo brief TSK-12.1.1" {
		t.Fatalf("action = %+v", next)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md"), []byte("brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err = Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "builder" || next.Machine != "heavy" {
		t.Fatalf("action = %+v, want the task's own tier heavy", next)
	}
}

func TestAnOllamaMachineBecomesACommandNotASpawn(t *testing.T) {
	plan := &Plan{Roles: []Role{{Name: "reviewer", Tier: "heavy", Machine: "ollama"}}, Worktree: "."}
	got := action(t.TempDir(), plan, Action{Action: "spawn", Role: "reviewer", Task: "TSK-12.1.1"})
	if got.Action != "run" || got.Command != "komodo machine --role reviewer TSK-12.1.1" || got.Role != "" {
		t.Fatalf("action = %+v", got)
	}
}

func TestALightTaskTierKeepsAWritingRoleOnTheHostsStandardTier(t *testing.T) {
	plan := &Plan{
		Roles: []Role{{Name: "builder", Tier: "standard", Tools: []string{"read", "edit", "write", "shell", "search"}, Session: true}},
		Profile: profile.Profile{Tiers: mount.Tiers{
			Light:    mount.Machine{Provider: "ollama", Model: "llama3.2"},
			Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
		}},
		Worktree: ".",
	}
	got := actionForTier(t.TempDir(), plan, Action{Action: "spawn", Role: "builder", Task: "TSK-12.1.1"}, "light")
	if got.Action != "spawn" || got.Role != "builder" || got.Machine != "claude/sonnet" {
		t.Fatalf("action = %+v, want the spawn kept on the host's standard tier", got)
	}
}

func TestEveryTierLocalKeepsAWritingRoleAsASpawnOnTheHost(t *testing.T) {
	local := mount.Machine{Provider: "ollama", Model: "llama3.2"}
	plan := &Plan{
		Roles: []Role{{Name: "builder", Tier: "standard", Tools: []string{"read", "edit", "write", "shell", "search"}, Session: true, Machine: "ollama"}},
		Profile: profile.Profile{Tiers: mount.Tiers{
			Light: local, Standard: local, Heavy: local, Reviewer: local,
		}},
		Worktree: ".",
	}
	got := actionForTier(t.TempDir(), plan, Action{Action: "spawn", Role: "builder", Task: "TSK-12.1.1"}, "")
	if got.Action != "spawn" || got.Role != "builder" || got.Command != "" {
		t.Fatalf("action = %+v, want a spawn kept on the host, never a refused machine call", got)
	}
	if got.Machine == "" {
		t.Fatalf("action = %+v; a spawn must still name the machine that serves it", got)
	}
}

func TestBeforeReviewLooksUpTheAbsoluteWorktree(t *testing.T) {
	worktree := t.TempDir()
	commandsDir := filepath.Join(worktree, StateDir)
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"before_review":"make lint"}`
	if err := os.WriteFile(filepath.Join(commandsDir, "commands.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Roles: []Role{{Name: "reviewer", Tier: "heavy"}}, Worktree: worktree}
	got := actionForTier(t.TempDir(), plan, Action{Action: "spawn", Role: "reviewer", Task: "x"}, "")
	found := false
	for _, command := range got.Commands {
		if command == "make lint" {
			found = true
		}
	}
	if !found {
		t.Fatalf("commands = %v, want before_review from the absolute worktree", got.Commands)
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

// fail records a failed close for a task, which is what a repair reads.
func fail(t *testing.T, root, taskID string, count int) {
	t.Helper()
	if _, err := bumpAttempt(root, taskID, "result: missing required key \"summary\"", ""); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt < count; attempt++ {
		if _, err := bumpAttempt(root, taskID, "result: missing required key \"summary\"", ""); err != nil {
			t.Fatal(err)
		}
	}
}

// seed writes a brief and a result for a task, the state a close acts on.
func seed(t *testing.T, root, taskID string) {
	t.Helper()
	for _, rel := range []string{filepath.Join(StateDir, "briefs", taskID+".md"), ResultPath(root, taskID)} {
		path := rel
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, rel)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAFailedTaskGoesBackForARepairInsteadOfClosingAgain(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	seed(t, root, "TSK-12.1.1")
	fail(t, root, "TSK-12.1.1", 1)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo brief TSK-12.1.1" {
		t.Fatalf("action = %+v; a failed task must get a repair brief, not the same close again", next)
	}
}

func TestARepairSpawnsOnceItsBriefIsFresh(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	seed(t, root, "TSK-12.1.1")
	fail(t, root, "TSK-12.1.1", 1)
	path := filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md")
	if err := os.WriteFile(path, []byte("repair brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "builder" {
		t.Fatalf("action = %+v; a fresh repair brief must spawn the builder", next)
	}
}

func TestARepairGivesUpAtTheProfilesLimit(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	seed(t, root, "TSK-12.1.1")
	fail(t, root, "TSK-12.1.1", 5)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "done" {
		t.Fatalf("action = %+v; the line must stop, not repair forever", next)
	}
}

const twoGroups = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
	"### [TG-12.2] The next group\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
	"#### [TSK-12.2.1] Later [P: C] [READY]\n```yaml\nfiles: [c/later.go]\ndone_when: [\"go test ./c/...\"]\n```\n"

func TestALaterReadyGroupCannotStealAnUnshippedRun(t *testing.T) {
	root := repo(t, twoGroups)
	state := RunState{
		Run: "TG-12.1-1", Group: "TG-12.1", Base: "main", Branch: "feat/a-group",
		Worktree: root, Waves: [][]string{{"TSK-12.1.1"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	seed(t, root, "TSK-12.1.1")
	Stamp(root, ledger.Entry{Group: "TG-12.1", Wave: 1, Station: "qc", Outcome: "done"})
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "reviewer" {
		t.Fatalf("action = %+v; an unshipped run keeps its review and ship stations", next)
	}
}

func TestStepMovesOnOnceTheRunHasShipped(t *testing.T) {
	root := repo(t, twoGroups)
	state := RunState{
		Run: "TG-12.1-1", Group: "TG-12.1", Base: "main", Branch: "feat/a-group",
		Worktree: root, Waves: [][]string{{"TSK-12.1.1"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	seed(t, root, "TSK-12.1.1")
	Stamp(root, ledger.Entry{Group: "TG-12.1", Station: "ship", Outcome: "done"})
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo next --start TG-12.2 --base main" {
		t.Fatalf("action = %+v; a shipped run releases the line to the next group", next)
	}
}

func TestARepairMakesItsReviewStale(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "a/one.go", "package a\n", "seed")
	root := repo(t, stepBacklog)
	state := RunState{
		Run: "TG-12.1-1", Group: "TG-12.1", Base: "main", Branch: "feat/a-group",
		Worktree: worktree, Waves: [][]string{{"TSK-12.1.1"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	seed(t, root, "TSK-12.1.1")
	Stamp(root, ledger.Entry{Group: "TG-12.1", Wave: 1, Station: "qc", Outcome: "done"})
	plan, err := PlanForRun(root)
	if err != nil {
		t.Fatal(err)
	}
	plan.Worktree = worktree

	review := ResultPath(root, "TG-12.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !reviewed(root, plan) {
		t.Fatal("a review written after the last commit is current")
	}

	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(review, old, old); err != nil {
		t.Fatal(err)
	}
	if reviewed(root, plan) {
		t.Fatal("a commit landing after the review makes it stale; a repair must be re-reviewed")
	}
}
