package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseWorktrees(t *testing.T) {
	out := strings.Join([]string{
		"worktree /repo",
		"HEAD 1111111111111111111111111111111111111111",
		"branch refs/heads/main",
		"",
		"worktree /repo/.komodo/wt/a",
		"HEAD 2222222222222222222222222222222222222222",
		"branch refs/heads/feat/a",
		"",
		"worktree /repo/.komodo/wt/detached",
		"HEAD 3333333333333333333333333333333333333333",
		"detached",
	}, "\n")
	want := []Worktree{
		{Path: "/repo", Branch: "main", Head: "1111111111111111111111111111111111111111"},
		{Path: "/repo/.komodo/wt/a", Branch: "feat/a", Head: "2222222222222222222222222222222222222222"},
		{Path: "/repo/.komodo/wt/detached", Head: "3333333333333333333333333333333333333333", Detached: true},
	}
	if got := ParseWorktrees(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseWorktrees = %+v, want %+v", got, want)
	}
	if got := ParseWorktrees(""); len(got) != 0 {
		t.Fatalf("ParseWorktrees of nothing = %+v, want none", got)
	}
}

// TestRunKillsAHungGitAndItsChildren proves a git command past Timeout is killed, process group included,
// so a hung git never blocks the gate or a hook.
func TestRunKillsAHungGitAndItsChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 30 &\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	saved := Timeout
	Timeout = 200 * time.Millisecond
	t.Cleanup(func() { Timeout = saved })
	started := time.Now()
	if _, err := Run(t.TempDir(), "status"); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Run error = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}

func TestRunOrAndWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if _, err := Run(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	branch, err := Run(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || branch != "main" {
		t.Fatalf("Run = %q, %v; want main", branch, err)
	}
	_, err = Run(root, "rev-parse", "--verify", "refs/heads/missing")
	if err == nil || !strings.HasPrefix(err.Error(), "git rev-parse --verify refs/heads/missing: ") {
		t.Fatalf("Run error = %v, want it to name the command", err)
	}
	if got := Or(root, "rev-parse", "--verify", "refs/heads/missing"); got != "" {
		t.Fatalf("Or on failure = %q, want empty", got)
	}
	if got := Or(root, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Fatalf("Or = %q, want main", got)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	if _, err := Run(root, "worktree", "add", "-q", "-b", "feat/x", linked); err != nil {
		t.Fatal(err)
	}
	worktrees, err := Worktrees(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(worktrees) != 2 || worktrees[0].Branch != "main" || worktrees[1].Branch != "feat/x" || worktrees[1].Head == "" {
		t.Fatalf("Worktrees = %+v", worktrees)
	}
	if resolved, _ := filepath.EvalSymlinks(linked); worktrees[1].Path != linked && worktrees[1].Path != resolved {
		t.Fatalf("Worktrees path = %q, want %q", worktrees[1].Path, linked)
	}
	if _, err := Worktrees(filepath.Join(os.TempDir(), "komodo-no-such-dir")); err == nil {
		t.Fatal("Worktrees outside a repo succeeded")
	}
}

func TestWorktreesReadsADetachedWorktreesTrackedBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if _, err := Run(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	detached := filepath.Join(t.TempDir(), "detached")
	if _, err := Run(root, "worktree", "add", "-q", "--detach", detached); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(root, "config", "extensions.worktreeConfig", "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(detached, "config", "--worktree", "komodo.branch", "task/x"); err != nil {
		t.Fatal(err)
	}
	worktrees, err := Worktrees(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(worktrees) != 2 || !worktrees[1].Detached || worktrees[1].Tracked != "task/x" {
		t.Fatalf("Worktrees = %+v, want the second detached and tracking task/x", worktrees)
	}
	if got := TrackedBranch(root); got != "main" {
		t.Fatalf("TrackedBranch(root) = %q, want main", got)
	}
	if got := TrackedBranch(detached); got != "task/x" {
		t.Fatalf("TrackedBranch(detached) = %q, want task/x", got)
	}
}

// TestWorktreesReadsEachTrackedBranchInsideAHook sets GIT_DIR as a git hook does, and still reads
// each detached worktree's own komodo.branch rather than the hook's.
func TestWorktreesReadsEachTrackedBranchInsideAHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
		{"config", "extensions.worktreeConfig", "true"},
	} {
		if _, err := Run(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	paths := map[string]string{}
	for _, branch := range []string{"task/a", "task/b"} {
		path := filepath.Join(t.TempDir(), "wt")
		if _, err := Run(root, "worktree", "add", "-q", "--detach", path); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(path, "config", "--worktree", "komodo.branch", branch); err != nil {
			t.Fatal(err)
		}
		paths[branch] = path
	}
	hookDir, err := Run(paths["task/a"], "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", hookDir)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(hookDir, "index"))

	worktrees, err := Worktrees(paths["task/b"])
	if err != nil {
		t.Fatal(err)
	}
	tracked := map[string]bool{}
	for _, worktree := range worktrees {
		tracked[worktree.Tracked] = true
	}
	if !tracked["task/a"] || !tracked["task/b"] {
		t.Fatalf("Worktrees = %+v; inside a hook each worktree must keep its own tracked branch", worktrees)
	}
	if got := TrackedBranch(paths["task/b"]); got != "task/b" {
		t.Fatalf("TrackedBranch = %q, want task/b; the hook's GIT_DIR must not redirect it", got)
	}
}

func TestWithoutRepoPointersKeepsEverythingElse(t *testing.T) {
	env := []string{"GIT_DIR=/x/.git", "GIT_WORK_TREE=/x", "GIT_COMMON_DIR=/x/.git", "PATH=/usr/bin", "GIT_AUTHOR_NAME=t"}
	want := []string{"PATH=/usr/bin", "GIT_AUTHOR_NAME=t"}
	if got := WithoutRepoPointers(env); !reflect.DeepEqual(got, want) {
		t.Fatalf("WithoutRepoPointers = %v, want %v", got, want)
	}
}
