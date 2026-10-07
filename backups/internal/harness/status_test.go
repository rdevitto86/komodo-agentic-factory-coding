package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
)

// writeStepResult writes a minimal DONE result for a task, as a builder would.
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
	group := backlog.GroupFile{ID: "TG-12.1", Title: "Group", Priority: "H", Status: "READY", Type: "fix", Version: "1.0.0",
		Tasks: []backlog.GroupTask{{ID: "TSK-12.1.1", Title: "A task", Files: []string{"a.go"}}}}
	for _, tc := range []struct {
		status string
		ok     bool
	}{{"DONE", true}, {"BLOCKED", true}, {"IN_PROGRESS", false}, {"READY", false}} {
		t.Run(tc.status, func(t *testing.T) {
			root := t.TempDir()
			backlogtest.Seed(t, root, group)
			path := taskPath(root, "TSK-12.1.1")
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

// TestWriteStatusTicksAGroupFileTask proves a repo holding a backlog tree gets its tick written into
// the task's own file, leaving the group index file alone.
func TestWriteStatusTicksAGroupFileTask(t *testing.T) {
	root := t.TempDir()
	backlogtest.Seed(t, root, backlog.GroupFile{ID: "TG-20.1", Title: "Group", Priority: "H", Status: "READY", Type: "fix", Version: "1.0.0",
		Tasks: []backlog.GroupTask{{ID: "TSK-20.1.1", Title: "A task", Files: []string{"a.go"}}}})
	header, err := os.ReadFile(groupPath(root, "TG-20.1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeStatus(root, "TSK-20.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(taskPath(root, "TSK-20.1.1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] **TSK-20.1.1**") {
		t.Fatalf("task file was not ticked: %s", data)
	}
	if after, _ := os.ReadFile(groupPath(root, "TG-20.1")); string(after) != string(header) {
		t.Fatalf("TG.md changed:\n%s", after)
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
	data, _ := os.ReadFile(groupPath(root, "TG-12.1"))
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

// TestRecordStatusThenLoadRoundTripsACloseTheBacklogNeverSaw proves a run-recorded close reads back live.
func TestRecordStatusThenLoadRoundTripsACloseTheBacklogNeverSaw(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	writeStepResult(t, root, "TSK-12.1.1")
	if err := RecordStatus(root, "TSK-12.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	if got := LoadStatus(root)["TSK-12.1.1"]; got.Status != "DONE" {
		t.Fatalf("status = %+v; a task's recorded close must read back DONE", got)
	}
}
