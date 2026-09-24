package line

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordStatusKeepsEveryTaskAndLeavesNoTempFile(t *testing.T) {
	root := t.TempDir()
	if err := RecordStatus(root, "TSK-1.1.1", "DONE", ""); err != nil {
		t.Fatal(err)
	}
	if err := RecordStatus(root, "TSK-1.1.2", "BLOCKED", "attempt 2: result: missing summary"); err != nil {
		t.Fatal(err)
	}
	got := LoadStatus(root)
	if got["TSK-1.1.1"].Status != "DONE" || got["TSK-1.1.2"].Status != "BLOCKED" || got["TSK-1.1.2"].Note == "" {
		t.Fatalf("status = %+v", got)
	}
	entries, err := os.ReadDir(filepath.Join(root, StateDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "status.json" {
		t.Fatalf("state dir holds %v; the write must be one atomic rename", entries)
	}
	if err := ClearStatus(root); err != nil {
		t.Fatal(err)
	}
	if err := ClearStatus(root); err != nil {
		t.Fatalf("a second clear must be a no-op: %v", err)
	}
	if len(LoadStatus(root)) != 0 {
		t.Fatal("a cleared run still holds status")
	}
}

func TestLoadBacklogReadsTheRunsStatusBeforeTheFile(t *testing.T) {
	root := stepRepo(t)
	if err := RecordStatus(root, "TSK-12.1.1", "DONE", ""); err != nil {
		t.Fatal(err)
	}
	parsed, _, err := LoadBacklog(root)
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := parsed.Task("TSK-12.1.1"); task.Status != "DONE" {
		t.Fatalf("status = %s; the run's live status must win", task.Status)
	}
	data, _ := os.ReadFile(filepath.Join(root, "BACKLOG.md"))
	if !strings.Contains(string(data), "[READY]") {
		t.Fatal("reading the overlay rewrote BACKLOG.md")
	}
}

func TestLoadBacklogKeepsAShippedGroupClosedUntilItLands(t *testing.T) {
	root := stepRepo(t)
	worktree := filepath.Join(root, StateDir, "wt", "TG-12.1")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	shipped := strings.Replace(stepBacklog, "[READY]", "[DONE]", 1)
	if err := os.WriteFile(filepath.Join(worktree, "BACKLOG.md"), []byte(shipped), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, _, err := LoadBacklog(root)
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := parsed.Task("TSK-12.1.1"); task.Status != "DONE" {
		t.Fatalf("status = %s; the ship commit's DONE must hold after status.json is cleared", task.Status)
	}
	if err := RecordStatus(root, "TSK-12.1.1", "IN_PROGRESS", "attempt 1: x"); err != nil {
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
	if err := RecordStatus(root, "TSK-12.1.1", "DONE", ""); err != nil {
		t.Fatal(err)
	}
	next, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if next.Command != "komodo close --wave 1" {
		t.Fatalf("action = %+v; a task the run closed must not close again", next)
	}
}
