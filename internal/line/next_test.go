package line

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
)

const groupText = "### [TG-05.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-05.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n\n" +
	"#### [TSK-05.1.2] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"go test ./b/...\"]\n```\n\n" +
	"#### [TSK-05.1.3] Three [P: C] [READY]\n```yaml\nfiles: [a/three.go]\ndone_when: [\"go test ./a/...\"]\ndepends_on: [TSK-05.1.1]\n```\n"

// repo writes a throwaway repo root holding a backlog and the role files.
func repo(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, RolesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	role := "---\nname: builder\ndescription: Writes code.\ntier: standard\ntools: [read, edit, write, shell, search]\nsession: true\nreturns: builder.schema.json\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(dir, "builder.md"), []byte(role), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWavesSplitByDirectoryAndDependency(t *testing.T) {
	parsed := backlog.Parse(groupText)
	waves, err := Waves(parsed.Groups[0].Tasks, nil)
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
	waves, err := Waves(parsed.Groups[0].Tasks, []string{"TSK-05.1.1"})
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
	if _, err := Topological(parsed.Groups[0].Tasks); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("err = %v", err)
	}
}

func TestBlockedByWalksTransitively(t *testing.T) {
	parsed := backlog.Parse(groupText)
	got := BlockedBy(parsed.Groups[0].Tasks, "TSK-05.1.1")
	if len(got) != 1 || got[0] != "TSK-05.1.3" {
		t.Fatalf("blocked = %v", got)
	}
}

func TestNextPrintsTheGroupAndItsWaves(t *testing.T) {
	root := repo(t, groupText)
	plan, err := Next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("no plan")
	}
	if plan.Group != "TG-05.1" || plan.Version != "2.0.0" || plan.Type != "feat" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Branch != "feat/a-group" {
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
	plan, err := Next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, role := range plan.Roles {
		if role.Name != "reviewer" {
			continue
		}
		found = true
		if role.Machine != "reviewer" {
			t.Fatalf("machine = %s, want the reviewer tier, not the heavy tier it declares", role.Machine)
		}
	}
	if !found {
		t.Fatal("no reviewer role in the plan")
	}
}

func TestNextTakesATaskIdOrAGroupId(t *testing.T) {
	root := repo(t, groupText)
	byTask, err := Next(root, "TSK-05.1.2")
	if err != nil {
		t.Fatal(err)
	}
	byGroup, err := Next(root, "TG-05.1")
	if err != nil {
		t.Fatal(err)
	}
	if byTask == nil || byGroup == nil || byTask.Group != byGroup.Group {
		t.Fatalf("by task = %v, by group = %v", byTask, byGroup)
	}
}

func TestNextPrintsNothingWhenNothingIsReady(t *testing.T) {
	root := repo(t, strings.ReplaceAll(groupText, "[READY]", "[DONE]"))
	plan, err := Next(root, "")
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
	plan, err := Next(root, "")
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
	text := strings.Replace(groupText, "type: feat\nversion: 2.0.0", "type: feat\nversion: 2.0.0\nmode: single", 1)
	root := repo(t, text)
	plan, err := Next(root, "")
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
	plan, err := Next(root, "")
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
	next, err := Next(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Group != "TG-05.2" {
		t.Fatalf("plan = %+v; the fixture must have a later group ready, which is what stole the wave", next)
	}
	plan, err := PlanForRun(root)
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

func TestPlanForStationKeepsAnUnshippedRun(t *testing.T) {
	root := repo(t, finishedText)
	state := RunState{
		Run: "TG-05.1-1", Group: "TG-05.1", Base: "main", Branch: "feat/a-group",
		Waves: [][]string{{"TSK-05.1.1"}, {"TSK-05.1.2"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanForStation(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.Group != "TG-05.1" {
		t.Fatalf("plan = %+v; the run's own group must not lose its station to a later ready group", plan)
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
