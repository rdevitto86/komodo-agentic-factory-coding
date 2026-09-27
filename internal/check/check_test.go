package check

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestScopeAllowsADeclaredFilesOwnTest(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{
		"a.go": "package a\n", "a_test.go": "package a\n", "b_test.go": "package b\n", "ui/c.test.ts": "x\n",
	})
	problems := Scope(worktree, base, []string{"a.go", "ui/c.ts"})
	if len(problems) != 1 || !strings.Contains(problems[0], "b_test.go") {
		t.Fatalf("problems = %v; a declared file's test is in scope, an undeclared file's test is not", problems)
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
		format  string
		lint    string
		checks  []string
		wantLen int
		want    string
	}{
		{"everything passes", "true", "true", []string{"true"}, 0, ""},
		{"format fails", "exit 1", "true", nil, 1, "format:"},
		{"lint fails", "true", "exit 2", nil, 1, "lint:"},
		{"a group check fails", "true", "true", []string{"exit 3"}, 1, "check:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worktree, base := initRepo(t, map[string]string{"a.go": "package a\n"})
			problems := Run(Group{Worktree: worktree, Base: base, Files: []string{"a.go"}}, tc.format, tc.lint, tc.checks)
			if len(problems) != tc.wantLen {
				t.Fatalf("problems = %v, want %d", problems, tc.wantLen)
			}
			if tc.want != "" && !strings.HasPrefix(problems[0], tc.want) {
				t.Fatalf("problems[0] = %q, want prefix %q", problems[0], tc.want)
			}
		})
	}
}

func TestRunCombinesFailuresWithScope(t *testing.T) {
	worktree, base := initRepo(t, map[string]string{"a.go": "package a\n", "b.go": "package b\n"})
	problems := Run(Group{Worktree: worktree, Base: base, Files: []string{"a.go"}}, "exit 1", "true", nil)
	if len(problems) != 2 {
		t.Fatalf("problems = %v, want a format failure and a scope failure", problems)
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
