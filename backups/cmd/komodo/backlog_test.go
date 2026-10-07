package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
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

// seedGroup writes flat group text into the tree: epic index if missing, group index, one file per task.
func seedGroup(t *testing.T, root, text string) string {
	t.Helper()
	file := backlog.ParseGroupFile(text)
	if file.ID == "" {
		t.Fatalf("seedGroup: no group heading in %q", text)
	}
	epicID := file.EpicID
	if epicID == "" {
		epicID = backlog.EpicIDOfGroup(file.ID)
	}
	seedEpic(t, root, epicID, file.Version, file.Type)
	dir, err := backlog.WriteGroup(root, file)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// seedEpic writes an epic's the epic index file at version under root, keeping one that exists.
func seedEpic(t *testing.T, root, epicID, version, kind string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(backlog.EpicDir(root, epicID), backlog.EpicFileName)); err == nil {
		return
	}
	epic := backlog.EpicFile{ID: epicID, Title: "Epic " + epicID, Status: "READY", Version: version, Type: kind, GroupsMax: 99}
	if _, err := backlog.WriteEpic(root, epic); err != nil {
		t.Fatal(err)
	}
}

// seedRawGroupDir writes the group index file verbatim into docs/backlog/<epicDir>/<groupDir>, for a fixture lint must reject.
func seedRawGroupDir(t *testing.T, root, epicDir, groupDir, text string) {
	t.Helper()
	dir := filepath.Join(root, groupFilesDir, epicDir, groupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, backlog.GroupFileName), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeFlatGroupFile writes one flat group file directly under docs/backlog, the layout migrate moves into the tree.
func writeFlatGroupFile(t *testing.T, root, name, text string) {
	t.Helper()
	dir := filepath.Join(root, groupFilesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// groupText joins every file of the group folder holding groupID, the group index file first, as one text.
func groupText(t *testing.T, root, groupID string) string {
	t.Helper()
	group, found, err := backlog.Locate(root, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("no group %s under %s", groupID, root)
	}
	var out string
	for _, path := range group.Paths() {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out += string(data) + "\n"
	}
	return out
}

// TestRunBacklogListsEveryOpenGroup proves backlog prints every group file, since a finished one is deleted.
func TestRunBacklogListsEveryOpenGroup(t *testing.T) {
	root := t.TempDir()
	seedGroup(t, root,
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n")
	seedGroup(t, root,
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
	seedGroup(t, root,
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n  - done_when: `go test ./a/...`\n")
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
	seedEpic(t, root, "EPIC-01", "1.0.0", "feat")
	seedRawGroupDir(t, root, "epic-01", "tg-01.1",
		"## [tg-01.1] Lowercase id [P: H] [READY]\n\n```yaml\ntype: feat\n```\n")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	if len(problems) == 0 {
		t.Fatal("want a problem for the malformed heading")
	}
}

// TestLintProblemsReportsAGroupFileWithNoVersionAndAnOpenTaskWithNoFiles proves the tree's lint
// checks what the legacy grammar's lint checks, not only the parse errors ParseGroupFile reports.
func TestLintProblemsReportsAGroupFileWithNoVersionAndAnOpenTaskWithNoFiles(t *testing.T) {
	root := t.TempDir()
	seedGroup(t, root,
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task with no files\n  - done_when: `go test ./...`\n")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	noVersion, noFiles := false, false
	for _, problem := range problems {
		noVersion = noVersion || strings.Contains(problem, "EPIC-01: no version")
		noFiles = noFiles || strings.Contains(problem, "TSK-01.1.1") && strings.Contains(problem, "declares no files")
	}
	if !noVersion || !noFiles {
		t.Fatalf("problems = %v, want the epic's no-version and the task's no-files problem", problems)
	}
}

// TestLintProblemsReportsAGroupDeclaringItsOwnVersion proves a group index carrying a version is a problem;
// only the epic index declares one.
func TestLintProblemsReportsAGroupDeclaringItsOwnVersion(t *testing.T) {
	root := t.TempDir()
	seedGroup(t, root,
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n")
	seedRawGroupDir(t, root, "epic-01", "tg-01.2",
		"## [TG-01.2] Second group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 2.0.0\ndepends_on: []\n```\n")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "tg-01.2") && strings.Contains(problem, "version belongs in the epic") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a problem naming the group's own version; got %v", problems)
	}
}

// TestLintProblemsReportsTwoFoldersDeclaringTheSameGroupID proves lint catches a duplicate group id
// instead of leaving Locate to pick whichever folder sorts first.
func TestLintProblemsReportsTwoFoldersDeclaringTheSameGroupID(t *testing.T) {
	root := t.TempDir()
	seedGroup(t, root,
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n")
	seedRawGroupDir(t, root, "epic-01", "tg-01.2",
		"## [TG-01.1] Same id again [P: H] [READY]\n\n```yaml\ntype: feat\ndepends_on: []\n```\n")
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-01.1") && strings.Contains(problem, "duplicate group id") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a duplicate group id problem; got %v", problems)
	}
}

// TestLintProblemsRejectsAGroupFileWithMoreThan12Tasks proves the task cap also runs over a group
// file's own tasks, not only a legacy backlog's.
func TestLintProblemsRejectsAGroupFileWithMoreThan12Tasks(t *testing.T) {
	root := t.TempDir()
	text := "## [TG-63.1] Large group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-63\ndepends_on: []\n```\n\n"
	for i := 1; i <= 13; i++ {
		text += fmt.Sprintf("- [ ] **TSK-63.1.%d** Task %d\n  - files: `a/t%d.go`\n  - done_when: `go test ./...`\n\n", i, i, i)
	}
	seedGroup(t, root, text)
	problems, err := lintProblems(root)
	if err != nil {
		t.Fatalf("lintProblems: %v", err)
	}
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "TG-63.1") && strings.Contains(problem, "exceeds limit of 12") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no problem mentions exceeding 12 tasks; got %v", problems)
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
		runBacklogAdd(root, []string{"EPIC-08", "Epic", "eight", "--version", "1.0.0", "--goal", "Ship eight.", "--status", "READY"})
	})
	if !strings.Contains(out, "EPIC-08") {
		t.Fatalf("add printed no epic id: %s", out)
	}
	epicPath := filepath.Join(root, groupFilesDir, "epic-08", backlog.EpicFileName)
	data, err := os.ReadFile(epicPath)
	if err != nil {
		t.Fatalf("epic file not written: %v", err)
	}
	for _, want := range []string{"## [EPIC-08] Epic eight [READY]", "version: 1.0.0", "type: feat", "\nShip eight.\n"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("epic file lacks %q: %s", want, data)
		}
	}

	out = captureStdout(t, func() {
		runBacklogAdd(root, []string{"TG-08.1", "A", "new", "group"})
	})
	if !strings.Contains(out, "TG-08.1") {
		t.Fatalf("add printed no group id: %s", out)
	}
	dir := filepath.Join(root, groupFilesDir, "epic-08", "tg-08.1")
	data, err = os.ReadFile(filepath.Join(dir, backlog.GroupFileName))
	if err != nil {
		t.Fatalf("group file not written: %v", err)
	}
	if !strings.Contains(string(data), "## [TG-08.1] A new group [P: M] [REFINEMENT]") {
		t.Fatalf("group file = %s", data)
	}
	if !strings.Contains(string(data), "type: feat") {
		t.Fatalf("group file keeps no type: %s", data)
	}
	if strings.Contains(string(data), "version:") || strings.Contains(string(data), "epic:") {
		t.Fatalf("group file repeats what the epic holds: %s", data)
	}

	out = captureStdout(t, func() {
		runBacklogAdd(root, []string{"TG-08.1", "Its", "first", "task", "--files", "a.go,b.go"})
	})
	if !strings.Contains(out, "TSK-08.1.1") {
		t.Fatalf("add printed no task id: %s", out)
	}
	data, err = os.ReadFile(filepath.Join(dir, "tsk-08.1.1.md"))
	if err != nil {
		t.Fatalf("task file not written: %v", err)
	}
	if !strings.Contains(string(data), "- [ ] **TSK-08.1.1** Its first task") {
		t.Fatalf("task file = %s", data)
	}
	if !strings.Contains(string(data), "files: `a.go`, `b.go`") {
		t.Fatalf("task carries no files: %s", data)
	}
	out = captureStdout(t, func() {
		runBacklogAdd(root, []string{"TG-08.1", "Its", "second", "task", "--files", "c.go", "--done-when", "go test ./..."})
	})
	if !strings.Contains(out, "TSK-08.1.2") {
		t.Fatalf("a second add printed no next task id: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "tsk-08.1.2.md")); err != nil {
		t.Fatalf("second task file not written: %v", err)
	}

	backlogOut := captureStdout(t, func() { runBacklog(root) })
	if !strings.Contains(backlogOut, "EPIC-08") || !strings.Contains(backlogOut, "TG-08.1") || !strings.Contains(backlogOut, "1 group(s)") {
		t.Fatalf("backlog after add = %s", backlogOut)
	}
}

// TestRunBacklogAddRefusesAGroupWhoseEpicFolderIsMissing proves a group lands only under an epic that exists.
func TestRunBacklogAddRefusesAGroupWhoseEpicFolderIsMissing(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	got := runCLI(t, root, "", "add", "TG-08.1", "A", "group", "with", "no", "epic")
	if got.code != 1 || !strings.Contains(got.stderr, "add the epic first") {
		t.Fatalf("add exited %d: %s%s; want a refusal naming the missing epic", got.code, got.stdout, got.stderr)
	}
	if _, err := os.Stat(filepath.Join(root, groupFilesDir)); !os.IsNotExist(err) {
		t.Fatalf("add wrote into docs/backlog with no epic: %v", err)
	}
}

// TestRunBacklogAddRefusesAnEpicWithNoVersionOrTwice proves an epic declares its version once.
func TestRunBacklogAddRefusesAnEpicWithNoVersionOrTwice(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	if got := runCLI(t, root, "", "add", "EPIC-08", "Eight"); got.code != 1 || !strings.Contains(got.stderr, "--version") {
		t.Fatalf("add with no version exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if got := runCLI(t, root, "", "add", "EPIC-08", "Eight", "--version", "1.0.0"); got.code != 0 {
		t.Fatalf("add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if got := runCLI(t, root, "", "add", "EPIC-08", "Eight again", "--version", "1.0.0"); got.code != 1 || !strings.Contains(got.stderr, "already exists") {
		t.Fatalf("a second add of the epic exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
}

// TestAddIgnoresALegacyBacklogFileAndWritesTheGroupFolder proves a repo holding both writes into its
// group folder, never the legacy backlog file, since the tree is the current grammar.
func TestAddIgnoresALegacyBacklogFileAndWritesTheGroupFolder(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	seedGroup(t, root,
		"## [TG-08.1] Existing [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-08\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-08.1.1** A task\n  - files: `a.go`\n")
	legacyPath := filepath.Join(root, backlog.LegacyName)
	if err := os.WriteFile(legacyPath, []byte("### [TG-08.1] Existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "add", "TG-08.1", "A", "second", "task", "--files", "b.go")
	if got.code != 0 {
		t.Fatalf("add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	data, err := os.ReadFile(filepath.Join(backlog.GroupDirPath(root, "TG-08.1"), "tsk-08.1.2.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "A second task") {
		t.Fatalf("group folder gained no task file: %s", data)
	}
	legacyData, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(legacyData), "A second task") {
		t.Fatalf("add wrote into the legacy backlog file: %s", legacyData)
	}
}

// TestMainDispatchesBacklogAndAdd proves `komodo backlog` and `komodo add` reach the tree commands.
func TestMainDispatchesBacklogAndAdd(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	if epic := runCLI(t, root, "", "add", "EPIC-09", "Nine", "--version", "1.0.0"); epic.code != 0 {
		t.Fatalf("add epic exited %d: %s%s", epic.code, epic.stdout, epic.stderr)
	}
	add := runCLI(t, root, "", "add", "TG-09.1", "A", "dispatched", "group")
	if add.code != 0 {
		t.Fatalf("add exited %d: %s%s", add.code, add.stdout, add.stderr)
	}
	if !strings.Contains(add.stdout, "TG-09.1") {
		t.Fatalf("add printed no group id: %s", add.stdout)
	}
	path := filepath.Join(root, groupFilesDir, "epic-09", "tg-09.1", backlog.GroupFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("add wrote no group folder: %v", err)
	}
	backlog := runCLI(t, root, "", "backlog")
	if backlog.code != 0 {
		t.Fatalf("backlog exited %d: %s%s", backlog.code, backlog.stdout, backlog.stderr)
	}
	if !strings.Contains(backlog.stdout, "EPIC-09") || !strings.Contains(backlog.stdout, "TG-09.1") || !strings.Contains(backlog.stdout, "1 group(s)") {
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

// TestRunBacklogAddRejectsInvalidGroupPriorityOrStatus proves add validates its own inputs before
// writing a group file, instead of writing a heading lint can never parse back.
func TestRunBacklogAddRejectsInvalidGroupPriorityOrStatus(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"lowercase group id", []string{"tg-08.1", "Auth"}},
		{"group id escaping the backlog dir", []string{"../x", "Auth"}},
		{"unknown priority", []string{"TG-08.1", "Auth", "--priority", "high"}},
		{"unknown status", []string{"TG-08.1", "Auth", "--status", "in_progress"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			runBacklogAdd(root, tc.args)
		})
	}
}

// TestLintProblemsRefusesAnOpenGroupAtATaggedVersion proves lint reads the repo's own tags.
func TestLintProblemsRefusesAnOpenGroupAtATaggedVersion(t *testing.T) {
	root := emptyRepo(t)
	seedGroup(t, root,
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n  - done_when: `go test ./a/...`\n")
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

// TestLintPrintsAGroupFileTasksCallerNote proves komodo lint shows a group file's own notes.
func TestLintPrintsAGroupFileTasksCallerNote(t *testing.T) {
	root := t.TempDir()
	seedGroup(t, root,
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `internal/a/a.go`\n  - done_when: `go test ./internal/a/...`\n")
	if notes := groupFileNotes(root); len(notes) != 1 || !strings.Contains(notes[0], "TSK-01.1.1") {
		t.Fatalf("notes = %v, want TSK-01.1.1 noted for a done_when that never runs its caller", notes)
	}
}

// TestCheckBaseDefaultsToTheGroupsEpicBranch proves check task diffs from the branch the harness cut, not the default.
func TestCheckBaseDefaultsToTheGroupsEpicBranch(t *testing.T) {
	root := emptyRepo(t)
	seedGroup(t, root,
		"## [TG-01.1] First group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** A task\n  - files: `a.go`\n  - done_when: `go test ./a/...`\n")
	runGit(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")
	runGit(t, root, "branch", "feat/1.0.0")
	parsed, err := backlog.LoadRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := checkBase(root, parsed, "TG-01.1", ""); got != "feat/1.0.0" {
		t.Fatalf("checkBase = %q, want the epic branch feat/1.0.0", got)
	}
	if got := checkBase(root, parsed, "TG-01.1", "main"); got != "main" {
		t.Fatalf("checkBase with --base = %q, want main", got)
	}
}

// TestAtoiOrFallsBackOnAnUnparseableNumber proves atoiOr parses a good number and falls back otherwise.
func TestAtoiOrFallsBackOnAnUnparseableNumber(t *testing.T) {
	if got := atoiOr("5", 1); got != 5 {
		t.Fatalf("atoiOr(5) = %d, want 5", got)
	}
	if got := atoiOr("nope", 7); got != 7 {
		t.Fatalf("atoiOr(nope) = %d, want the fallback 7", got)
	}
}
