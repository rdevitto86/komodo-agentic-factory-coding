package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// writeGroupFile writes one group file under docs/backlog, creating the directory as needed.
func writeGroupFile(t *testing.T, root, name, text string) {
	t.Helper()
	dir := filepath.Join(root, groupFilesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunBacklogListsEveryOpenGroup proves backlog prints every group file, since a finished one is deleted.
func TestRunBacklogListsEveryOpenGroup(t *testing.T) {
	root := t.TempDir()
	writeGroupFile(t, root, "TG-01.1-first.md",
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n")
	writeGroupFile(t, root, "TG-02.1-second.md",
		"## [TG-02.1] Second group [P: M] [BLOCKED]\n\n```yaml\ntype: fix\nversion: 1.0.0\nepic: EPIC-02\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-02.1.1** Another task\n  - files: `b.go`\n")
	out := captureStdout(t, func() { runBacklog(root) })
	if !strings.Contains(out, "TG-01.1") || !strings.Contains(out, "First group") || !strings.Contains(out, "[READY]") {
		t.Fatalf("backlog printed no TG-01.1: %s", out)
	}
	if !strings.Contains(out, "TG-02.1") || !strings.Contains(out, "Second group") || !strings.Contains(out, "[BLOCKED]") {
		t.Fatalf("backlog printed no TG-02.1: %s", out)
	}
	if !strings.Contains(out, "2 group(s)") {
		t.Fatalf("backlog printed no count: %s", out)
	}
}

// TestRunBacklogOnAnEmptyRepoPrintsNoGroups proves backlog tolerates a repo with no docs/backlog yet.
func TestRunBacklogOnAnEmptyRepoPrintsNoGroups(t *testing.T) {
	root := t.TempDir()
	out := captureStdout(t, func() { runBacklog(root) })
	if strings.TrimSpace(out) != "0 group(s)" {
		t.Fatalf("backlog on an empty repo = %q", out)
	}
}

// TestRunBacklogAddWritesAGroupThenATask proves add creates a group file, then appends to it on the next call.
func TestRunBacklogAddWritesAGroupThenATask(t *testing.T) {
	root := t.TempDir()
	out := captureStdout(t, func() {
		runBacklogAdd(root, []string{"TG-08.1", "A", "new", "group"})
	})
	if !strings.Contains(out, "TG-08.1") {
		t.Fatalf("add printed no group id: %s", out)
	}
	path := filepath.Join(root, groupFilesDir, "TG-08.1-a-new-group.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("group file not written: %v", err)
	}
	if !strings.Contains(string(data), "## [TG-08.1] A new group [P: M] [REFINEMENT]") {
		t.Fatalf("group file = %s", data)
	}
	if !strings.Contains(string(data), "type: feat") {
		t.Fatalf("group file keeps no type: %s", data)
	}

	out = captureStdout(t, func() {
		runBacklogAdd(root, []string{"TG-08.1", "Its", "first", "task", "--files", "a.go,b.go"})
	})
	if !strings.Contains(out, "TSK-08.1.1") {
		t.Fatalf("add printed no task id: %s", out)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [ ] **TSK-08.1.1** Its first task") {
		t.Fatalf("group file gained no task: %s", data)
	}
	if !strings.Contains(string(data), "files: `a.go`, `b.go`") {
		t.Fatalf("task carries no files: %s", data)
	}

	backlogOut := captureStdout(t, func() { runBacklog(root) })
	if !strings.Contains(backlogOut, "TG-08.1") || !strings.Contains(backlogOut, "1 group(s)") {
		t.Fatalf("backlog after add = %s", backlogOut)
	}
}

// TestRunBacklogAddRejectsAMissingTitle proves add fails clearly when it gets fewer than a group and a title.
func TestRunBacklogAddRejectsAMissingTitle(t *testing.T) {
	root := t.TempDir()
	oldExit := exit
	defer func() { exit = oldExit }()
	var code int
	exit = func(c int) { code = c; panic("exit") }
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("want a panic from exit")
		}
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	}()
	runBacklogAdd(root, []string{"TG-08.1"})
}
