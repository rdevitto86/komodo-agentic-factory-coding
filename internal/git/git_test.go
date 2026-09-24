package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
		{Path: "/repo/.komodo/wt/detached", Head: "3333333333333333333333333333333333333333"},
	}
	if got := ParseWorktrees(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseWorktrees = %+v, want %+v", got, want)
	}
	if got := ParseWorktrees(""); len(got) != 0 {
		t.Fatalf("ParseWorktrees of nothing = %+v, want none", got)
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
