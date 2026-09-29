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

// TestLintProblemsFallsBackToGroupFilesWithNoBacklogMd proves the gate's lint check never needs a flat backlog file.
func TestLintProblemsFallsBackToGroupFilesWithNoBacklogMd(t *testing.T) {
	root := t.TempDir()
	writeGroupFile(t, root, "TG-01.1-first.md",
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

// TestLintProblemsReportsAGroupFileWithAMalformedHeading proves a bad heading never vanishes silently.
func TestLintProblemsReportsAGroupFileWithAMalformedHeading(t *testing.T) {
	root := t.TempDir()
	writeGroupFile(t, root, "TG-01.1-broken.md",
		"## [tg-01.1] Lowercase id [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\n```\n")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	if len(problems) == 0 {
		t.Fatal("want a problem for the malformed heading")
	}
}

// TestLintProblemsReportsAGroupFileWithNoVersionAndAnOpenTaskWithNoFiles proves group-file lint
// checks what the legacy grammar's lint checks, not only the parse errors ParseGroupFile reports.
func TestLintProblemsReportsAGroupFileWithNoVersionAndAnOpenTaskWithNoFiles(t *testing.T) {
	root := t.TempDir()
	writeGroupFile(t, root, "TG-01.1-first.md",
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task with no files\n")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	if len(problems) != 2 {
		t.Fatalf("problems = %v, want a no-version and a no-files problem", problems)
	}
}

// TestLintProblemsReportsAnEpicVersionDisagreementAcrossGroupFiles proves group files in the same
// epic must agree on the version it ships, as the legacy grammar's epics and groups must.
func TestLintProblemsReportsAnEpicVersionDisagreementAcrossGroupFiles(t *testing.T) {
	root := t.TempDir()
	writeGroupFile(t, root, "TG-01.1-first.md",
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n")
	writeGroupFile(t, root, "TG-01.2-second.md",
		"## [TG-01.2] Second group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 2.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.2.1** A task\n  - files: `b.go`\n")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "EPIC-01") && strings.Contains(problem, "TG-01.2") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a problem naming the epic version disagreement; got %v", problems)
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

// TestAddPrefersGroupFilesOverALegacyBacklogMd proves a repo holding both writes into its group
// file, never the legacy BACKLOG.md, since group files are the current grammar.
func TestAddPrefersGroupFilesOverALegacyBacklogMd(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeGroupFile(t, root, "TG-08.1-existing.md",
		"## [TG-08.1] Existing [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-08\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-08.1.1** A task\n  - files: `a.go`\n")
	backlogPath := filepath.Join(root, "BACKLOG.md")
	if err := os.WriteFile(backlogPath, []byte("### [TG-08.1] Existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "add", "TG-08.1", "A", "second", "task", "--files", "b.go")
	if got.code != 0 {
		t.Fatalf("add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, groupFilesDir, "TG-08.1-existing.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "A second task") {
		t.Fatalf("group file gained no task: %s", data)
	}
	backlogData, err := os.ReadFile(backlogPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(backlogData), "A second task") {
		t.Fatalf("add wrote into the legacy BACKLOG.md: %s", backlogData)
	}
}

// TestMainDispatchesBacklogAndAdd proves `komodo backlog` and `komodo add` reach the group-file commands.
func TestMainDispatchesBacklogAndAdd(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	add := runCLI(t, root, "", "add", "TG-09.1", "A", "dispatched", "group")
	if add.code != 0 {
		t.Fatalf("add exited %d: %s%s", add.code, add.stdout, add.stderr)
	}
	if !strings.Contains(add.stdout, "TG-09.1") {
		t.Fatalf("add printed no group id: %s", add.stdout)
	}
	path := filepath.Join(root, groupFilesDir, "TG-09.1-a-dispatched-group.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("add wrote no group file: %v", err)
	}
	backlog := runCLI(t, root, "", "backlog")
	if backlog.code != 0 {
		t.Fatalf("backlog exited %d: %s%s", backlog.code, backlog.stdout, backlog.stderr)
	}
	if !strings.Contains(backlog.stdout, "TG-09.1") || !strings.Contains(backlog.stdout, "1 group(s)") {
		t.Fatalf("backlog through main = %s", backlog.stdout)
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

// TestLintProblemsRefusesAnOpenGroupAtATaggedVersion proves lint reads the repo's own tags.
func TestLintProblemsRefusesAnOpenGroupAtATaggedVersion(t *testing.T) {
	root := emptyRepo(t)
	writeGroupFile(t, root, "TG-01.1-first.md",
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n")
	runGit(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")
	if problems, _ := lintProblems(root); len(problems) != 0 {
		t.Fatalf("problems before any tag = %v, want none", problems)
	}
	runGit(t, root, "tag", "v1.0.0")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "TG-01.1") {
		t.Fatalf("problems = %v, want TG-01.1 named at the tagged 1.0.0", problems)
	}
}
