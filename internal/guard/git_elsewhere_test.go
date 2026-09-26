package guard

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// runGit executes git in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestGitDashCIntoALineWorktreeIsJudgedByItsBranch(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
	runGit(t, root, "worktree", "add", "-q", filepath.Join(".komodo", "wt", "TG-1"), "-b", "fix/group")
	policy := DefaultPolicy()
	cases := []struct {
		name, command string
		deny          bool
	}{
		{"rebase the group branch", "git -C .komodo/wt/TG-1 rebase main", false},
		{"reset the group branch", "git -C .komodo/wt/TG-1 reset -q --hard", false},
		{"commit on the group branch", "git -C .komodo/wt/TG-1 commit -m 'fix: x'", false},
		{"push the group branch to main", "git -C .komodo/wt/TG-1 push origin HEAD:main", true},
		{"a checkout outside the repo", "git -C ../other commit -m 'fix: x'", true},
		{"a missing directory inside the repo", "git -C .komodo/wt/gone commit -m 'fix: x'", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			decision := CheckCommand(c.command, root, policy)
			if decision.Deny != c.deny {
				t.Fatalf("%s: deny = %v, want %v (%v)", c.command, decision.Deny, c.deny, decision.Findings)
			}
		})
	}
}
