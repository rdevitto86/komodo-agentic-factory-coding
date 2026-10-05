package check

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// initRepo creates a git repo at base HEAD with one committed file, then commits again with edits
// to the given files so a caller can diff base against HEAD.
func initRepo(t *testing.T, edits map[string]string) (worktree, base string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "builder@example.com")
	run("config", "user.name", "builder")
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	baseSHA := strings.TrimSpace(mustOutput(t, dir, "rev-parse", "HEAD"))
	for name, content := range edits {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-A")
	run("commit", "-q", "-m", "edit")
	return dir, baseSHA
}

func mustOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestScopeFailsAnEditOutsideTheGroupsFiles(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{"a.go": "package a\n", "b.go": "package b\n"})
	problems := Scope(worktree, base, []string{"a.go"})
	if len(problems) != 1 || !strings.Contains(problems[0], "b.go") {
		t.Fatalf("problems = %v, want one naming b.go", problems)
	}
}

// writeFile writes content to name in dir without staging or committing it.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScopeFailsAnUncommittedEditOutsideTheGroupsFiles(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{"a tracked file edited and left uncommitted", "seed.txt"},
		{"a new file left untracked", "c.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worktree, base := initRepo(t, map[string]string{"a.go": "package a\n"})
			writeFile(t, worktree, tc.file, "changed\n")
			problems := Scope(worktree, base, []string{"a.go"})
			if len(problems) != 1 || !strings.Contains(problems[0], tc.file) {
				t.Fatalf("problems = %v, want one naming %s", problems, tc.file)
			}
		})
	}
}

func TestDiffCoversCommittedUncommittedAndUntrackedEdits(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{"a.go": "package a\n"})
	writeFile(t, worktree, "seed.txt", "seed\nmore\n")
	writeFile(t, worktree, "c.go", "package c\n\nvar x = 1\n")
	diff, err := Diff(worktree, base)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	got := map[string]bool{}
	for _, line := range ParseAddedLines(diff) {
		got[fmt.Sprintf("%s:%d %s", line.File, line.Line, line.Text)] = true
	}
	for _, want := range []string{"a.go:1 package a", "seed.txt:2 more", "c.go:1 package c", "c.go:3 var x = 1"} {
		if !got[want] {
			t.Fatalf("added lines = %v, want %q", got, want)
		}
	}
}

func TestDiffIgnoresTheCallersGitConfig(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{"a.go": "package a\n", "café.go": "package a\n"})
	mustOutput(t, worktree, "config", "diff.noprefix", "true")
	mustOutput(t, worktree, "config", "color.ui", "always")
	diff, err := Diff(worktree, base)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	got := map[string]bool{}
	for _, line := range ParseAddedLines(diff) {
		got[line.File] = true
	}
	for _, want := range []string{"a.go", "café.go"} {
		if !got[want] {
			t.Fatalf("added lines = %v, want %q", got, want)
		}
	}
}

func TestScopeAllowsADeclaredFilesOwnTest(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{
		"a.go": "package a\n", "a_test.go": "package a\n", "b/b_test.go": "package b\n", "ui/c.test.ts": "x\n",
	})
	problems := Scope(worktree, base, []string{"a.go", "ui/c.ts"})
	if len(problems) != 1 || !strings.Contains(problems[0], "b_test.go") {
		t.Fatalf("problems = %v; a declared file's test is in scope, an undeclared file's test is not", problems)
	}
}

func TestScopeAllowsADeclaredFilesPlatformTest(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{
		"a/a_unix_test.go": "package a\n", "b/b_linux_amd64_test.go": "package b\n", "c/c_windows_test.go": "package c\n",
	})
	problems := Scope(worktree, base, []string{"a/a.go", "b/b.go"})
	if len(problems) != 1 || !strings.Contains(problems[0], "c_windows_test.go") {
		t.Fatalf("problems = %v; a declared file's platform test is in scope, an undeclared file's is not", problems)
	}
}

func TestUntaggedKeepsANameWithNoPlatformSuffix(t *testing.T) {
	for name, want := range map[string]string{
		"x/evidence_unix.go": "x/evidence.go", "watch_windows.go": "watch.go", "a_linux_amd64.go": "a.go",
		"_unix.go": "_unix.go", "user_input.go": "user_input.go", "README.md": "README.md",
	} {
		if got := untagged(name); got != want {
			t.Errorf("untagged(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestScopeAllowsAnyTestOfADeclaredGoPackage(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{
		"cmd/cli_test.go": "package main\n", "cmd/main_test.go": "package main\n", "lib/x_test.go": "package lib\n",
	})
	problems := Scope(worktree, base, []string{"cmd/host.go"})
	if len(problems) != 1 || !strings.Contains(problems[0], "lib/x_test.go") {
		t.Fatalf("problems = %v; a declared package's tests are in scope, another package's are not", problems)
	}
}

func TestScopeAllowsAFileInsideADeclaredDirectory(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{
		"skills/review/SKILL.md": "x\n", "skills/reviewer.md": "x\n",
	})
	problems := Scope(worktree, base, []string{"skills/review"})
	if len(problems) != 1 || !strings.Contains(problems[0], "skills/reviewer.md") {
		t.Fatalf("problems = %v; a declared directory covers its files, never a sibling sharing its prefix", problems)
	}
}

func TestScopePassesWhenEveryEditIsDeclared(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{"a.go": "package a\n"})
	if problems := Scope(worktree, base, []string{"a.go"}); len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestScopeSkipsWithNoBase(t *testing.T) {
	worktree, _ := initRepo(t, map[string]string{"a.go": "package a\n"})
	if problems := Scope(worktree, "", []string{"a.go"}); problems != nil {
		t.Fatalf("problems = %v, want nil with no base", problems)
	}
}

func TestRunReportsEachFailure(t *testing.T) {
	cases := []struct {
		name    string
		checks  []string
		wantLen int
		want    string
	}{
		{"everything passes", []string{"true"}, 0, ""},
		{"a check fails", []string{"exit 1"}, 1, "check:"},
		{"two checks fail", []string{"exit 1", "exit 2"}, 2, "check:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worktree, base := initRepo(t, map[string]string{"a.go": "package a\n"})
			problems := Run(Group{Worktree: worktree, Base: base, Files: []string{"a.go"}}, tc.checks)
			if len(problems) != tc.wantLen {
				t.Fatalf("problems = %v, want %d", problems, tc.wantLen)
			}
			if tc.want != "" && !strings.HasPrefix(problems[0], tc.want) {
				t.Fatalf("problems[0] = %q, want prefix %q", problems[0], tc.want)
			}
		})
	}
}

func TestRunContextKillsItsCommandsOnceTheContextIsDone(t *testing.T) {
	const (
		stopAfter = 200 * time.Millisecond
		bound     = 10 * time.Second
	)
	worktree, base := initRepo(t, map[string]string{"a.go": "package a\n"})
	ctx, cancel := context.WithTimeout(context.Background(), stopAfter)
	defer cancel()
	began := time.Now()
	problems := RunContext(ctx, Group{Worktree: worktree, Base: base, Files: []string{"a.go"}}, []string{"sleep 60"})
	if took := time.Since(began); took > bound {
		t.Fatalf("run took %s after its context ended, want under %s", took, bound)
	}
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "check: `sleep 60`") {
		t.Fatalf("problems = %v, want the killed command named", problems)
	}
}

func TestRunCombinesFailuresWithScope(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{"a.go": "package a\n", "b.go": "package b\n"})
	problems := Run(Group{Worktree: worktree, Base: base, Files: []string{"a.go"}}, []string{"exit 1"})
	if len(problems) != 2 {
		t.Fatalf("problems = %v, want a check failure and a scope failure", problems)
	}
}

// advanceBase points a trunk branch at base, commits a file outside the group's scope on it, and returns to HEAD.
func advanceBase(t *testing.T, worktree, base string) string {
	t.Helper()
	mustOutput(t, worktree, "checkout", "-q", "-b", "trunk", base)
	writeFile(t, worktree, "z.go", "package z\n")
	mustOutput(t, worktree, "add", "-A")
	mustOutput(t, worktree, "commit", "-q", "-m", "base moves on")
	mustOutput(t, worktree, "checkout", "-q", "-")
	return "trunk"
}

// commitAll stages every change in dir and commits it under message.
func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	mustOutput(t, dir, "add", "-A")
	mustOutput(t, dir, "commit", "-q", "-m", message)
}

func TestScopeRunsAgainstTheBranchsOwnForkPoint(t *testing.T) {
	cases := []struct {
		name string
		refs func(t *testing.T, worktree, seed, integration string)
	}{
		{"a stale local base behind its origin copy", func(t *testing.T, worktree, seed, integration string) {
			mustOutput(t, worktree, "branch", "stack", seed)
			mustOutput(t, worktree, "update-ref", "refs/remotes/origin/stack", integration)
		}},
		{"a stack branch held only on origin", func(t *testing.T, worktree, _, integration string) {
			mustOutput(t, worktree, "update-ref", "refs/remotes/origin/stack", integration)
		}},
		{"an integration branch held only at the line's tip", func(t *testing.T, worktree, _, integration string) {
			mustOutput(t, worktree, "update-ref", "refs/komodo/stack", integration)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worktree, seed := initRepo(t, map[string]string{"z.go": "package z\n"})
			integration := strings.TrimSpace(mustOutput(t, worktree, "rev-parse", "HEAD"))
			tc.refs(t, worktree, seed, integration)
			writeFile(t, worktree, "a.go", "package a\n")
			commitAll(t, worktree, "group work")
			if problems := Scope(worktree, "stack", []string{"a.go"}); len(problems) != 0 {
				t.Fatalf("problems = %v, want none; z.go is the integration branch's, not the group's", problems)
			}
		})
	}
}

func TestScopeNeverCountsABacklogTick(t *testing.T) {
	const (
		task = "docs/backlog/epic-1/tg-1.1/tsk-1.1.1.md"
		open = "- [ ] **TSK-1.1.1** One\n  - files: `a.go`\n"
	)
	cases := []struct {
		name     string
		edited   string
		problems int
	}{
		{"a tick", "- [x] **TSK-1.1.1** One\n  - files: `a.go`\n", 0},
		{"a tick and a retitled task", "- [x] **TSK-1.1.1** Renamed\n  - files: `a.go`\n", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worktree, _ := initRepo(t, map[string]string{task: open})
			base := strings.TrimSpace(mustOutput(t, worktree, "rev-parse", "HEAD"))
			writeFile(t, worktree, task, tc.edited)
			writeFile(t, worktree, "a.go", "package a\n")
			commitAll(t, worktree, "close TSK-1.1.1")
			if problems := Scope(worktree, base, []string{"a.go"}); len(problems) != tc.problems {
				t.Fatalf("problems = %v, want %d", problems, tc.problems)
			}
		})
	}
}

// TestOnlyTicksReadsANestedTaskFile proves a tick anywhere under docs/backlog counts, while a file
// elsewhere or one whose heading changed does not.
func TestOnlyTicksReadsANestedTaskFile(t *testing.T) {
	const (
		task  = "docs/backlog/epic-12/tg-12.3/tsk-12.3.2.md"
		group = "docs/backlog/epic-12/tg-12.3/TG.md"
		open  = "- [ ] **TSK-12.3.2** Two\n"
	)
	worktree, _ := initRepo(t, map[string]string{
		task: open, group: "## [TG-12.3] G [P: M] [READY]\n", "notes/todo.md": open,
	})
	base := strings.TrimSpace(mustOutput(t, worktree, "rev-parse", "HEAD"))
	writeFile(t, worktree, task, "- [x] **TSK-12.3.2** Two\n")
	writeFile(t, worktree, group, "## [TG-12.3] G [P: M] [DONE]\n")
	writeFile(t, worktree, "notes/todo.md", "- [x] **TSK-12.3.2** Two\n")
	commitAll(t, worktree, "tick")
	for path, want := range map[string]bool{task: true, group: false, "notes/todo.md": false} {
		if got := onlyTicks(worktree, base, path); got != want {
			t.Fatalf("onlyTicks(%s) = %v, want %v", path, got, want)
		}
	}
}

func TestScopeAndDiffIgnoreCommitsLandingOnBaseAfterTheFork(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{"a.go": "package a\n"})
	trunk := advanceBase(t, worktree, base)
	if problems := Scope(worktree, trunk, []string{"a.go"}); len(problems) != 0 {
		t.Fatalf("problems = %v, want none for a file only the base changed", problems)
	}
	diff, err := Diff(worktree, trunk)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if strings.Contains(diff, "z.go") || !strings.Contains(diff, "+package a") {
		t.Fatalf("diff = %q, want a.go added and nothing of z.go", diff)
	}
}
