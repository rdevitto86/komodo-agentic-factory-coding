package run

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/mount"
)

// escalateBacklog is a one-group backlog whose group declares its version and its tasks their proof.
const escalateBacklog = "# Backlog\n\n### [TG-40.1] Group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-40.1.1] Do the work [P: C] [READY]\n```yaml\nfiles: [change.txt]\ndone_when:\n  - go test ./...\n```\n"

// writeFile writes content to name under dir, creating its parent directories.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOrchestratorRequestFillsTheRoleWithTheEscalation(t *testing.T) {
	plan := &line.Plan{
		Group: "TG-40.1", Branch: "feat/tg-40-1",
		Tasks: []line.PlanTask{{ID: "TSK-40.1.1", Title: "Do the work", Files: []string{"change.txt"}}},
	}
	req, err := orchestratorRequest(t.TempDir(), plan, conductor.Escalation{
		Group: "TG-40.1", Left: conductor.Building, Reason: "which clock?",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TG-40.1 on `feat/tg-40-1` escalated at Building", "which clock?",
		"- TSK-40.1.1 Do the work: `change.txt`"} {
		if !strings.Contains(req.Brief, want) {
			t.Fatalf("brief = %q, want %q", req.Brief, want)
		}
	}
	if req.Role != "escalation" || strings.Contains(req.Brief, "{{") || !strings.Contains(string(req.Schema), "action") {
		t.Fatalf("request = %+v, want the escalation role, every slot filled and its schema", req)
	}
}

// TestOrchestratorRequestNeverStartsWithAnEmptyModel proves a profile with no escalation machine
// still starts the session on the role's own tier, where an empty model crashes the host.
func TestOrchestratorRequestNeverStartsWithAnEmptyModel(t *testing.T) {
	plan := &line.Plan{Group: "TG-40.1"}
	plan.Profile.Tiers = mount.Tiers{Standard: mount.Machine{Model: "standard-model"}, Heavy: mount.Machine{Model: "heavy-model"}}
	req, err := orchestratorRequest(t.TempDir(), plan, conductor.Escalation{Group: "TG-40.1"})
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "standard-model" {
		t.Fatalf("model = %q, want the escalation role's standard tier", req.Model)
	}
}

func TestOrchestratorRequestFailsWithNoOrchestratorRole(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, filepath.Join(line.RolesDir, "builder.md"), "---\nname: builder\n---\n")
	if _, err := orchestratorRequest(root, &line.Plan{}, conductor.Escalation{}); err == nil {
		t.Fatal("request = nil error, want the missing role named")
	}
}

func TestLintBacklogReportsTheWorktreesProblems(t *testing.T) {
	clean := t.TempDir()
	backlogtest.SeedText(t, clean, escalateBacklog)
	if problems, err := lintBacklog(clean); err != nil || len(problems) != 0 {
		t.Fatalf("lint = %v, %v; want a clean backlog to pass", problems, err)
	}
	broken := t.TempDir()
	backlogtest.SeedText(t, broken, strings.Replace(escalateBacklog, "done_when:\n  - go test ./...\n", "", 1))
	if problems, err := lintBacklog(broken); err != nil || len(problems) == 0 {
		t.Fatalf("lint = %v, %v; want a task with no done_when to fail", problems, err)
	}
	files := t.TempDir()
	backlogtest.SeedText(t, files, escalateBacklog)
	writeFile(t, files, "docs/backlog/epic-40/tg-40.1/TG.md", "not a group file\n")
	if problems, err := lintBacklog(files); err != nil || len(problems) == 0 {
		t.Fatalf("lint = %v, %v; want a malformed group file to fail", problems, err)
	}
}

func TestNewDriverWiresTheOrchestratorLintAndBlock(t *testing.T) {
	root := requestsRepo(t)
	// The fixture's own roles directory hides the embedded tree, so it gets the shipped escalation role.
	for _, name := range []string{"escalation.md", "escalation.schema.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", line.RolesDir, name))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, filepath.Join(line.RolesDir, name), string(data))
	}
	plan := requestsPlan()
	plan.Profile.Tiers.Heavy = mount.Machine{Provider: "anthropic", Model: "claude-opus-4-heavy"}
	driver, err := newDriver(root, plan, "run-1", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if driver.Block == nil {
		t.Fatal("newDriver left Block nil; a stopped group would never be written down")
	}
	req, err := driver.Orchestrator(conductor.Escalation{Group: "TG-20.1", Left: conductor.Reviewing, Reason: "stalled"})
	if err != nil || req.Role != "escalation" || !strings.Contains(req.Brief, "escalated at Reviewing") {
		t.Fatalf("orchestrator request = %+v, %v; want the escalation filled in", req, err)
	}
	if _, err := driver.Lint(); err != nil {
		t.Fatalf("lint = %v, want the worktree's backlog linted", err)
	}
	builder, ok := plan.Profile.Machine("builder")
	if !ok {
		t.Fatal("the plan's builder role has no machine; the fixture must name one")
	}
	if driver.Heavy.Model != plan.Profile.Tiers.Heavy.Model || driver.Heavy.Effort != builder.Effort {
		t.Fatalf("heavy = %+v, want the profile's heavy model at the builder machine's effort %q",
			driver.Heavy, builder.Effort)
	}
}

func TestDrainHoldsAGroupThatDependsOnAParkedOne(t *testing.T) {
	dependent := strings.Replace(driveDrainText, "version: 1.1.0\n", "version: 1.1.0\ndepends_on: [TG-07.1]\n", 1)
	root := driveDrainRepo(t, dependent)
	setupDrainDriveFakeClaude(t)
	t.Setenv("FAKE_BLOCK_GROUP", "TG-07.1")
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)})
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		t.Fatalf("code = 0; want the drain failing once TG-07.1 parked")
	}
	if !strings.Contains(out.String(), "TG-07.1 parked:") {
		t.Fatalf("output lacks TG-07.1 parking:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "TG-07.2 held: it depends on TG-07.1, which parked") {
		t.Fatalf("output lacks the held line:\n%s", out.String())
	}
}

func TestAnEditedBlockedGroupRestartsWithItsEditedText(t *testing.T) {
	worktree := t.TempDir()
	backlogtest.SeedText(t, worktree, escalateBacklog)
	statePath := filepath.Join(t.TempDir(), "state.json")
	saved := conductor.State{Group: "TG-40.1", Current: conductor.Blocked, Edited: true}
	if err := conductor.SaveState(statePath, saved); err != nil {
		t.Fatal(err)
	}
	driver := &conductor.Driver{}
	driver.Builder.Brief = "the brief"
	plan := &line.Plan{Group: "TG-40.1"}
	final, err := driveState(context.Background(), driver, statePath, plan, worktree)
	if err == nil {
		t.Fatal("drive = nil; an unwired driver refuses to run")
	}
	if final.Current != conductor.Ready || !final.SlotFree || final.Edited {
		t.Fatalf("state = %+v, want Ready with its slot taken and the edit consumed", final)
	}
	if !strings.HasPrefix(driver.Builder.Brief, "# The group, as a person edited it\n\n## [TG-40.1] Group") ||
		!strings.HasSuffix(driver.Builder.Brief, "the brief") {
		t.Fatalf("brief = %q, want the edited group ahead of the builder's brief", driver.Builder.Brief)
	}
	if _, ok := editedGroup(t.TempDir(), "TG-40.1"); ok {
		t.Fatal("edited group found in a worktree with no backlog")
	}
}
