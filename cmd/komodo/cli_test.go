package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/line"
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
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte("# Backlog\n\n"+shippedGroup+"\n"+cliPendingGroup), 0o644); err != nil {
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
		{[]string{"release", "check"}, 0, ""},
		{[]string{"install", "--host", "nope"}, 1, `unknown host "nope"`},
		{[]string{"install", "--host", "ollama"}, 1, "nothing to install"},
		{[]string{"machine"}, 1, "usage: komodo machine"},
		{[]string{"machine", "TSK-90.2.1", "--role", "builder"}, 1, "cannot run on the local machine"},
		{[]string{"machine", "TSK-90.2.1", "--role", "reviewer"}, 1, "no such file"},
		{[]string{"recall", "--model", "m"}, 1, "127.0.0.1:1"},
		{[]string{"run", "TG-90.2", "--dry-run"}, 1, "no mount is installed"},
		{[]string{"diff"}, 0, ""},
		{[]string{"close"}, 1, "usage: komodo close"},
		{[]string{"close", "TSK-90.2.1"}, 0, "TSK-90.2.1"},
		{[]string{"comments", "check"}, 0, ""},
		{[]string{"release"}, 1, "usage"},
		{[]string{"gate"}, 1, "gate: go vet"},
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
	if got := runCLI(t, root, "", "add", "TG-90.2", "A second task"); got.code != 0 {
		t.Fatalf("add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	got := runCLI(t, root, "", "list", "TG-90.2")
	if !strings.Contains(got.stdout, "A second task") {
		t.Fatalf("list after add = %s", got.stdout)
	}
	if lint := runCLI(t, root, "", "lint"); lint.code != 0 {
		t.Fatalf("lint after add exited %d: %s", lint.code, lint.stdout)
	}
}

// TestListShowsTheOpenRunsLiveStatus proves list overlays status.json while BACKLOG.md stays as committed.
func TestListShowsTheOpenRunsLiveStatus(t *testing.T) {
	root := fixtureRepo(t)
	if got := runCLI(t, root, "", "list", "TG-90.2"); !strings.Contains(got.stdout, "[READY]") {
		t.Fatalf("list before the run = %s", got.stdout)
	}
	state := line.RunState{Run: "TG-90.2-1", Group: "TG-90.2", Base: "main", Branch: "feat/a-pending-group", Worktree: root}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	if err := line.RecordStatus(root, "TSK-90.2.1", "DONE", ""); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, root, "", "list", "TG-90.2"); !strings.Contains(got.stdout, "TSK-90.2.1       [DONE]") {
		t.Fatalf("list during the run = %s; it must show the run's live status", got.stdout)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "BACKLOG.md")); !strings.Contains(string(data), "[TSK-90.2.1] Not done [P: C] [READY]") {
		t.Fatal("list rewrote BACKLOG.md")
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

// TestGateRunsEveryCheckOnAGoModule proves the gate reaches its markdown checks once vet and test pass.
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
	for _, step := range []string{"gate: go vet", "gate: go test", "gate: komodo lint", "gate: komodo doctor"} {
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
