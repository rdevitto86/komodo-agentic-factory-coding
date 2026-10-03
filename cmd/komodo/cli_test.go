package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/line"
	"komodo/internal/mount"
)

// exitCode is what the swapped exit panics with, so a test can recover the code main chose.
type exitCode int

// cliResult is what one in-process run of the binary printed and the code it exited with.
type cliResult struct {
	stdout string
	stderr string
	code   int
}

// runCLI runs main in dir with args and stdin, capturing both streams and the exit code.
func runCLI(t *testing.T, dir, stdin string, args ...string) cliResult {
	t.Helper()
	// A suite run inside a line session inherits the launcher's pid, which the nested-run guard refuses.
	t.Setenv(line.LockEnv, "")
	oldArgs, oldOut, oldErr, oldIn, oldExit := os.Args, os.Stdout, os.Stderr, os.Stdin, exit
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	open := func(name string) *os.File {
		file, err := os.Create(filepath.Join(scratch, name))
		if err != nil {
			t.Fatal(err)
		}
		return file
	}
	outFile, errFile := open("stdout"), open("stderr")
	if err := os.WriteFile(filepath.Join(scratch, "stdin"), []byte(stdin), 0o644); err != nil {
		t.Fatal(err)
	}
	inFile, err := os.Open(filepath.Join(scratch, "stdin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	result := cliResult{}
	func() {
		defer func() {
			os.Args, os.Stdout, os.Stderr, os.Stdin, exit = oldArgs, oldOut, oldErr, oldIn, oldExit
			_ = os.Chdir(wd)
			if recovered := recover(); recovered != nil {
				code, ok := recovered.(exitCode)
				if !ok {
					panic(recovered)
				}
				result.code = int(code)
			}
		}()
		os.Args = append([]string{"komodo"}, args...)
		os.Stdout, os.Stderr, os.Stdin = outFile, errFile, inFile
		exit = func(code int) { panic(exitCode(code)) }
		main()
	}()
	for _, file := range []*os.File{outFile, errFile, inFile} {
		file.Close()
	}
	out, _ := os.ReadFile(filepath.Join(scratch, "stdout"))
	errOut, _ := os.ReadFile(filepath.Join(scratch, "stderr"))
	result.stdout, result.stderr = string(out), string(errOut)
	return result
}

// cliPendingGroup is the pending group with a done_when the lint reads as a command.
var cliPendingGroup = strings.Replace(pendingGroup, `done_when: ["true"]`, `done_when: ["go vet ./..."]`, 1)

// fixtureRepo builds a committed repo on main holding a two-group backlog and a changelog.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:1")
	root, _ := tagRepo(t, "main", releaseChangelog)
	backlogtest.SeedText(t, root, shippedGroup+"\n"+cliPendingGroup)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "backlog")
	return root
}

// TestDispatchReachesEveryReadOnlyCommand runs each command that touches nothing outside the repo.
func TestDispatchReachesEveryReadOnlyCommand(t *testing.T) {
	root := fixtureRepo(t)
	cases := []struct {
		args []string
		code int
		want string
	}{
		{nil, 2, "the code assembly line"},
		{[]string{"help"}, 0, "komodo lint"},
		{[]string{"--version"}, 0, "komodo dev (unknown)"},
		{[]string{"nonsense"}, 2, `unknown command "nonsense"`},
		{[]string{"lint"}, 0, ""},
		{[]string{"list"}, 0, "TSK-90.2.1"},
		{[]string{"list", "--json"}, 0, `"TSK-90.2.1"`},
		{[]string{"next"}, 0, "TG-90.2"},
		{[]string{"next", "--json"}, 0, `"group":"TG-90.2"`},
		{[]string{"brief", "TSK-90.2.1", "--dry-run"}, 0, "token"},
		{[]string{"brief"}, 1, "usage: komodo brief"},
		{[]string{"detect"}, 0, "languages:"},
		{[]string{"detect", "--json"}, 0, "{"},
		{[]string{"guard", "check"}, 0, "0 wrong"},
		{[]string{"metrics"}, 0, ""},
		{[]string{"step", "--json"}, 0, "{"},
		{[]string{"resume", "TG-90.2"}, 1, "has no saved state to resume"},
		{[]string{"ship"}, 1, "usage: komodo ship"},
		{[]string{"ship", "TSK-90.2.1"}, 1, "TG-90.2 has no ship waiting on a credential"},
		{[]string{"release", "check"}, 0, ""},
		{[]string{"install", "--host", "nope"}, 1, `unknown host "nope"`},
		{[]string{"install", "--host", "ollama"}, 1, "nothing to install"},
		{[]string{"install", "--host", "codex"}, 1, "deferred to a later version"},
		{[]string{"machine"}, 1, "usage: komodo machine"},
		{[]string{"machine", "TSK-90.2.1", "--role", "builder"}, 1, "cannot run on the local machine"},
		{[]string{"machine", "TSK-90.2.1", "--role", "reviewer"}, 1, "no such file"},
		{[]string{"recall", "--model", "m"}, 1, "127.0.0.1:1"},
		{[]string{"run", "TG-90.2", "--dry-run"}, 0, "1. TG-90.2"},
		{[]string{"diff"}, 0, ""},
		{[]string{"close"}, 1, "usage: komodo close"},
		{[]string{"close", "TSK-90.2.1"}, 0, "TSK-90.2.1"},
		{[]string{"comments", "check"}, 0, ""},
		{[]string{"release"}, 1, "usage"},
		{[]string{"release", "publish"}, 1, "publish"},
		{[]string{"gate"}, 0, "gate: komodo guard check"},
		{[]string{"tag"}, 0, "v2.0.0"},
	}
	for _, each := range cases {
		got := runCLI(t, root, "", each.args...)
		if got.code != each.code {
			t.Fatalf("komodo %v exited %d, want %d\nstdout: %s\nstderr: %s", each.args, got.code, each.code, got.stdout, got.stderr)
		}
		if !strings.Contains(got.stdout+got.stderr, each.want) {
			t.Fatalf("komodo %v printed no %q\nstdout: %s\nstderr: %s", each.args, each.want, got.stdout, got.stderr)
		}
	}
}

// TestAddAppendsATaskTheListThenShows proves add writes a task that list and lint then read.
func TestAddAppendsATaskTheListThenShows(t *testing.T) {
	root := fixtureRepo(t)
	if got := runCLI(t, root, "", "add", "TG-90.2", "A second task", "--files", "c/three.go"); got.code != 0 {
		t.Fatalf("add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	got := runCLI(t, root, "", "list", "TG-90.2")
	if !strings.Contains(got.stdout, "A second task") {
		t.Fatalf("list after add = %s", got.stdout)
	}
	// add has no --done-when flag yet; a READY group's new task needs one for lint, so add it by hand.
	groupPath := filepath.Join(root, "docs", "backlog", "TG-90.2-a-pending-group.md")
	data, err := os.ReadFile(groupPath)
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(data), "files: `c/three.go`\n", "files: `c/three.go`\n  - done_when: `go test ./c/...`\n", 1)
	if err := os.WriteFile(groupPath, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	if lint := runCLI(t, root, "", "lint"); lint.code != 0 {
		t.Fatalf("lint after add exited %d: %s", lint.code, lint.stdout)
	}
}

// TestAddHonorsFilesBeforeANewGroupsPositionals proves flags given ahead of group and title still
// land on the new group's first task, instead of being dropped with an empty group left behind.
func TestAddHonorsFilesBeforeANewGroupsPositionals(t *testing.T) {
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "add", "--files", "c/four.go", "--done-when", "go test ./c/...", "TG-91", "A new group")
	if got.code != 0 {
		t.Fatalf("add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	groupPath := filepath.Join(root, "docs", "backlog", "TG-91-a-new-group.md")
	data, err := os.ReadFile(groupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "c/four.go") {
		t.Fatalf("add dropped --files on a new group; got %s", data)
	}
	if !strings.Contains(string(data), "- [ ] **TSK-91.1**") {
		t.Fatalf("add wrote a new group with no task; got %s", data)
	}
}

// TestAddAcceptsRepeatedAcceptFlags proves --accept can be passed more than once instead of comma-joined,
// so an acceptance line may itself hold a comma.
func TestAddAcceptsRepeatedAcceptFlags(t *testing.T) {
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "add", "TG-90.2", "A third task", "--files", "c/five.go",
		"--accept", "first line, with a comma", "--accept", "second line")
	if got.code != 0 {
		t.Fatalf("add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "backlog", "TG-90.2-a-pending-group.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "accept: first line, with a comma") {
		t.Fatalf("add lost the comma in a repeated --accept line; got %s", data)
	}
	if !strings.Contains(string(data), "accept: second line") {
		t.Fatalf("add dropped the second --accept line; got %s", data)
	}
}

// TestAddRefusesAGroupIDTakenOnAnUnmergedBranch proves add checks every local branch's docs/backlog,
// not only the working tree, before writing a new group file.
func TestAddRefusesAGroupIDTakenOnAnUnmergedBranch(t *testing.T) {
	root := fixtureRepo(t)
	runGit(t, root, "checkout", "-b", "feat/TG-08.12-taken")
	writeGroupFile(t, root, "TG-08.12-taken.md",
		"## [TG-08.12] Taken elsewhere [P: H] [REFINEMENT]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-08\ndepends_on: []\n```\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "taken")
	runGit(t, root, "checkout", "main")
	got := runCLI(t, root, "", "add", "TG-08.12", "A fresh checkout cannot see this")
	if got.code == 0 {
		t.Fatalf("add wrote a group id already taken on another branch: %s", got.stdout)
	}
	if !strings.Contains(got.stdout+got.stderr, "TG-08.12") {
		t.Fatalf("refusal names no colliding id: %s%s", got.stdout, got.stderr)
	}
}

// TestAddNextProposesTheFreeGroupIDAcrossBranches proves --next skips every id taken in the working
// tree or on an unmerged branch, for both.
func TestAddNextProposesTheFreeGroupIDAcrossBranches(t *testing.T) {
	root := fixtureRepo(t)
	writeGroupFile(t, root, "TG-08.13-untracked.md",
		"## [TG-08.13] Untracked [P: H] [REFINEMENT]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-08\ndepends_on: []\n```\n")
	runGit(t, root, "checkout", "-b", "feat/TG-08.14-taken")
	writeGroupFile(t, root, "TG-08.14-taken.md",
		"## [TG-08.14] Taken elsewhere [P: H] [REFINEMENT]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-08\ndepends_on: []\n```\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "taken")
	runGit(t, root, "checkout", "main")
	got := runCLI(t, root, "", "add", "--next", "EPIC-08")
	if got.code != 0 {
		t.Fatalf("add --next exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "TG-08.15") {
		t.Fatalf("add --next = %q, want TG-08.15 free past the untracked and unmerged ids", got.stdout)
	}
}

// TestListShowsTheOpenRunsLiveStatus proves list overlays status.json while the group file stays as committed.
func TestListShowsTheOpenRunsLiveStatus(t *testing.T) {
	root := fixtureRepo(t)
	if got := runCLI(t, root, "", "list", "TG-90.2"); !strings.Contains(got.stdout, "[READY]") {
		t.Fatalf("list before the run = %s", got.stdout)
	}
	state := line.RunState{Run: "TG-90.2-1", Group: "TG-90.2", Base: "main", Branch: "feat/a-pending-group", Worktree: root}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	if err := line.RecordStatus(root, "TSK-90.2.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, root, "", "list", "TG-90.2"); !strings.Contains(got.stdout, "TSK-90.2.1       [DONE]") {
		t.Fatalf("list during the run = %s; it must show the run's live status", got.stdout)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "docs", "backlog", "TG-90.2-a-pending-group.md")); !strings.Contains(string(data), "- [ ] **TSK-90.2.1**") {
		t.Fatal("list rewrote the group file")
	}
}

// TestListAfterShipShowsWhatStepSees proves list reads the ship commit's status once status.json is cleared.
func TestListAfterShipShowsWhatStepSees(t *testing.T) {
	root := fixtureRepo(t)
	worktree := filepath.Join(root, line.StateDir, "wt", "TG-90.2")
	runGit(t, root, "worktree", "add", "-b", "feat/a-pending-group", worktree)
	state := line.RunState{Run: "TG-90.2-1", Group: "TG-90.2", Base: "main", Branch: "feat/a-pending-group", Worktree: worktree}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	if err := line.RecordStatus(root, "TSK-90.2.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	plan := &line.Plan{
		Group: "TG-90.2", Title: "A pending group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-pending-group", Worktree: worktree,
		Tasks: []line.PlanTask{{ID: "TSK-90.2.1", Title: "Not done", Status: "READY"}},
	}
	review := line.ResultPath(root, "TG-90.2-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = line.ShipGroup(root, plan, nil, nil)
	if len(line.LoadStatus(root)) != 0 {
		t.Fatal("ship left the group's live status behind")
	}
	if got := runCLI(t, root, "", "list", "TG-90.2"); !strings.Contains(got.stdout, "TSK-90.2.1       [DONE]") {
		t.Fatalf("list after ship = %s; it must show the ship commit's status, as step does", got.stdout)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "docs", "backlog", "TG-90.2-a-pending-group.md")); !strings.Contains(string(data), "- [ ] **TSK-90.2.1**") {
		t.Fatal("ship rewrote the root's group file")
	}
}

// TestGuardHookDeniesAndAllowsThroughMain proves the hook path reads stdin and sets the exit code.
func TestGuardHookDeniesAndAllowsThroughMain(t *testing.T) {
	root := fixtureRepo(t)
	payload := func(command string) string {
		data, err := json.Marshal(map[string]any{
			"hook_event_name": "PreToolUse", "tool_name": "Bash", "cwd": root,
			"tool_input": map[string]any{"command": command},
		})
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	denied := runCLI(t, root, payload("git push origin main"), "guard")
	if !strings.Contains(denied.stdout+denied.stderr, "Refused") {
		t.Fatalf("a push to main passed the hook: %+v", denied)
	}
	allowed := runCLI(t, root, payload("go test ./..."), "guard")
	if allowed.code != 0 || strings.Contains(allowed.stdout+allowed.stderr, "Refused") {
		t.Fatalf("a test run was refused: %+v", allowed)
	}
}

// TestDoctorReportsAndExitsOnProblems proves doctor prints its count and exits non-zero only on a problem.
func TestDoctorReportsAndExitsOnProblems(t *testing.T) {
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "doctor", "--no-git")
	if !strings.Contains(got.stdout, "problem(s)") {
		t.Fatalf("doctor printed no count: %s%s", got.stdout, got.stderr)
	}
	clean := strings.Contains(got.stdout, "\n0 problem(s)") || strings.HasPrefix(got.stdout, "0 problem(s)")
	if clean != (got.code == 0) {
		t.Fatalf("doctor exit %d disagrees with its report: %s", got.code, got.stdout)
	}
	if asJSON := runCLI(t, root, "", "doctor", "--no-git", "--json"); !strings.HasPrefix(strings.TrimSpace(asJSON.stdout), "[") && strings.TrimSpace(asJSON.stdout) != "null" {
		t.Fatalf("doctor --json = %s", asJSON.stdout)
	}
}

// TestDoctorListsEachPluginType proves doctor names each plugin type as disabled when none is installed.
func TestDoctorListsEachPluginType(t *testing.T) {
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "doctor", "--no-git")
	for _, kind := range []string{"notifier", "tool-pack", "stage-hook"} {
		if !strings.Contains(got.stdout, "note plugin "+kind+": disabled, none installed") {
			t.Fatalf("doctor did not list plugin %s: %s%s", kind, got.stdout, got.stderr)
		}
	}
}

// TestCommentsCheckAndReportRunOnACleanRepo proves the comment lint and the report read a fresh repo.
func TestCommentsCheckAndReportRunOnACleanRepo(t *testing.T) {
	root := fixtureRepo(t)
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\n// F does a thing.\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, root, "", "comments", "check", "a.go"); got.code != 0 {
		t.Fatalf("comments check exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if got := runCLI(t, root, "", "report"); got.code != 0 {
		t.Fatalf("report exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
}

// TestCommentsCheckReadsUntrackedFilesButSkipsIgnoredOnes proves the default file set matches git's own.
func TestCommentsCheckReadsUntrackedFilesButSkipsIgnoredOnes(t *testing.T) {
	root := fixtureRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.py\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	restating := "import os\n\n# greet\ndef greet():\n    pass\n"
	if err := os.WriteFile(filepath.Join(root, "greet.py"), []byte(restating), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.py"), []byte(restating), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "comments", "check")
	if got.code != 1 {
		t.Fatalf("comments check on an untracked restating comment exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "greet.py") {
		t.Fatalf("comments check never read the untracked file: %s", got.stdout)
	}
	if strings.Contains(got.stdout, "ignored.py") {
		t.Fatalf("comments check read a file the exclude rules ignore: %s", got.stdout)
	}
}

// TestCommentsCheckFixAppliesTheMechanicalRules proves --fix rewrites a malformed marker on disk.
func TestCommentsCheckFixAppliesTheMechanicalRules(t *testing.T) {
	root := fixtureRepo(t)
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, []byte("package a\n\n// TODO fix this later\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, root, "", "comments", "check", "--fix", "a.go"); got.code != 0 {
		t.Fatalf("comments check --fix exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "// TODO: fix this later") {
		t.Fatalf("fix did not rewrite the file: %s", data)
	}
	if got := runCLI(t, root, "", "comments", "check", "a.go"); got.code != 0 {
		t.Fatalf("the fixed file still fails comments check: %s%s", got.stdout, got.stderr)
	}
}

// TestInstallDryRunWritesNothing proves a dry-run install prints its plan and leaves the tree alone.
func TestInstallDryRunWritesNothing(t *testing.T) {
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "install", "--host", "all", "--dry-run")
	if got.code != 0 {
		t.Fatalf("install --dry-run exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	status := gitLines(root, "status", "--porcelain")
	for _, line := range status {
		if !strings.Contains(line, ".komodo") {
			t.Fatalf("a dry run changed the tree: %v", status)
		}
	}
}

// fakeToolkitBinary points the running binary at a real file outside any go-build dir, so install renders and accepts it.
func fakeToolkitBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "komodo")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := mount.Executable
	t.Cleanup(func() { mount.Executable = saved })
	mount.Executable = func() (string, error) { return path, nil }
	return path
}

// renderedIgnores are the lines install appends for the default host's rendered project copies, in newline's ending.
func renderedIgnores(t *testing.T, root, newline string) string {
	t.Helper()
	host, _ := mount.Get(mount.Names()[0])
	plan, err := host.Render(root, mount.BinaryPath())
	if err != nil {
		t.Fatal(err)
	}
	var out string
	for _, change := range plan.Changes {
		if change.Remove || !(change.Project || change.Seed) {
			continue
		}
		rel, err := filepath.Rel(root, change.Path)
		if err != nil {
			t.Fatal(err)
		}
		out += "/" + filepath.ToSlash(rel) + newline
	}
	if out == "" {
		t.Fatal("the default host renders no project copy, so the test proves nothing")
	}
	return out
}

// TestInstallIgnoresTheStateDirAndRenderedCopiesOnceAndKeepsTheFilesLineEndings proves each ignore line lands once, in the file's ending.
func TestInstallIgnoresTheStateDirAndRenderedCopiesOnceAndKeepsTheFilesLineEndings(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolates check-ignore from the developer's own global excludes
	fakeToolkitBinary(t)
	cases := []struct {
		name, before, after string
	}{
		{"missing", "node_modules/\n", "node_modules/\n/.komodo/\n"},
		{"unanchored", "node_modules/\n.komodo/\n", "node_modules/\n.komodo/\n"},
		{"crlf", "node_modules/\r\n*.log\r\n", "node_modules/\r\n*.log\r\n/.komodo/\r\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := fixtureRepo(t)
			path := filepath.Join(root, ".gitignore")
			if err := os.WriteFile(path, []byte(c.before), 0o644); err != nil {
				t.Fatal(err)
			}
			newline := "\n"
			if strings.Contains(c.before, "\r\n") {
				newline = "\r\n"
			}
			want := c.after + renderedIgnores(t, root, newline)
			for run := 1; run <= 2; run++ {
				got := runCLI(t, root, "", "install")
				if got.code != 0 {
					t.Fatalf("install run %d exited %d: %s%s", run, got.code, got.stdout, got.stderr)
				}
				data, _ := os.ReadFile(path)
				if string(data) != want {
					t.Fatalf("after install run %d .gitignore = %q, want %q", run, data, want)
				}
			}
		})
	}
}

// TestInstallRefusesAHookBinaryThatDoesNotExist proves install never renders a guard hook naming a missing file.
func TestInstallRefusesAHookBinaryThatDoesNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "komodo")
	saved := mount.Executable
	t.Cleanup(func() { mount.Executable = saved })
	mount.Executable = func() (string, error) { return missing, nil }
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "install")
	if got.code == 0 || !strings.Contains(got.stderr, "does not exist") {
		t.Fatalf("install with a missing binary exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if dry := runCLI(t, root, "", "install", "--dry-run"); dry.code != 0 {
		t.Fatalf("install --dry-run with a missing binary exited %d: %s", dry.code, dry.stderr)
	}
}

// TestGateRunsEveryCheckOnAGoModule proves a Go repo gates on its detected vet and tests, then the markdown checks.
func TestGateRunsEveryCheckOnAGoModule(t *testing.T) {
	root := fixtureRepo(t)
	files := map[string]string{
		"go.mod":   "module fixture\n\ngo 1.22\n",
		"a/one.go": "// Package a is a fixture.\npackage a\n\n// One returns one.\nfunc One() int { return 1 }\n",
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "module")
	got := runCLI(t, root, "", "gate")
	for _, step := range []string{"go vet ./...", "gate: go test ./...", "gate: komodo lint", "gate: komodo doctor"} {
		if !strings.Contains(got.stdout, step) {
			t.Fatalf("the gate never reached %q: %s%s", step, got.stdout, got.stderr)
		}
	}
}

// TestNextStartOpensARunTheOtherStationsRead proves next --start cuts a worktree that step and report then see.
func TestNextStartOpensARunTheOtherStationsRead(t *testing.T) {
	root := fixtureRepo(t)
	runGit(t, root, "push", "-u", "origin", "main")
	started := runCLI(t, root, "", "next", "--start", "--json")
	if started.code != 0 || !strings.Contains(started.stdout, `"branch":`) {
		t.Fatalf("next --start = %+v", started)
	}
	if again := runCLI(t, root, "", "next", "--start"); again.code != 0 || !strings.Contains(again.stdout, "TG-90.2") {
		t.Fatalf("a second start on the same group must resume it: %+v", again)
	}
	for _, args := range [][]string{{"step"}, {"report"}, {"diff"}, {"close", "--wave", "1"}, {"doctor", "--prune"}} {
		got := runCLI(t, root, "", args...)
		if got.stdout == "" && got.stderr == "" {
			t.Fatalf("komodo %v printed nothing", args)
		}
	}
}

func TestBareDiffPairsTheOpenGroupWithItsOwnBranch(t *testing.T) {
	root := fixtureRepo(t)
	text := "# Backlog\n\n### [TG-91.1] First\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-91.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n\n" +
		"### [TG-91.2] Second\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
		"#### [TSK-91.2.1] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"true\"]\n```\n"
	if err := os.RemoveAll(filepath.Join(root, "docs", "backlog")); err != nil {
		t.Fatal(err)
	}
	backlogtest.SeedText(t, root, text)
	started := time.Now().UTC()
	for index, group := range []string{"TG-91.1", "TG-91.2"} {
		state := line.RunState{Run: group + "-1", Group: group, Base: "main", Branch: "feat/" + group,
			Worktree: root, Started: started.Add(time.Duration(index) * time.Second)}
		if err := line.SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}
	if plan := currentPlan(root); plan.Group != "TG-91.1" || plan.Branch != "feat/TG-91.1" {
		t.Fatalf("plan = %s on %s; a bare diff must pair the earliest open group with its own branch", plan.Group, plan.Branch)
	}
	if got := runCLI(t, root, "", "diff"); got.code != 0 || !strings.Contains(got.stdout, "Review of group TG-91.1") {
		t.Fatalf("diff = %+v; a bare diff must review the earliest open group", got)
	}
}

// TestInstallSetsUpTheRepoAndTheMachineInOneStep proves a plain install renders the repo's layer and the
// user's global layer, while --global leaves the repo alone.
func TestInstallSetsUpTheRepoAndTheMachineInOneStep(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := fixtureRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "komodo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, root, "", "install"); got.code != 0 {
		t.Fatalf("install exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	for _, path := range []string{filepath.Join(root, ".claude", "settings.json"), filepath.Join(home, ".claude", "settings.json")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v; one install must set up the repo and the machine", path, err)
		}
	}
	other := fixtureRepo(t)
	if err := os.MkdirAll(filepath.Join(other, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "bin", "komodo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, other, "", "install", "--global"); got.code != 0 {
		t.Fatalf("install --global exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if _, err := os.Stat(filepath.Join(other, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("install --global rendered the repo (%v); it must touch only the machine", err)
	}
}
