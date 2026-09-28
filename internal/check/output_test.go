package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a git repo with one commit and returns its path.
func newRepo(t *testing.T) string {
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
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestCompareFindsNoProblemWhenNothingChanged(t *testing.T) {
	repo := newRepo(t)
	before, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	after, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if problems := Compare(before, after); len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
}

func TestCompareCatchesAModelCommit(t *testing.T) {
	repo := newRepo(t)
	before, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "sneaky.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "a model commit")
	after, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	problems := Compare(before, after)
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "moved HEAD") {
		t.Fatalf("problems = %v, want one naming a moved HEAD", problems)
	}
}

func TestCompareCatchesAChangedRef(t *testing.T) {
	repo := newRepo(t)
	before, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	gitRun(t, repo, "tag", "sneaky")
	after, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	problems := Compare(before, after)
	if len(problems) != 1 || !strings.Contains(problems[0], "refs/tags/sneaky") || !strings.Contains(problems[0], "added") {
		t.Fatalf("problems = %v, want one naming the added ref", problems)
	}
}

func TestCompareCatchesAChangedHook(t *testing.T) {
	repo := newRepo(t)
	before, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	hookPath := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	after, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	problems := Compare(before, after)
	if len(problems) != 1 || !strings.Contains(problems[0], "pre-commit") {
		t.Fatalf("problems = %v, want one naming the changed hook", problems)
	}
}

func TestCompareCatchesAChangedConfig(t *testing.T) {
	repo := newRepo(t)
	before, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	gitRun(t, repo, "config", "user.email", "sneaky@example.com")
	after, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	problems := Compare(before, after)
	if len(problems) != 1 || !strings.Contains(problems[0], "user.email") {
		t.Fatalf("problems = %v, want one naming the changed config key", problems)
	}
}

func TestCompareCatchesAChangeToItsOwnBranchsConfig(t *testing.T) {
	repo := newRepo(t)
	own := strings.TrimSpace(mustOutput(t, repo, "symbolic-ref", "--short", "HEAD"))
	before, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	gitRun(t, repo, "config", "branch."+own+".remote", "sneaky")
	after, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	problems := Compare(before, after)
	if len(problems) != 1 || !strings.Contains(problems[0], "branch."+own+".remote") {
		t.Fatalf("problems = %v, want one naming the own branch's config key", problems)
	}
}

func TestCompareCatchesASwitchedBranch(t *testing.T) {
	repo := newRepo(t)
	before, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	gitRun(t, repo, "checkout", "-q", "-b", "sneaky")
	after, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	joined := strings.Join(Compare(before, after), "\n")
	if !strings.Contains(joined, "switched") || !strings.Contains(joined, "sneaky") {
		t.Fatalf("problems = %q, want one naming the switched branch", joined)
	}
}

func TestCompareIgnoresAnotherLanesBranchRefsAndConfig(t *testing.T) {
	repo := newRepo(t)
	lane := filepath.Join(t.TempDir(), "lane")
	gitRun(t, repo, "worktree", "add", "-q", "-b", "feat/other", lane)
	before, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lane, "other.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, lane, "add", "-A")
	gitRun(t, lane, "commit", "-q", "-m", "another lane ships")
	gitRun(t, lane, "update-ref", "refs/remotes/origin/feat/other", "HEAD")
	gitRun(t, lane, "config", "branch.feat/other.remote", "origin")
	gitRun(t, lane, "config", "branch.feat/other.merge", "refs/heads/feat/other")
	gitRun(t, repo, "branch", "feat/third")
	after, err := TakeSnapshot(repo)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if problems := Compare(before, after); len(problems) != 0 {
		t.Fatalf("problems = %v, want none from another lane's branch, remote ref and branch config", problems)
	}
}
