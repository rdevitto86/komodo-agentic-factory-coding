package line

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/ledger"
	"komodo/internal/mount"
	"komodo/internal/plan"
)

const groupText = "### [TG-05.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-05.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
	"#### [TSK-05.1.2] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"go test ./b/...\"]\n```\n\n" +
	"#### [TSK-05.1.3] Three [P: C] [READY]\n```yaml\nfiles: [a/three.go]\ndone_when: [\"go test ./a/...\"]\ndepends_on: [TSK-05.1.1]\n```\n"

// repo writes a throwaway repo root holding a backlog, seeded from legacy-grammar text as group
// files, and the role files; it is a real git repo, so a station that leases a branch finds one.
func repo(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	backlogtest.SeedText(t, root, text)
	writeBuilderRole(t, root)
	return root
}

// writeBuilderRole ships the one builder role a throwaway repo needs to plan and brief.
func writeBuilderRole(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, RolesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	role := "---\nname: builder\ndescription: Writes code.\ntier: standard\ntools: [read, edit, write, shell, search]\nsession: true\nreturns: builder.schema.json\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(dir, "builder.md"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWavesSplitByDirectoryAndDependency(t *testing.T) {
	parsed := backlog.Parse(groupText)
	waves, err := plan.Waves(parsed.Groups[0].Tasks, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 2 {
		t.Fatalf("waves = %d, want 2", len(waves))
	}
	if len(waves[0]) != 2 || waves[0][0].ID != "TSK-05.1.1" || waves[0][1].ID != "TSK-05.1.2" {
		t.Fatalf("first wave = %v", ids(waves[0]))
	}
	if len(waves[1]) != 1 || waves[1][0].ID != "TSK-05.1.3" {
		t.Fatalf("second wave = %v", ids(waves[1]))
	}
}

func TestWavesSkipWhatIsDone(t *testing.T) {
	parsed := backlog.Parse(groupText)
	waves, err := plan.Waves(parsed.Groups[0].Tasks, []string{"TSK-05.1.1"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, wave := range waves {
		for _, task := range wave {
			if task.ID == "TSK-05.1.1" {
				t.Fatal("a done task was scheduled again")
			}
		}
	}
}

func TestTopologicalRefusesACycle(t *testing.T) {
	text := "### [TG-06.1] Loop\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-06.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/x.go]\ndone_when: [\"go test\"]\ndepends_on: [TSK-06.1.2]\n```\n\n" +
		"#### [TSK-06.1.2] Two [P: C] [READY]\n```yaml\nfiles: [b/y.go]\ndone_when: [\"go test\"]\ndepends_on: [TSK-06.1.1]\n```\n"
	parsed := backlog.Parse(text)
	if _, err := plan.Topological(parsed.Groups[0].Tasks); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("err = %v", err)
	}
}

func TestBlockedByWalksTransitively(t *testing.T) {
	parsed := backlog.Parse(groupText)
	got := plan.BlockedBy(parsed.Groups[0].Tasks, "TSK-05.1.1")
	if len(got) != 1 || got[0] != "TSK-05.1.3" {
		t.Fatalf("blocked = %v", got)
	}
}

func TestNextPrintsTheGroupAndItsWaves(t *testing.T) {
	root := repo(t, groupText)
	plan, err := next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("no plan")
	}
	if plan.Group != "TG-05.1" || plan.Version != "2.0.0" || plan.Type != "feat" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Branch != "feat/TG-05.1-a-group" {
		t.Fatalf("branch = %s", plan.Branch)
	}
	if len(plan.Waves) != 2 || len(plan.Tasks) != 3 {
		t.Fatalf("waves = %v tasks = %d", plan.Waves, len(plan.Tasks))
	}
	if len(plan.Roles) != 1 || plan.Roles[0].Tier != "standard" || len(plan.Roles[0].Tools) != 5 {
		t.Fatalf("roles = %+v", plan.Roles)
	}
}

func TestReviewerRoleDispatchesToTheReviewerTier(t *testing.T) {
	root := repo(t, groupText)
	role := "---\nname: reviewer\ndescription: Reviews diffs.\ntier: heavy\ntools: [read, search]\nsession: true\nreturns: reviewer.schema.json\n---\n\nBody.\n"
	path := filepath.Join(root, RolesDir, "reviewer.md")
	if err := os.WriteFile(path, []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, ".fakehost-reviewer-marker")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(mount.Host{
		Name: "fakehost-reviewer-tier",
		Installed: func(r string) bool {
			_, err := os.Stat(filepath.Join(r, ".fakehost-reviewer-marker"))
			return err == nil
		},
		Tiers: func(string, bool) mount.Tiers {
			return mount.Tiers{
				Heavy:    mount.Machine{Provider: "claude", Model: "opus"},
				Reviewer: mount.Machine{Provider: "claude", Model: "sonnet"},
			}
		},
	})
	plan, err := next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, role := range plan.Roles {
		if role.Name != "reviewer" {
			continue
		}
		found = true
		if role.Machine != "claude/sonnet" {
			t.Fatalf("machine = %s, want Tiers.Reviewer resolved, not the heavy tier it declares", role.Machine)
		}
	}
	if !found {
		t.Fatal("no reviewer role in the plan")
	}
}

func TestATaskNeedleNarrowsThePlanToThatTask(t *testing.T) {
	root := repo(t, groupText)
	plan, err := next(root, "TSK-05.1.3")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("no plan")
	}
	if len(plan.Tasks) != 1 || plan.Tasks[0].ID != "TSK-05.1.3" {
		t.Fatalf("tasks = %v, want only the named task, not the whole group", plan.Tasks)
	}
	if len(plan.Waves) != 1 || len(plan.Waves[0]) != 1 || plan.Waves[0][0] != "TSK-05.1.3" {
		t.Fatalf("waves = %v; a dependency outside the planned set must count as met", plan.Waves)
	}
}

const blockedGroupText = "### [TG-05.3] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-05.3.1] One [P: C] [BLOCKED]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

func TestPlanForGroupNeverCountsABlockedTaskAsDone(t *testing.T) {
	root := repo(t, blockedGroupText)
	plan, err := planForGroup(root, "TG-05.3")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("no plan")
	}
	if contains(plan.Skipped, "TSK-05.3.1") {
		t.Fatal("a blocked task must not count as already done, only a DONE status may")
	}
}

func TestNextTakesATaskIdOrAGroupId(t *testing.T) {
	root := repo(t, groupText)
	byTask, err := next(root, "TSK-05.1.2")
	if err != nil {
		t.Fatal(err)
	}
	byGroup, err := next(root, "TG-05.1")
	if err != nil {
		t.Fatal(err)
	}
	if byTask == nil || byGroup == nil || byTask.Group != byGroup.Group {
		t.Fatalf("by task = %v, by group = %v", byTask, byGroup)
	}
}

func TestNextPrintsNothingWhenNothingIsReady(t *testing.T) {
	root := repo(t, strings.ReplaceAll(groupText, "[READY]", "[DONE]"))
	plan, err := next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("plan = %+v, want nothing", plan)
	}
}

func TestNextSkipsATaskWithAValidResult(t *testing.T) {
	root := repo(t, groupText)
	dir := filepath.Join(root, StateDir, "results")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"result":"DONE","summary":"done","changed":[],"verified":[]}`)
	if err := os.WriteFile(filepath.Join(dir, "TSK-05.1.1.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0] != "TSK-05.1.1" {
		t.Fatalf("skipped = %v", plan.Skipped)
	}
	for _, wave := range plan.Waves {
		if contains(wave, "TSK-05.1.1") {
			t.Fatal("a task with a result was scheduled again")
		}
	}
}

func TestHasResultRejectsBrokenJSON(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, StateDir, "results")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TSK-01.1.1.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasResult(root, "TSK-01.1.1") {
		t.Fatal("broken JSON must not count as a result")
	}
}

func TestSingleModeIsOneWave(t *testing.T) {
	// The group-file grammar cannot yet carry a group's mode, so this plans off the parsed text.
	text := strings.Replace(groupText, "type: feat\nversion: 2.0.0", "type: feat\nversion: 2.0.0\nmode: single", 1)
	root := repo(t, "")
	parsed := backlog.Parse(text)
	plan, err := buildPlan(root, parsed, parsed.Groups[0], false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Waves) != 1 || len(plan.Waves[0]) != 3 {
		t.Fatalf("waves = %v", plan.Waves)
	}
}

func TestRunStateRoundTrips(t *testing.T) {
	root := t.TempDir()
	want := RunState{Run: "TG-05.1-1", Group: "TG-05.1", Base: "main", Branch: "feat/a-group"}
	if err := SaveRun(root, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRun(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Base != want.Base || got.Branch != want.Branch || got.Group != want.Group {
		t.Fatalf("state = %+v", got)
	}
}

// ids reduces a wave to its task ids.
func ids(wave []backlog.Task) []string {
	var out []string
	for _, task := range wave {
		out = append(out, task.ID)
	}
	return out
}

func TestEveryStationSeesTheWavesTheRunPinned(t *testing.T) {
	root := repo(t, groupText)
	pinned := [][]string{{"TSK-05.1.1", "TSK-05.1.2", "TSK-05.1.3"}}
	state := RunState{Run: "TG-05.1-1", Group: "TG-05.1", Base: "main", Branch: "feat/a-group", Waves: pinned}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	plan, err := next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Waves) != 1 || len(plan.Waves[0]) != 3 {
		t.Fatalf("waves = %v; close and ship must see the run's own waves", plan.Waves)
	}
}

const finishedText = "### [TG-05.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-05.1.1] One [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
	"#### [TSK-05.1.2] Two [P: C] [DONE]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"go test ./b/...\"]\n```\n\n" +
	"### [TG-05.2] The next group\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
	"#### [TSK-05.2.1] Later [P: C] [READY]\n```yaml\nfiles: [c/later.go]\ndone_when: [\"go test ./c/...\"]\n```\n"

func TestTheRunsOwnGroupOutlivesItsLastClosedTask(t *testing.T) {
	root := repo(t, finishedText)
	state := RunState{
		Run: "TG-05.1-1", Group: "TG-05.1", Base: "main", Branch: "feat/a-group",
		Waves: [][]string{{"TSK-05.1.1"}, {"TSK-05.1.2"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	next, err := next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Group != "TG-05.2" {
		t.Fatalf("plan = %+v; the fixture must have a later group ready, which is what stole the wave", next)
	}
	plan, err := PlanForStation(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Group != "TG-05.1" {
		t.Fatalf("plan = %+v; close --wave and ship must stay on the run's own group", plan)
	}
	if len(plan.Waves) != 2 {
		t.Fatalf("waves = %v; the run's waves must survive so QC can still merge them", plan.Waves)
	}
}

func TestPlanForStationMovesOnOnceTheRunIsShipped(t *testing.T) {
	root := repo(t, finishedText)
	state := RunState{
		Run: "TG-05.1-1", Group: "TG-05.1", Base: "main", Branch: "feat/a-group",
		Waves: [][]string{{"TSK-05.1.1"}, {"TSK-05.1.2"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	Stamp(root, ledger.Entry{Group: "TG-05.1", Station: "ship", Outcome: "done"})
	plan, err := PlanForStation(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Group != "TG-05.2" {
		t.Fatalf("plan = %+v; a shipped run must release the line to the next ready group", plan)
	}
}

const stackedText = "### [TG-05.3] Stacked on a missing branch\n```yaml\ntype: feat\nversion: 2.0.0\nbase: feat/missing\n```\n\n" +
	"#### [TSK-05.3.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
	"### [TG-05.4] Off the default\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
	"#### [TSK-05.4.1] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"go test ./b/...\"]\n```\n\n" +
	"### [TG-05.5] Stacked on the one before\n```yaml\ntype: feat\nversion: 2.2.0\nbase: feat/TG-05.4-off-the-default\n```\n\n" +
	"#### [TSK-05.5.1] Three [P: C] [READY]\n```yaml\nfiles: [c/three.go]\ndone_when: [\"go test ./c/...\"]\n```\n"

// stackedRepo is a remoted repo whose backlog stacks groups on branches origin may not hold; the
// group-file grammar cannot yet carry a group's base, so its text is never written, only parsed.
func stackedRepo(t *testing.T) string {
	t.Helper()
	root, _ := remotedRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/main")
	return root
}

// planForParsed mirrors planFor's own pick-then-build, for a backlog already parsed in memory,
// since the group-file grammar this package's tests otherwise seed cannot yet carry a base.
func planForParsed(root string, parsed backlog.Backlog) (*Plan, error) {
	group, only, ok := pick(root, parsed, "")
	if !ok {
		return nil, nil
	}
	return buildPlan(root, parsed, group, false, only)
}

func TestNextSkipsAGroupWhoseBaseIsNotOnOrigin(t *testing.T) {
	root := stackedRepo(t)
	plan, err := planForParsed(root, backlog.Parse(stackedText))
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Group != "TG-05.4" {
		t.Fatalf("plan = %+v; a group stacked on a missing branch must wait, not be picked", plan)
	}
}

func TestNextPicksAStackedGroupOnceItsBaseReachesOrigin(t *testing.T) {
	root := stackedRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/missing")
	plan, err := planForParsed(root, backlog.Parse(stackedText))
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Group != "TG-05.3" || plan.Base != "feat/missing" {
		t.Fatalf("plan = %+v; the stacked group's base is on origin now", plan)
	}
}

func TestNextCutsFromTheDefaultOnceTheParentMergedAndItsBranchIsGone(t *testing.T) {
	root, _ := remotedRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/main")
	text := "### [TG-05.2] The parent\n```yaml\ntype: fix\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-05.2.1] Done [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
		"### [TG-05.3] The child\n```yaml\ntype: feat\nversion: 2.0.0\nbase: fix/TG-05.2-the-parent\ndepends_on: [TG-05.2]\n```\n\n" +
		"#### [TSK-05.3.1] Next [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"go test ./b/...\"]\n```\n"
	plan, err := planForParsed(root, backlog.Parse(text))
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Group != "TG-05.3" || plan.Base != "main" {
		t.Fatalf("plan = %+v; a group whose parent merged and lost its branch must cut from main", plan)
	}
}

func TestNextStillWaitsWhenTheParentIsOpen(t *testing.T) {
	root, _ := remotedRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/main")
	text := "### [TG-05.2] The parent\n```yaml\ntype: fix\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-05.2.1] Open [P: C] [REFINEMENT]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
		"### [TG-05.3] The child\n```yaml\ntype: feat\nversion: 2.0.0\nbase: fix/TG-05.2-the-parent\ndepends_on: [TG-05.2]\n```\n\n" +
		"#### [TSK-05.3.1] Next [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"go test ./b/...\"]\n```\n"
	plan, err := planForParsed(root, backlog.Parse(text))
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("plan = %+v; a group stacked on an open parent with no branch must wait", plan)
	}
}

func TestNextPlansOntoTheEpicBranchWhenNoBaseIsDeclared(t *testing.T) {
	root, _ := remotedRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/main")
	runGit(t, root, "push", "origin", "HEAD:refs/heads/feat/2.0.0")
	text := "### [TG-05.6] No base\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-05.6.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"
	backlogtest.SeedText(t, root, text)
	plan, err := next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Base != "feat/2.0.0" {
		t.Fatalf("plan = %+v; a group with no base must plan onto its epic branch", plan)
	}
}

func TestNextFallsBackToDefaultWhenTheEpicBranchCannotBeOpened(t *testing.T) {
	root, _ := remotedRepo(t)
	runGit(t, root, "push", "origin", "HEAD:refs/heads/main")
	text := "### [TG-05.6] No base, no epic branch\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-05.6.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"
	backlogtest.SeedText(t, root, text)
	plan, err := next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Base != "main" {
		t.Fatalf("plan = %+v; a missing epic branch must fall back to the remote's default", plan)
	}
}

func TestReadyGroupsCountsAnEarlierGroupsBranchAsABase(t *testing.T) {
	root := stackedRepo(t)
	groups := readyGroups(root, backlog.Parse(stackedText), true)
	var got []string
	for _, group := range groups {
		got = append(got, group.ID)
	}
	if strings.Join(got, ",") != "TG-05.4,TG-05.5" {
		t.Fatalf("ready = %v; want the default-based group, then the group stacked on it", got)
	}
}
