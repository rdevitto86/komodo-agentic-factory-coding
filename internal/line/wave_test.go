package line

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
)

func TestAtOrAboveRanksSeverities(t *testing.T) {
	if !AtOrAbove("critical", "high") || !AtOrAbove("high", "high") {
		t.Fatal("critical and high must clear a high floor")
	}
	if AtOrAbove("medium", "high") || AtOrAbove("low", "medium") {
		t.Fatal("a lower severity must not clear the floor")
	}
	if AtOrAbove("nonsense", "low") {
		t.Fatal("an unknown severity must not clear the floor")
	}
}

func TestSplitFindingsSendsTheRestToTheBacklog(t *testing.T) {
	findings := []Finding{
		{Severity: "critical", Title: "one"}, {Severity: "high", Title: "two"},
		{Severity: "medium", Title: "three"}, {Severity: "low", Title: "four"},
	}
	repair, file := SplitFindings(findings, "high")
	if len(repair) != 2 || len(file) != 2 {
		t.Fatalf("repair = %d, file = %d", len(repair), len(file))
	}
	if repair[0].Title != "one" || file[0].Title != "three" {
		t.Fatalf("split = %+v %+v", repair, file)
	}
}

func TestVerifyCommandFollowsTheDiscoveryOrder(t *testing.T) {
	root := t.TempDir()
	if got := VerifyCommand(root, root); got != "" {
		t.Fatalf("an empty repo has no verify command, got %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := VerifyCommand(root, root); got != "go test ./..." {
		t.Fatalf("verify = %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("verify:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := VerifyCommand(root, root); got != "make verify" {
		t.Fatalf("the discovery order did not prefer the Makefile: %q", got)
	}
}

func TestRepoCommandsOverrideDiscovery(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"verify":"make check","compile":"go build ./..."}`
	if err := os.WriteFile(filepath.Join(dir, "commands.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := VerifyCommand(root, root); got != "make check" {
		t.Fatalf("verify = %q", got)
	}
	if got := CompileCommands(root, root); len(got) != 1 || got[0] != "go build ./..." {
		t.Fatalf("compile = %v", got)
	}
}

func TestCompileCommandsFollowTheManifests(t *testing.T) {
	root := t.TempDir()
	if got := CompileCommands(root, root); len(got) != 0 {
		t.Fatalf("an empty repo compiles nothing, got %v", got)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := CompileCommands(root, root); len(got) != 1 || !strings.Contains(got[0], "go build") {
		t.Fatalf("compile = %v", got)
	}
}

func TestRunCommandCapturesOutputAndExit(t *testing.T) {
	root := t.TempDir()
	ok := RunCommand(root, "echo hello")
	if !ok.OK() || !strings.Contains(ok.Output, "hello") {
		t.Fatalf("result = %+v", ok)
	}
	bad := RunCommand(root, "exit 3")
	if bad.OK() || bad.ExitCode != 3 {
		t.Fatalf("result = %+v", bad)
	}
}

func TestRunGateStopsAtTheFirstFailure(t *testing.T) {
	root := t.TempDir()
	results := RunGate(root, []string{"true", "exit 1", "true"})
	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	if _, failed := FirstFailure(results); !failed {
		t.Fatal("the failure was not reported")
	}
}

func TestReportBodyMarksWhatBlocked(t *testing.T) {
	plan := &Plan{Group: "TG-09.1", Title: "A group", Tasks: []PlanTask{{ID: "a", Title: "One"}, {ID: "b", Title: "Two"}}}
	result := &ShipResult{Done: []string{"a"}, Blocked: []string{"b"}}
	waves := []*WaveResult{{Wave: 1, Merged: []string{"a"}, Conflict: "b conflicts with a"}}
	body := ReportBody(plan, result, waves, BodyContext{})
	for _, want := range []string{"- **a** — One", "- **b** — Two", "## Validation", "conflict", "- **Unproven** b Two is blocked"} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q:\n%s", want, body)
		}
	}
}

func TestTaskBranchIsLowercase(t *testing.T) {
	if got := TaskBranch("TSK-03.2.5"); got != "task/tsk-03.2.5" {
		t.Fatalf("branch = %s", got)
	}
}

// TestCloseWaveSkipsTheMergeForASingleModeGroup proves QC merges no task branch for a
// single-mode group: neither task branch below exists, yet the wave still closes.
func TestCloseWaveSkipsTheMergeForASingleModeGroup(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "a/one.go", "package a\n", "seed")
	plan := &Plan{Group: "TG-13.1", Mode: "single", Waves: [][]string{{"TSK-13.1.1", "TSK-13.1.2"}}}
	result, err := CloseWave(root, plan, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Conflict != "" {
		t.Fatalf("result = %+v; a single-mode group commits on the group branch, with nothing to merge", result)
	}
	if len(result.Merged) != 2 || result.Merged[0] != "TSK-13.1.1" || result.Merged[1] != "TSK-13.1.2" {
		t.Fatalf("merged = %v", result.Merged)
	}
}

// TestFileFindingsAppendsIntoTheGroupsOwnFile proves a repo holding docs/backlog group files gets
// its findings filed into the group's own file.
func TestFileFindingsAppendsIntoTheGroupsOwnFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "backlog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "## [TG-09.1] A group [P: C] [READY]\n\n```yaml\ntype: feat\nversion: 2.0.0\nepic: EPIC-09\ndepends_on: []\n```\n\n" +
		"- [x] **TSK-09.1.1** One\n  - files: `a/x.go`\n"
	path := filepath.Join(dir, "TG-09.1-a-group.md")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := []Finding{
		{Severity: "low", Class: "simplify", File: "a/x.go", Line: 3, Title: "dead branch", Detail: "d", Fix: "f"},
	}
	added, err := FileFindings(root, "TG-09.1", findings)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 {
		t.Fatalf("added = %v", added)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text = string(data)
	if !strings.Contains(text, "dead branch") || !strings.Contains(text, "status: REFINEMENT") {
		t.Fatalf("filed task is wrong:\n%s", text)
	}
	if _, err := os.Stat(filepath.Join(root, backlog.LegacyName)); !os.IsNotExist(err) {
		t.Fatal("FileFindings must never create a legacy backlog file in a group-file repo")
	}
}

func TestFileFindingsIsANoOpWithoutFindings(t *testing.T) {
	added, err := FileFindings(t.TempDir(), "TG-09.1", nil)
	if err != nil || added != nil {
		t.Fatalf("added = %v, err = %v", added, err)
	}
}

func TestTheRunPinsItsWavesSoClosingATaskDoesNotRenumberThem(t *testing.T) {
	root := t.TempDir()
	state := RunState{
		Run: "TG-09.1-1", Group: "TG-09.1", Base: "main", Branch: "feat/a",
		Worktree: root, Waves: [][]string{{"TSK-09.1.1", "TSK-09.1.2"}, {"TSK-09.1.3"}},
	}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRun(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Waves) != 2 || loaded.Waves[0][0] != "TSK-09.1.1" {
		t.Fatalf("waves = %v; the run must keep the waves it cut", loaded.Waves)
	}
}

// review writes a group's review result holding the findings given.
func review(t *testing.T, root, groupID string, findings ...Finding) {
	t.Helper()
	path := ResultPath(root, groupID+"-review")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"findings": findings})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheReviewsFindingsReachTheLine(t *testing.T) {
	root := t.TempDir()
	if got := ReviewFindings(root, "TG-09.1"); got != nil {
		t.Fatalf("findings = %v; no review means no findings", got)
	}
	review(t, root, "TG-09.1",
		Finding{Severity: "high", Class: "bug", File: "a.go", Title: "One", Detail: "d"},
		Finding{Severity: "low", Class: "simplify", File: "b.go", Title: "Two", Detail: "d"})
	got := ReviewFindings(root, "TG-09.1")
	if len(got) != 2 {
		t.Fatalf("findings = %v; the ship must see what the reviewer returned", got)
	}
	blocking, minor := SplitFindings(got, "high")
	if len(blocking) != 1 || len(minor) != 1 {
		t.Fatalf("blocking = %v, minor = %v; the floor decides which stop a ship", blocking, minor)
	}
}

func TestACollisionIsASharedFileNotASharedDirectory(t *testing.T) {
	root := gitRepo(t)
	commitGroupFile(t, root, backlog.GroupFile{
		ID: "TG-21.2", Title: "A group", Priority: "C", Status: "READY", Type: "feat", Version: "1.0.0",
		Tasks: []backlog.GroupTask{
			{ID: "TSK-21.2.1", Title: "First", Files: []string{"web/a/x.ts"}, Checks: []string{"true"}},
			{ID: "TSK-21.2.2", Title: "Sibling", Files: []string{"web/a/y.ts"}, Checks: []string{"true"}},
			{ID: "TSK-21.2.3", Title: "Same", Files: []string{"web/a/x.ts"}, Checks: []string{"true"}},
		},
	})
	gitCmd(t, root, "update-ref", TipRef("feat/group"), "HEAD")
	gitCmd(t, root, "checkout", "-q", "-b", TaskBranch("TSK-21.2.1"))
	commit(t, root, "web/a/x.ts", "export const x = 1\n", "first")
	gitCmd(t, root, "update-ref", TipRef(TaskBranch("TSK-21.2.1")), "HEAD")
	gitCmd(t, root, "checkout", "-q", "main")
	gitCmd(t, root, "branch", "-D", TaskBranch("TSK-21.2.1"))
	state := RunState{Group: "TG-21.2", Branch: "feat/group", Waves: [][]string{{"TSK-21.2.1", "TSK-21.2.2", "TSK-21.2.3"}}}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	if err := RefuseCollision(root, "TSK-21.2.2"); err != nil {
		t.Fatalf("a sibling file in the same directory must brief: %v", err)
	}
	err := RefuseCollision(root, "TSK-21.2.3")
	if err == nil || !strings.Contains(err.Error(), "TSK-21.2.1") {
		t.Fatalf("err = %v; the same file on an unmerged branch must refuse", err)
	}
}
