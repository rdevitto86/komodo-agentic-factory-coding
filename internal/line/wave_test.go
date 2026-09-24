package line

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if got := VerifyCommand(root); got != "" {
		t.Fatalf("an empty repo has no verify command, got %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := VerifyCommand(root); got != "go test ./..." {
		t.Fatalf("verify = %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("verify:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := VerifyCommand(root); got != "make verify" {
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
	if got := VerifyCommand(root); got != "make check" {
		t.Fatalf("verify = %q", got)
	}
	if got := CompileCommands(root); len(got) != 1 || got[0] != "go build ./..." {
		t.Fatalf("compile = %v", got)
	}
}

func TestCompileCommandsFollowTheManifests(t *testing.T) {
	root := t.TempDir()
	if got := CompileCommands(root); len(got) != 0 {
		t.Fatalf("an empty repo compiles nothing, got %v", got)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := CompileCommands(root); len(got) != 1 || !strings.Contains(got[0], "go build") {
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

func TestChangelogLineCountsWhatShipped(t *testing.T) {
	plan := &Plan{Group: "TG-09.1", Title: "A group", Version: "2.0.0"}
	result := &ShipResult{Done: []string{"a", "b"}, Blocked: []string{"c"}}
	line := ChangelogLine(plan, result)
	if !strings.Contains(line, "TG-09.1") || !strings.Contains(line, "2 task(s)") || !strings.Contains(line, "blocked: c") {
		t.Fatalf("line = %q", line)
	}
	if ChangelogLine(&Plan{Group: "TG-09.1"}, result) != "" {
		t.Fatal("a group with no version writes no changelog line")
	}
}

func TestAppendChangelogLandsUnderTheVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "CHANGELOG.md")
	body := "# Changelog\n\n## 2.0.0 — 2026-09-21\n\n- **TG-03.1** The markdown\n\n## 1.3.0 — 2026-09-01\n\n- old\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendChangelog(path, "2.0.0", "- **TG-03.2** The conveyor"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, "- **TG-03.2** The conveyor") {
		t.Fatal("the line was not written")
	}
	if strings.Index(text, "TG-03.2") > strings.Index(text, "## 1.3.0") {
		t.Fatal("the line landed under the wrong version")
	}
}

func TestAppendChangelogReplacesTheGroupsLineBelowTheIntro(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "CHANGELOG.md")
	body := "# Changelog\n\n## 2.0.0 — unreleased\n\nThe intro paragraph.\n\n- **TG-03.8** The line (16 task(s))\n- **TG-03.7** The pass (27 task(s))\n\n## 1.3.0 — 2026-09-01\n\n- **TG-03.8** Kept\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendChangelog(path, "2.0.0", "- **TG-03.8** The line (18 task(s))"); err != nil {
		t.Fatal(err)
	}
	if err := AppendChangelog(path, "2.0.0", "- **TG-03.9** New"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	want := "# Changelog\n\n## 2.0.0 — unreleased\n\nThe intro paragraph.\n\n- **TG-03.9** New\n- **TG-03.8** The line (18 task(s))\n- **TG-03.7** The pass (27 task(s))\n\n## 1.3.0 — 2026-09-01\n\n- **TG-03.8** Kept\n"
	if string(data) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", data, want)
	}
}

func TestAppendChangelogOpensANewVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "CHANGELOG.md")
	body := "# Changelog\n\n## 1.3.0 — 2026-09-01\n\n- old\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendChangelog(path, "2.0.0", "- **TG-03.2** The conveyor"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, "## 2.0.0") || strings.Index(text, "## 2.0.0") > strings.Index(text, "## 1.3.0") {
		t.Fatalf("the new version did not open above the old one:\n%s", text)
	}
}

func TestReportBodyMarksWhatBlocked(t *testing.T) {
	plan := &Plan{Group: "TG-09.1", Title: "A group", Tasks: []PlanTask{{ID: "a", Title: "One"}, {ID: "b", Title: "Two"}}}
	result := &ShipResult{Done: []string{"a"}, Blocked: []string{"b"}}
	waves := []*WaveResult{{Wave: 1, Merged: []string{"a"}, Conflict: "b conflicts with a"}}
	body := ReportBody(plan, result, waves)
	for _, want := range []string{"- [x] **a**", "- [ ] **b**", "## QC", "conflict", "## Blocked"} {
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

func TestFileFindingsAppendsTasksByClass(t *testing.T) {
	root := t.TempDir()
	body := "### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-09.1.1] One [P: C] [DONE]\n```yaml\nfiles: [a/x.go]\ndone_when: [\"go test\"]\n```\n"
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := []Finding{
		{Severity: "low", Class: "simplify", File: "a/x.go", Line: 3, Title: "dead branch", Detail: "d", Fix: "f"},
		{Severity: "medium", Class: "test-gap", File: "a/y.go", Line: 9, Title: "no test", Detail: "d", Fix: "f"},
	}
	added, err := FileFindings(root, "TG-09.1", findings)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 2 {
		t.Fatalf("added = %v", added)
	}
	data, _ := os.ReadFile(filepath.Join(root, "BACKLOG.md"))
	text := string(data)
	if !strings.Contains(text, "dead branch") || !strings.Contains(text, "type: refactor") || !strings.Contains(text, "type: test") {
		t.Fatalf("filed tasks are wrong:\n%s", text)
	}
	if !strings.Contains(text, "[REFINEMENT]") {
		t.Fatal("a filed finding must open in REFINEMENT")
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
	text := "### [TG-21.2] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-21.2.1] First [P: C] [READY]\n```yaml\nfiles: [internal/a/x.go]\ndone_when: [\"true\"]\n```\n\n" +
		"#### [TSK-21.2.2] Sibling [P: C] [READY]\n```yaml\nfiles: [internal/a/y.go]\ndone_when: [\"true\"]\n```\n\n" +
		"#### [TSK-21.2.3] Same [P: C] [READY]\n```yaml\nfiles: [internal/a/x.go]\ndone_when: [\"true\"]\n```\n"
	commit(t, root, "BACKLOG.md", text, "seed")
	gitCmd(t, root, "branch", "feat/group")
	gitCmd(t, root, "checkout", "-q", "-b", TaskBranch("TSK-21.2.1"))
	commit(t, root, "internal/a/x.go", "package a\n", "first")
	gitCmd(t, root, "checkout", "-q", "main")
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
