package line

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/git"
	"komodo/internal/ledger"
	"komodo/internal/mount"
	"komodo/internal/profile"
)

const stepBacklog = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

// singleModeBacklog is a two-task group that shares one builder and one worktree.
const singleModeBacklog = "### [TG-13.1] A single-mode group\n```yaml\ntype: feat\nversion: 2.0.0\nmode: single\n```\n\n" +
	"#### [TSK-13.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - test -f a/one.go\n```\n\n" +
	"#### [TSK-13.1.2] Two [P: C] [READY]\n```yaml\nfiles: [a/two.go]\ndone_when:\n  - test -f a/two.go\n```\n"

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
	if next.Command != "komodo close TSK-12.1.1 --gate" {
		t.Fatalf("action = %+v; a task close must pass --gate so the local gate runs", next)
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
	if next.Command != "komodo close --wave 1 TG-12.1" {
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
	gitWorktreeWithReview(t, root, "TG-12.1")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "reviewer" || next.Task != "TG-12.1-review" {
		t.Fatalf("action = %+v", next)
	}
	writeStepResult(t, root, "TG-12.1-review")
	next, _ = Step(root, "")
	if next.Command != "komodo close --group TG-12.1" {
		t.Fatalf("action = %+v", next)
	}
	if err := writeShipHandoff(root, ShipHandoff{Group: "TG-12.1", Branch: "feat/a-group"}); err != nil {
		t.Fatal(err)
	}
	next, _ = Step(root, "")
	if next.Action != "done" || !strings.Contains(next.Why, "handed off") {
		t.Fatalf("action = %+v; a pending handoff must stop the loop, not ship again", next)
	}
	if err := os.Remove(HandoffPath(root, "TG-12.1")); err != nil {
		t.Fatal(err)
	}
	if err := book.Stamp(ledger.Entry{Run: "TG-12.1-1", Group: "TG-12.1", Station: "ship", Outcome: "done"}); err != nil {
		t.Fatal(err)
	}
	next, _ = Step(root, "")
	if next.Action != "done" {
		t.Fatalf("action = %+v", next)
	}
}

// TestTheReviewerSpawnCarriesAWrittenBriefPath checks the reviewer spawn's brief is a path to a
// filled brief on disk, not the bare command string a reviewer with no shell tool cannot run.
func TestTheReviewerSpawnCarriesAWrittenBriefPath(t *testing.T) {
	root := stepRepo(t)
	role := "---\nname: reviewer\ndescription: Reviews.\ntier: heavy\ntools: [read, search]\nsession: true\nreturns: reviewer.schema.json\n---\n\n" +
		"Review of group {{group_id}}: {{title}}\n\n{{tasks}}\n\n{{standards}}\n\n{{diff}}\n"
	if err := os.WriteFile(filepath.Join(root, RolesDir, "reviewer.md"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	markDone(t, root, "TSK-12.1.1")
	if err := Book(root).Stamp(ledger.Entry{Run: "TG-12.1-1", Group: "TG-12.1", Station: "qc", Wave: 1, Outcome: "done"}); err != nil {
		t.Fatal(err)
	}
	gitWorktreeWithReview(t, root, "TG-12.1")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Brief == "komodo diff" || next.Brief == "" {
		t.Fatalf("brief = %q, want a written brief path the reviewer's read-only tools can open", next.Brief)
	}
	text, err := os.ReadFile(filepath.Join(root, next.Brief))
	if err != nil {
		t.Fatalf("brief path %q does not exist: %v", next.Brief, err)
	}
	if !strings.Contains(string(text), "Review of group TG-12.1") || !strings.Contains(string(text), "Your result") {
		t.Fatalf("brief does not carry the filled review and its result instruction:\n%s", text)
	}
}

// gitWorktreeFor makes a group's own worktree path a git repo with one commit on main,
// mirroring what a run's own group worktree looks like once cut.
func gitWorktreeFor(t *testing.T, root, groupID string) string {
	t.Helper()
	worktree := filepath.Join(root, StateDir, "wt", groupID)
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = worktree
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	commit(t, worktree, "a/one.go", "package a\n", "seed")
	return worktree
}

// gitWorktreeWithReview is gitWorktreeFor, plus a branch that diverged past the profile's
// review-skip-lines cap, so step must spawn the reviewer instead of skipping it.
func gitWorktreeWithReview(t *testing.T, root, groupID string) string {
	t.Helper()
	worktree := gitWorktreeFor(t, root, groupID)
	cmd := exec.Command("git", "checkout", "-q", "-b", "feat/a-group")
	cmd.Dir = worktree
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v: %s", err, out)
	}
	commit(t, worktree, "a/one.go", "package a\n\n"+strings.Repeat("x\n", 45), "grow")
	return worktree
}

func TestASmallDiffSkipsTheReviewStation(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	markDone(t, root, "TSK-12.1.1")
	if err := Book(root).Stamp(ledger.Entry{Run: "TG-12.1-1", Group: "TG-12.1", Station: "qc", Wave: 1, Outcome: "done"}); err != nil {
		t.Fatal(err)
	}
	gitWorktreeFor(t, root, "TG-12.1")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo close --group TG-12.1" {
		t.Fatalf("action = %+v; a diff at or under the profile's review-skip-lines cap must reach ship without a reviewer spawn", next)
	}
}

func TestBlockingFindingsPointAtStepNotTheRefusedClose(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	markDone(t, root, "TSK-12.1.1")
	if err := Book(root).Stamp(ledger.Entry{Run: "TG-12.1-1", Group: "TG-12.1", Station: "qc", Wave: 1, Outcome: "done"}); err != nil {
		t.Fatal(err)
	}
	review := ResultPath(root, "TG-12.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(blockingReview), 0o644); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		Stamp(root, ledger.Entry{Group: "TG-12.1", Station: "fix", Outcome: "done"})
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "done" || !strings.Contains(next.Why, "komodo step") || strings.Contains(next.Why, "close --group") {
		t.Fatalf("why = %q; blocking findings must point at komodo step, not the refused komodo close --group", next.Why)
	}
}

// blockingReview is a review result with one finding at the default severity floor.
const blockingReview = `{"findings":[{"severity":"high","class":"bug","file":"a/one.go","line":1,` +
	`"title":"t","detail":"d","fix":"f"}]}`

// reviewedRun builds a run whose one wave is merged and whose group worktree holds an unreviewed diff.
func reviewedRun(t *testing.T) (string, string) {
	t.Helper()
	root := stepRepo(t)
	schema := `{"type":"object","required":["result"],"properties":{"result":{"type":"string","enum":["DONE","BLOCKED"]}}}`
	if err := os.WriteFile(filepath.Join(root, RolesDir, "builder.schema.json"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	markDone(t, root, "TSK-12.1.1")
	Stamp(root, ledger.Entry{Group: "TG-12.1", Wave: 1, Station: "qc", Outcome: "done"})
	return root, gitWorktreeWithReview(t, root, "TG-12.1")
}

// writeReview puts a reviewer result on disk for the group.
func writeReview(t *testing.T, root, body string) {
	t.Helper()
	path := ResultPath(root, "TG-12.1-review")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// walkFixRound drives one blocking review through its fix brief, builder spawn, and close --fix,
// and checks the next step spawns a fresh review.
func walkFixRound(t *testing.T, root, worktree string) {
	t.Helper()
	writeReview(t, root, blockingReview)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo brief --review TG-12.1" {
		t.Fatalf("action = %+v; a blocking review must ask for a fix brief", next)
	}
	plan, err := PlanForStation(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FixBrief(root, plan); err != nil {
		t.Fatal(err)
	}
	next, err = Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "builder" || next.Task != "TG-12.1-fix" ||
		next.Brief != filepath.Join(StateDir, "briefs", "TG-12.1-fix.md") || next.Worktree != plan.Worktree {
		t.Fatalf("action = %+v; a fresh fix brief must spawn the builder in the group worktree", next)
	}
	fixed := "package a\n\n" + strings.Repeat("var _ = 0\n", 45)
	if err := os.WriteFile(filepath.Join(worktree, "go.mod"), []byte("module example.com/w\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "a", "one.go"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	writeResult(t, root, "TG-12.1-fix", goodResult())
	next, err = Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo close --fix TG-12.1" {
		t.Fatalf("action = %+v; a fix result must be gated and committed", next)
	}
	outcome, err := CloseFix(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "DONE" {
		t.Fatalf("outcome = %+v", outcome)
	}
	next, err = Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "reviewer" {
		t.Fatalf("action = %+v; a committed fix must be reviewed again", next)
	}
}

func TestABlockingReviewIsRepairedThenReviewedAgain(t *testing.T) {
	root, worktree := reviewedRun(t)
	walkFixRound(t, root, worktree)
	log, err := git.Run(worktree, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "TG-12.1-fix") {
		t.Fatalf("last commit = %q; the fix round must commit on the group branch", log)
	}
	if rounds := FixRounds(root, "TG-12.1"); rounds != 1 {
		t.Fatalf("rounds = %d, want the ledger to hold one fix round", rounds)
	}
	writeReview(t, root, blockingReview)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo brief --review TG-12.1" {
		t.Fatalf("action = %+v; a second blocking review under the default limit gets a second round", next)
	}
}

func TestBlockingReviewsPastReviewRepairsStopForAPerson(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	overlay := filepath.Join(home, ".komodo", "config.json")
	if err := os.MkdirAll(filepath.Dir(overlay), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, []byte(`{"review_repairs":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	root, worktree := reviewedRun(t)
	walkFixRound(t, root, worktree)
	writeReview(t, root, blockingReview)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "done" || !strings.Contains(next.Why, "a/one.go:1 t") || !strings.Contains(next.Why, "1 fix round") {
		t.Fatalf("action = %+v; past review_repairs a blocking review stops and names its findings", next)
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

// noKomodoRepo writes only a backlog for the text given, so the toolkit falls back to the
// embedded komodo/ tree for its roles, skills, and facets, unlike stepRepo's own overrides.
func noKomodoRepo(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestASpawnNamesTheStandardsItsFilesPullIn(t *testing.T) {
	root := noKomodoRepo(t, stepBacklog)
	startRun(t, root)
	briefPath := filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md")
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(briefPath, []byte("brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, skill := range next.Skills {
		if skill == "go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("skills = %v, want the go standard for a/one.go", next.Skills)
	}
}

const stepBacklogWithFacet = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\nfacets: [github-actions]\n```\n"

func TestASpawnNamesTheFacetsItsTaskDeclares(t *testing.T) {
	root := noKomodoRepo(t, stepBacklogWithFacet)
	startRun(t, root)
	briefPath := filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md")
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(briefPath, []byte("brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range next.Facets {
		if name == "github-actions" {
			found = true
		}
	}
	if !found {
		t.Fatalf("facets = %v, want github-actions from the task's own facets field", next.Facets)
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
	gitWorktreeWithReview(t, root, "TG-12.1")
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

func TestAReadOnlySessionRoleReachesItsLocalMachine(t *testing.T) {
	plan := &Plan{
		Roles: []Role{{Name: "reviewer", Tier: "heavy", Machine: "ollama", Session: true, Tools: []string{"read", "search"}}},
		Profile: profile.Profile{Tiers: mount.Tiers{
			Standard: mount.Machine{Provider: "claude", Model: "haiku"},
			Heavy:    mount.Machine{Provider: "claude", Model: "opus"},
			Reviewer: mount.Machine{Provider: "ollama", Model: "llama3.2"},
		}},
		Worktree: ".",
	}
	got := actionForTier(t.TempDir(), plan, Action{Action: "spawn", Role: "reviewer", Task: "x"}, "")
	if got.Machine != "ollama" || got.Action != "run" || got.Command != "komodo machine --role reviewer x" {
		t.Fatalf("action = %+v; a read-only session role must reach its own local machine, not be refused it", got)
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

// TestSingleModeSpawnSharesTheGroupWorktree proves step names the group's own worktree for a
// single-mode task's spawn, the same one brief.go already picks, so the two never disagree.
func TestSingleModeSpawnSharesTheGroupWorktree(t *testing.T) {
	root := repo(t, singleModeBacklog)
	if err := SaveRun(root, RunState{Run: "TG-13.1-1", Group: "TG-13.1", Base: "main", Branch: "feat/a-single-mode-group", Worktree: root}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, StateDir, "briefs", "TSK-13.1.1.md")
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
	want := filepath.Join(StateDir, "wt", "TG-13.1")
	if next.Action != "spawn" || next.Worktree != want {
		t.Fatalf("worktree = %q, want the group's own worktree %q (action %s)", next.Worktree, want, next.Action)
	}
}

// TestSingleModeWalksBriefCloseAndCloseWave proves a two-task single-mode group builds every
// task in the group's own worktree on the group branch, then closes the wave with no merge.
func TestSingleModeWalksBriefCloseAndCloseWave(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "BACKLOG.md", singleModeBacklog, "seed")
	role := "---\nname: builder\ndescription: Writes code.\ntier: standard\ntools: [read, edit, write, shell, search]\n" +
		"session: true\nreturns: builder.schema.json\n---\n\nTask {{task_id}}: {{title}}\n\n{{task_block}}\n" +
		"{{repo_rules}}{{repo_context}}{{context}}{{files}}{{repo_profile}}{{standards}}{{done_when}}{{failure}}\n"
	commit(t, root, filepath.Join(RolesDir, "builder.md"), role, "role")
	schema := `{"type":"object","required":["result"],"properties":{"result":{"type":"string","enum":["DONE","BLOCKED"]}}}`
	commit(t, root, filepath.Join(RolesDir, "builder.schema.json"), schema, "schema")

	plan, err := PlanForStation(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Mode != "single" {
		t.Fatalf("plan = %+v", plan)
	}
	worktree := WorktreePath(root, plan.Worktree)
	if err := AddWorktree(root, plan.Branch, plan.Base, worktree); err != nil {
		t.Fatal(err)
	}
	if err := SaveRun(root, RunState{
		Run: plan.Group + "-1", Group: plan.Group, Base: plan.Base, Branch: plan.Branch,
		Worktree: worktree, Waves: plan.Waves,
	}); err != nil {
		t.Fatal(err)
	}

	// Each task briefs, spawns, and closes in turn, on the group branch, before the next briefs.
	files := map[string]string{"TSK-13.1.1": "a/one.go", "TSK-13.1.2": "a/two.go"}
	for _, taskID := range []string{"TSK-13.1.1", "TSK-13.1.2"} {
		next, err := Step(root, "")
		if err != nil {
			t.Fatal(err)
		}
		if next.Command != "komodo brief "+taskID {
			t.Fatalf("action = %+v, want a brief for %s", next, taskID)
		}
		brief, err := BuildBrief(root, worktree, taskID, "builder", "")
		if err != nil {
			t.Fatal(err)
		}
		if brief.Worktree != plan.Worktree {
			t.Fatalf("brief worktree = %q, want the group's own worktree %q", brief.Worktree, plan.Worktree)
		}
		if err := WriteBrief(root, brief, plan.Branch); err != nil {
			t.Fatal(err)
		}
		if _, err := git.Run(root, "rev-parse", "--verify", "refs/heads/"+TaskBranch(taskID)); err == nil {
			t.Fatalf("%s cut a task branch; single mode shares the group worktree with no split", taskID)
		}

		next, err = Step(root, "")
		if err != nil {
			t.Fatal(err)
		}
		if next.Action != "spawn" || next.Worktree != plan.Worktree {
			t.Fatalf("spawn = %+v, want the group's own worktree %q", next, plan.Worktree)
		}

		path := filepath.Join(worktree, files[taskID])
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeResult(t, root, taskID, map[string]any{"result": "DONE"})

		next, err = Step(root, "")
		if err != nil {
			t.Fatal(err)
		}
		if next.Command != "komodo close "+taskID+" --gate" {
			t.Fatalf("action = %+v, want a close for %s", next, taskID)
		}
		outcome, err := CloseTask(root, taskID, false)
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Status != "DONE" {
			t.Fatalf("outcome = %+v", outcome)
		}
	}

	log, err := git.Run(worktree, "log", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "TSK-13.1.1") || !strings.Contains(log, "TSK-13.1.2") {
		t.Fatalf("log = %q; every task must commit directly onto the group branch", log)
	}

	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo close --wave 1 TG-13.1" {
		t.Fatalf("action = %+v, want the wave close", next)
	}
	closed, err := PlanForStation(root, "")
	if err != nil {
		t.Fatal(err)
	}
	result, err := CloseWave(root, closed, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Conflict != "" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Merged) != 2 {
		t.Fatalf("merged = %v", result.Merged)
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

// TestARepairThatWroteItsResultClosesInsteadOfSpawningAgain drives step through a first
// failure, a repair brief, and a repair result, and checks the next action closes, not spawns.
func TestARepairThatWroteItsResultClosesInsteadOfSpawningAgain(t *testing.T) {
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
	writeStepResult(t, root, "TSK-12.1.1")
	next, err = Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo close TSK-12.1.1 --gate" {
		t.Fatalf("action = %+v; a repair that wrote its result must close, not spawn forever", next)
	}
}

// TestARepairGivesUpAtTheProfilesLimitAndTheRunContinues checks step stops repairing a task
// past the profile's limit but keeps walking the run, rather than ending it outright.
func TestARepairGivesUpAtTheProfilesLimitAndTheRunContinues(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	seed(t, root, "TSK-12.1.1")
	fail(t, root, "TSK-12.1.1", 5)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo close --wave 1 TG-12.1" {
		t.Fatalf("action = %+v; a blocked task must stop repairing without ending the whole run", next)
	}
}

// TestABlockedTaskSkipsItsDependentsAndReachesADraftShip drives step past a permanently
// failed task and its dependent, and checks the run still reaches the review station.
func TestABlockedTaskSkipsItsDependentsAndReachesADraftShip(t *testing.T) {
	text := "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-12.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
		"#### [TSK-12.1.2] Two [P: C] [READY]\n```yaml\nfiles: [a/two.go]\ndone_when: [\"go test ./a/...\"]\ndepends_on: [TSK-12.1.1]\n```\n"
	root := repo(t, text)
	role := "---\nname: reviewer\ndescription: Reviews.\ntier: heavy\ntools: [read, search]\nsession: true\nreturns: reviewer.schema.json\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(root, RolesDir, "reviewer.md"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	state := RunState{
		Run: "TG-12.1-1", Group: "TG-12.1", Base: "main", Branch: "feat/a-group",
		Worktree: root, Waves: [][]string{{"TSK-12.1.1"}, {"TSK-12.1.2"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	seed(t, root, "TSK-12.1.1")
	fail(t, root, "TSK-12.1.1", 5)
	Stamp(root, ledger.Entry{Group: "TG-12.1", Wave: 1, Station: "qc", Outcome: "done"})
	Stamp(root, ledger.Entry{Group: "TG-12.1", Wave: 2, Station: "qc", Outcome: "done"})
	gitWorktreeWithReview(t, root, "TG-12.1")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "reviewer" {
		t.Fatalf("action = %+v; a blocked task must skip its dependent, not end the whole run", next)
	}
}

// TestAPausedProfileWaitsRatherThanShipsUnbuiltWork registers a mount whose usage pauses the
// window, and checks step waits instead of walking a plan whose waves the pause emptied.
func TestAPausedProfileWaitsRatherThanShipsUnbuiltWork(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	marker := filepath.Join(root, ".fakehost-pause-marker")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	resetsAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	mount.Register(mount.Host{
		Name: "fakehost-pause",
		Installed: func(r string) bool {
			_, err := os.Stat(filepath.Join(r, ".fakehost-pause-marker"))
			return err == nil
		},
		Tiers: func(string, bool) mount.Tiers {
			return mount.Tiers{Standard: mount.Machine{Provider: "vendora", Model: "model-a"}}
		},
		Probe: func() (mount.Usage, bool) {
			return mount.Usage{Plan: "pro", FiveHour: 0.95, ResetsAt: resetsAt}, true
		},
	})
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "done" || next.Until != resetsAt.Format(time.RFC3339) {
		t.Fatalf("action = %+v; a paused profile must wait, not ship a plan whose waves it emptied", next)
	}
}

const twoGroups = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
	"### [TG-12.2] The next group\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
	"#### [TSK-12.2.1] Later [P: C] [READY]\n```yaml\nfiles: [c/later.go]\ndone_when: [\"go test ./c/...\"]\n```\n"

func TestALaterReadyGroupCannotStealAnUnshippedRun(t *testing.T) {
	root := repo(t, twoGroups)
	role := "---\nname: reviewer\ndescription: Reviews.\ntier: heavy\ntools: [read, search]\nsession: true\nreturns: reviewer.schema.json\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(root, RolesDir, "reviewer.md"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	state := RunState{
		Run: "TG-12.1-1", Group: "TG-12.1", Base: "main", Branch: "feat/a-group",
		Worktree: root, Waves: [][]string{{"TSK-12.1.1"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	seed(t, root, "TSK-12.1.1")
	Stamp(root, ledger.Entry{Group: "TG-12.1", Wave: 1, Station: "qc", Outcome: "done"})
	gitWorktreeWithReview(t, root, "TG-12.1")
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
	plan, err := PlanForStation(root, "")
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

// waveBacklog is a three-task group whose tasks share no directory, so one wave holds all three.
const waveBacklog = "### [TG-14.1] A wide group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-14.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\n```\n\n" +
	"#### [TSK-14.1.2] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\n```\n\n" +
	"#### [TSK-14.1.3] Three [P: C] [READY]\n```yaml\nfiles: [c/three.go]\n```\n"

// briefWave cuts a group with its waves pinned to one, and writes a brief for each named task.
func briefWave(t *testing.T, text, group string, wave []string, taskIDs ...string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root := repo(t, text)
	if err := SaveRun(root, RunState{Run: group + "-1", Group: group, Base: "main", Branch: "feat/wide", Worktree: root, Waves: [][]string{wave}}); err != nil {
		t.Fatal(err)
	}
	for _, taskID := range taskIDs {
		path := filepath.Join(root, StateDir, "briefs", taskID+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("a brief"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// spawnedTasks names the tasks a wave spawn carries, checking each is a whole builder spawn.
func spawnedTasks(t *testing.T, next *Action) []string {
	t.Helper()
	var tasks []string
	for _, spawn := range next.Spawns {
		want := filepath.Join(StateDir, "wt", spawn.Task)
		if spawn.Action != "spawn" || spawn.Role != "builder" || spawn.Machine == "" ||
			spawn.Brief != filepath.Join(StateDir, "briefs", spawn.Task+".md") || spawn.Worktree != want {
			t.Fatalf("spawn = %+v", spawn)
		}
		tasks = append(tasks, spawn.Task)
	}
	return tasks
}

func TestABriefedParallelWaveSpawnsEveryTaskAtOnce(t *testing.T) {
	root := briefWave(t, waveBacklog, "TG-14.1", wideWave, "TSK-14.1.1", "TSK-14.1.2", "TSK-14.1.3")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(spawnedTasks(t, next), " ")
	if next.Action != "spawn" || next.Wave != 1 || got != "TSK-14.1.1 TSK-14.1.2 TSK-14.1.3" {
		t.Fatalf("action = %+v, spawns %q", next, got)
	}
}

func TestAWaveWithOneResultSpawnsOnlyTheOtherTwo(t *testing.T) {
	root := briefWave(t, waveBacklog, "TG-14.1", wideWave, "TSK-14.1.1", "TSK-14.1.2", "TSK-14.1.3")
	writeStepResult(t, root, "TSK-14.1.2")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(spawnedTasks(t, next), " ")
	if next.Action != "spawn" || got != "TSK-14.1.1 TSK-14.1.3" {
		t.Fatalf("action = %+v, spawns %q", next, got)
	}
}

func TestAWaveWritesEveryBriefBeforeItSpawns(t *testing.T) {
	root := briefWave(t, waveBacklog, "TG-14.1", wideWave, "TSK-14.1.1", "TSK-14.1.3")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo brief TSK-14.1.2" || len(next.Spawns) != 0 {
		t.Fatalf("action = %+v", next)
	}
}

func TestALocalTaskInAWaveRunsAloneThenTheOthersSpawn(t *testing.T) {
	text := strings.Replace(waveBacklog, "files: [a/one.go]", "files: [a/one.go]\ntier: light", 1)
	root := briefWave(t, text, "TG-14.1", wideWave, "TSK-14.1.1", "TSK-14.1.2", "TSK-14.1.3")
	role := "---\nname: builder\ndescription: Writes code.\ntier: standard\ntools: [read, search]\nsession: true\nreturns: builder.schema.json\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(root, RolesDir, "builder.md"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fakehost-wave-local-marker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(mount.Host{
		Name: "fakehost-wave-local",
		Installed: func(r string) bool {
			_, err := os.Stat(filepath.Join(r, ".fakehost-wave-local-marker"))
			return err == nil
		},
		Tiers: func(string, bool) mount.Tiers {
			return mount.Tiers{
				Light:    mount.Machine{Provider: mount.LocalName, Model: "llama3.2"},
				Standard: mount.Machine{Provider: "vendora", Model: "model-a"},
			}
		},
	})
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "run" || next.Command != "komodo machine --role builder TSK-14.1.1" || len(next.Spawns) != 0 {
		t.Fatalf("action = %+v; a local task in a wave must run alone as one step", next)
	}
	writeStepResult(t, root, "TSK-14.1.1")
	next, err = Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(spawnedTasks(t, next), " ")
	if next.Action != "spawn" || got != "TSK-14.1.2 TSK-14.1.3" {
		t.Fatalf("action = %+v, spawns %q; the other two must spawn once the local task returns", next, got)
	}
}

func TestASingleModeGroupSpawnsOneTaskAtATime(t *testing.T) {
	root := briefWave(t, singleModeBacklog, "TG-13.1", []string{"TSK-13.1.1", "TSK-13.1.2"}, "TSK-13.1.1", "TSK-13.1.2")
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Task != "TSK-13.1.1" || len(next.Spawns) != 0 {
		t.Fatalf("action = %+v", next)
	}
}

// wideWave is waveBacklog's one wave, pinned so the plan's parallel cap cannot split it.
var wideWave = []string{"TSK-14.1.1", "TSK-14.1.2", "TSK-14.1.3"}

// twoGroupBacklog holds two groups on disjoint files and a third that overlaps the first.
const twoGroupBacklog = "### [TG-15.1] First\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-15.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-15.2] Second\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
	"#### [TSK-15.2.1] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-15.3] Third\n```yaml\ntype: feat\nversion: 2.2.0\n```\n\n" +
	"#### [TSK-15.3.1] Three [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n"

// twoOpenRuns cuts the first two groups side by side, each task briefed.
func twoOpenRuns(t *testing.T) string {
	t.Helper()
	root := repo(t, twoGroupBacklog)
	started := time.Now().UTC()
	for index, group := range []string{"TG-15.1", "TG-15.2"} {
		state := RunState{Run: group + "-1", Group: group, Base: "main", Branch: "feat/" + group, Worktree: root,
			Started: started.Add(time.Duration(index) * time.Second)}
		if err := SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}
	for _, taskID := range []string{"TSK-15.1.1", "TSK-15.2.1"} {
		path := filepath.Join(root, StateDir, "briefs", taskID+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("brief"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestTwoGroupsOnDisjointFilesEachStepTheirOwnSpawn(t *testing.T) {
	root := twoOpenRuns(t)
	if err := RefuseOpenRun(root, "TG-15.2"); err != nil {
		t.Fatalf("a group on disjoint files was refused: %v", err)
	}
	for group, task := range map[string]string{"TG-15.1": "TSK-15.1.1", "TG-15.2": "TSK-15.2.1"} {
		next, err := Step(root, group)
		if err != nil {
			t.Fatal(err)
		}
		if next.Action != "spawn" || next.Role != "builder" || next.Task != task {
			t.Fatalf("step %s = %+v, want its own %s spawn", group, next, task)
		}
	}
	if _, err := os.Stat(filepath.Join(root, StateDir, "runs", "TG-15.2", "run.json")); err != nil {
		t.Fatalf("each group keeps its own run record: %v", err)
	}
}

func TestAnOverlappingGroupIsRefusedWhileTheFirstIsOpen(t *testing.T) {
	root := twoOpenRuns(t)
	err := RefuseOpenRun(root, "TG-15.3")
	if err == nil || !strings.Contains(err.Error(), "TG-15.1 is open") {
		t.Fatalf("err = %v; a group sharing a/one.go with an open group must be refused", err)
	}
}

func TestBareStepWithTwoOpenRunsNamesBoth(t *testing.T) {
	root := twoOpenRuns(t)
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "done" || !strings.Contains(next.Why, "TG-15.1") || !strings.Contains(next.Why, "TG-15.2") ||
		!strings.Contains(next.Why, "komodo step <group>") {
		t.Fatalf("action = %+v; a bare step with two open runs must name both and ask for one", next)
	}
}

func TestAWaveMergedInOneGroupDoesNotMergeAnothers(t *testing.T) {
	root := twoOpenRuns(t)
	for _, taskID := range []string{"TSK-15.1.1", "TSK-15.2.1"} {
		writeStepResult(t, root, taskID)
		if err := RecordStatus(root, taskID, "DONE"); err != nil {
			t.Fatal(err)
		}
	}
	Stamp(root, ledger.Entry{Group: "TG-15.1", Wave: 1, Station: "qc", Outcome: "done"})
	next, err := Step(root, "TG-15.2")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo close --wave 1 TG-15.2" {
		t.Fatalf("action = %+v; another group's merged wave 1 must not count as this group's", next)
	}
}

func TestEachGroupKeepsItsOwnLiveStatusAndHandoff(t *testing.T) {
	root := twoOpenRuns(t)
	if err := RecordStatus(root, "TSK-15.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	if err := RecordStatus(root, "TSK-15.2.1", "BLOCKED"); err != nil {
		t.Fatal(err)
	}
	for group, want := range map[string]string{"TG-15.1": "TSK-15.1.1", "TG-15.2": "TSK-15.2.1"} {
		statuses := loadStatusFile(filepath.Join(root, StateDir, "runs", group, "status.json"))
		if len(statuses) != 1 || statuses[want].Status == "" {
			t.Fatalf("%s status = %+v; a group's live status must hold only its own tasks", group, statuses)
		}
	}
	if err := keepGroupStatus(root, "TG-15.1"); err != nil {
		t.Fatal(err)
	}
	if got := LoadStatus(root); got["TSK-15.2.1"].Status != "BLOCKED" || got["TSK-15.1.1"].Status != "DONE" {
		t.Fatalf("status = %+v; cutting one group must leave another open group's status alone", got)
	}
	if err := writeShipHandoff(root, ShipHandoff{Group: "TG-15.2", Branch: "feat/TG-15.2"}); err != nil {
		t.Fatal(err)
	}
	first, err := LoadSnapshot(root, "TG-15.1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Handoff {
		t.Fatal("another group's handoff must not stop this group")
	}
}

func TestALegacyRunRecordIsMovedUnderItsGroupOnce(t *testing.T) {
	root := t.TempDir()
	legacy := RunState{Run: "TG-16.1-1", Group: "TG-16.1", Base: "main", Branch: "feat/legacy"}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"run.json": data, "ship.json": []byte(`{"group":"TG-16.1"}`)} {
		if err := os.WriteFile(filepath.Join(root, StateDir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := LoadRun(root)
	if err != nil || got.Branch != "feat/legacy" {
		t.Fatalf("state = %+v, err = %v; a legacy record must still be read", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, StateDir, "run.json")); !os.IsNotExist(err) {
		t.Fatalf("stat = %v; the legacy record must be moved, not copied", err)
	}
	for _, name := range []string{"run.json", "ship.json"} {
		if _, err := os.Stat(filepath.Join(root, StateDir, "runs", "TG-16.1", name)); err != nil {
			t.Fatalf("%s was not moved under its group: %v", name, err)
		}
	}
}

// tieredRepo is a one-task run on a fake host with a light, standard, and heavy machine, its brief written.
func tieredRepo(t *testing.T, fields, overlay string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if overlay != "" {
		if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	text := "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-12.1.1] One [P: C] [READY]\n```yaml\n" + fields + "done_when: [\"go test ./a/...\"]\n```\n"
	root := repo(t, text)
	startRun(t, root)
	if err := os.WriteFile(filepath.Join(root, ".fakehost-tiers-marker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(mount.Host{
		Name: "fakehost-tiers",
		Installed: func(r string) bool {
			_, err := os.Stat(filepath.Join(r, ".fakehost-tiers-marker"))
			return err == nil
		},
		Tiers: func(string, bool) mount.Tiers {
			return mount.Tiers{
				Light:    mount.Machine{Provider: "vendora", Model: "haiku"},
				Standard: mount.Machine{Provider: "vendora", Model: "sonnet"},
				Heavy:    mount.Machine{Provider: "vendora", Model: "opus"},
			}
		},
	})
	path := filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// builderMachine steps once and returns the builder spawn's machine and why.
func builderMachine(t *testing.T, root string) (string, string) {
	t.Helper()
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Action != "spawn" || next.Role != "builder" {
		t.Fatalf("action = %+v, want a builder spawn", next)
	}
	return next.Machine, next.Why
}

func TestAOneFileTaskBuildsOnLightAndRepairsOnStandard(t *testing.T) {
	root := tieredRepo(t, "files: [a/one.go]\n", "")
	machine, why := builderMachine(t, root)
	if machine != "vendora/haiku" || !strings.Contains(why, "TSK-12.1.1 is one file, so it builds on light") {
		t.Fatalf("machine = %q, why = %q; a one-file task must build on light", machine, why)
	}
	writeStepResult(t, root, "TSK-12.1.1")
	fail(t, root, "TSK-12.1.1", 1)
	if err := os.WriteFile(filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md"), []byte("repair brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	machine, why = builderMachine(t, root)
	if machine != "vendora/sonnet" || !strings.Contains(why, "TSK-12.1.1 failed once on light, so its repair builds on standard") {
		t.Fatalf("machine = %q, why = %q; a repair must build on standard", machine, why)
	}
}

func TestASourceAndItsTestFileStillBuildOnLight(t *testing.T) {
	root := tieredRepo(t, "files: [a/one.go, a/one_test.go]\n", "")
	if machine, _ := builderMachine(t, root); machine != "vendora/haiku" {
		t.Fatalf("machine = %q; a source file and its own test file are one small task", machine)
	}
}

func TestATwoFileTaskBuildsOnStandard(t *testing.T) {
	root := tieredRepo(t, "files: [a/one.go, a/two.go]\n", "")
	machine, why := builderMachine(t, root)
	if machine != "vendora/sonnet" || strings.Contains(why, "light") {
		t.Fatalf("machine = %q, why = %q; a two-file task must build on standard", machine, why)
	}
}

func TestLightBuilderFalseKeepsEveryBuilderOnStandard(t *testing.T) {
	root := tieredRepo(t, "files: [a/one.go]\n", `{"light_builder": false}`)
	if machine, _ := builderMachine(t, root); machine != "vendora/sonnet" {
		t.Fatalf("machine = %q; light_builder false must keep the builder on standard", machine)
	}
}

func TestATaskTierHeavyStaysOnHeavy(t *testing.T) {
	root := tieredRepo(t, "files: [a/one.go]\ntier: heavy\n", "")
	if machine, _ := builderMachine(t, root); machine != "vendora/opus" {
		t.Fatalf("machine = %q; a task tier key must win over the light choice", machine)
	}
}

func TestSmallTaskCountsOneSourceFilePlusItsOwnTest(t *testing.T) {
	cases := map[string]bool{
		"a/one.go":                 true,
		"a/one.go a/one_test.go":   true,
		"a/one_test.go a/one.go":   true,
		"a/one.go a/two_test.go":   false,
		"a/one.go a/two.go":        false,
		"a/one.go a/one_test.go b": false,
	}
	for files, want := range cases {
		if got := smallTask(strings.Fields(files)); got != want {
			t.Errorf("smallTask(%s) = %v, want %v", files, got, want)
		}
	}
}
