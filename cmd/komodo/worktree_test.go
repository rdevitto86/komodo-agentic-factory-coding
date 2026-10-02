package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeAddCutsADetachedWorktreeAndPrintsThePushCommand(t *testing.T) {
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	got := runCLI(t, root, "", "worktree", "add", "task/x")
	if got.code != 0 {
		t.Fatalf("worktree add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(resolved, ".komodo", "wt", "x")
	if !strings.Contains(got.stdout, path) {
		t.Fatalf("stdout = %q, want the cut path %q", got.stdout, path)
	}
	if !strings.Contains(got.stdout, "git -C "+path+" push origin HEAD:refs/heads/task/x") {
		t.Fatalf("stdout = %q, want the push command", got.stdout)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat = %v; the worktree must be cut", err)
	}
}

func TestWorktreeAddRefusesACriticalBranch(t *testing.T) {
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	got := runCLI(t, root, "", "worktree", "add", "main")
	if got.code == 0 || !strings.Contains(got.stderr, "critical") {
		t.Fatalf("want a critical-branch refusal, got %d: %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestWorktreeAddUsageWithNoBranch(t *testing.T) {
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	got := runCLI(t, root, "", "worktree", "add")
	if got.code == 0 || !strings.Contains(got.stderr, "usage: komodo worktree add") {
		t.Fatalf("want the usage line, got %d: %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestWorktreeAddReadsFromAfterTheBranch(t *testing.T) {
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	first := gitOutput(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "later.txt"), []byte("later\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "later.txt")
	runGit(t, root, "commit", "-q", "-m", "later")
	got := runCLI(t, root, "", "worktree", "add", "task/y", "--from", first)
	if got.code != 0 {
		t.Fatalf("worktree add exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	head := gitOutput(t, filepath.Join(root, ".komodo", "wt", "y"), "rev-parse", "HEAD")
	if head != first {
		t.Fatalf("worktree HEAD = %s, want --from %s", head, first)
	}
}
