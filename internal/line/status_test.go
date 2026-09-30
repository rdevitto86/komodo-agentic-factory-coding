package line

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog/backlogtest"
)

func TestRecordStatusKeepsEveryTaskAndLeavesNoTempFile(t *testing.T) {
	root := t.TempDir()
	if err := RecordStatus(root, "TSK-1.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	if err := RecordStatus(root, "TSK-1.1.2", "BLOCKED"); err != nil {
		t.Fatal(err)
	}
	got := LoadStatus(root)
	if got["TSK-1.1.1"].Status != "DONE" || got["TSK-1.1.2"].Status != "BLOCKED" {
		t.Fatalf("status = %+v", got)
	}
	entries, err := os.ReadDir(filepath.Join(root, StateDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "status.json" {
		t.Fatalf("state dir holds %v; the write must be one atomic rename", entries)
	}
	if err := ClearStatus(root, []string{"TSK-1.1.1"}); err != nil {
		t.Fatal(err)
	}
	if got := LoadStatus(root); len(got) != 1 || got["TSK-1.1.2"].Status != "BLOCKED" {
		t.Fatalf("status = %+v; a clear must drop only the named tasks", got)
	}
	if err := ClearStatus(root, []string{"TSK-1.1.2"}); err != nil {
		t.Fatal(err)
	}
	if err := ClearStatus(root, []string{"TSK-1.1.2"}); err != nil {
		t.Fatalf("a second clear must be a no-op: %v", err)
	}
	if len(LoadStatus(root)) != 0 {
		t.Fatal("a cleared run still holds status")
	}
	if _, err := os.Stat(filepath.Join(root, StateDir, "status.json")); !os.IsNotExist(err) {
		t.Fatalf("an empty status must leave no file: %v", err)
	}
}

func TestWriteStatusChangesOnlyTheTickOrTheBlocker(t *testing.T) {
	text := "## [TG-12.1] Group [P: H] [READY]\n\n```yaml\ntype: fix\nversion: 1.0.0\nepic: EPIC-12\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-12.1.1** A task\n  - files: `a.go`\n"
	for _, tc := range []struct {
		status string
		ok     bool
	}{{"DONE", true}, {"BLOCKED", true}, {"IN_PROGRESS", false}, {"READY", false}} {
		t.Run(tc.status, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "docs", "backlog")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "TG-12.1-group.md")
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			err := writeStatus(root, "TSK-12.1.1", tc.status)
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v; only DONE and BLOCKED may reach the backlog", err)
			}
			want := "- [ ]"
			if tc.ok && tc.status == "DONE" {
				want = "- [x]"
			} else if tc.ok {
				want = "status: " + tc.status
			}
			if data, _ := os.ReadFile(path); !strings.Contains(string(data), want) {
				t.Fatalf("group file =\n%s\nwant it to contain %q", data, want)
			}
		})
	}
}

// TestWriteStatusTicksAGroupFileTask proves a repo holding docs/backlog group files gets its tick
// or blocker written into the task's own group file.
func TestWriteStatusTicksAGroupFileTask(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "backlog")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "## [TG-20.1] Group [P: H] [READY]\n\n```yaml\ntype: fix\nversion: 1.0.0\nepic: EPIC-20\ndepends_on: []\n```\n\n" +
		"- [ ] **TSK-20.1.1** A task\n  - files: `a.go`\n"
	path := filepath.Join(dir, "TG-20.1-group.md")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeStatus(root, "TSK-20.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] **TSK-20.1.1**") {
		t.Fatalf("group file was not ticked: %s", data)
	}
}

func TestANewRunKeepsOnlyItsOwnGroupsLiveStatus(t *testing.T) {
	root := stepRepo(t)
	for _, taskID := range []string{"TSK-12.1.1", "TSK-99.1.1"} {
		if err := RecordStatus(root, taskID, "BLOCKED"); err != nil {
			t.Fatal(err)
		}
	}
	if err := keepGroupStatus(root, "TG-12.1"); err != nil {
		t.Fatal(err)
	}
	if got := LoadStatus(root); len(got) != 1 || got["TSK-12.1.1"].Status != "BLOCKED" {
		t.Fatalf("status = %+v; a run must drop every other group's live status", got)
	}
}

func TestKeepStatusDropsEveryOtherTask(t *testing.T) {
	root := t.TempDir()
	for _, taskID := range []string{"TSK-1.1.1", "TSK-2.1.1"} {
		if err := RecordStatus(root, taskID, "DONE"); err != nil {
			t.Fatal(err)
		}
	}
	if err := KeepStatus(root, []string{"TSK-2.1.1"}); err != nil {
		t.Fatal(err)
	}
	if got := LoadStatus(root); len(got) != 1 || got["TSK-2.1.1"].Status != "DONE" {
		t.Fatalf("status = %+v; only the kept task may remain", got)
	}
}

func TestLoadBacklogReadsTheRunsStatusBeforeTheFile(t *testing.T) {
	root := stepRepo(t)
	if err := RecordStatus(root, "TSK-12.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	parsed, _, err := LoadBacklog(root)
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := parsed.Task("TSK-12.1.1"); task.Status != "DONE" {
		t.Fatalf("status = %s; the run's live status must win", task.Status)
	}
	groupFile := filepath.Join(root, "docs", "backlog", "TG-12.1-a-group.md")
	data, _ := os.ReadFile(groupFile)
	if !strings.Contains(string(data), "[READY]") {
		t.Fatal("reading the overlay rewrote the group file")
	}
}

func TestLoadBacklogKeepsAShippedGroupClosedUntilItLands(t *testing.T) {
	root := stepRepo(t)
	worktree := filepath.Join(root, StateDir, "wt", "TG-12.1")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	shipped := strings.Replace(stepBacklog, "[READY]", "[DONE]", 1)
	backlogtest.SeedText(t, worktree, shipped)
	parsed, _, err := LoadBacklog(root)
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := parsed.Task("TSK-12.1.1"); task.Status != "READY" {
		t.Fatalf("status = %s; a worktree no recorded run names must not close root's tasks", task.Status)
	}
	if err := SaveRun(root, RunState{Run: "TG-12.1-1", Group: "TG-12.1", Base: "main", Branch: "feat/a-group", Worktree: worktree}); err != nil {
		t.Fatal(err)
	}
	parsed, _, err = LoadBacklog(root)
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := parsed.Task("TSK-12.1.1"); task.Status != "DONE" {
		t.Fatalf("status = %s; the ship commit's DONE must hold after status.json is cleared", task.Status)
	}
	if err := RecordStatus(root, "TSK-12.1.1", "IN_PROGRESS"); err != nil {
		t.Fatal(err)
	}
	parsed, _, err = LoadBacklog(root)
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := parsed.Task("TSK-12.1.1"); task.Status != "IN_PROGRESS" {
		t.Fatalf("status = %s; the run's live status reads first", task.Status)
	}
}

func TestStepReadsACloseFromTheRunNotTheBacklog(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	if err := RecordStatus(root, "TSK-12.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo close --wave 1 TG-12.1" {
		t.Fatalf("action = %+v; a task the run closed must not close again", next)
	}
}
