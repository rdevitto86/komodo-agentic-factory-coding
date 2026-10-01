package guard

import (
	"os/exec"
	"testing"
)

// gitRepo builds a real git repository on branch, so a -C or cd into it resolves a real branch.
func gitRepo(t *testing.T, branch string) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", branch},
		{"config", "user.email", "a@example.com"},
		{"config", "user.name", "a"},
		{"commit", "--allow-empty", "-q", "-m", "seed"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return root
}

// TestGitDashCIsJudgedByTheTargetDirsBranch proves a -C commit is judged by the branch it targets,
// not the session checkout's branch.
func TestGitDashCIsJudgedByTheTargetDirsBranch(t *testing.T) {
	registerFakeHost()
	session := gitRepo(t, "feat/x")
	other := gitRepo(t, "main")
	command := "git -C " + other + " commit -m x"
	decision := CheckCommand(command, session, DefaultPolicy())
	if !decision.Deny {
		t.Fatalf("a -C commit on main is allowed; want it refused")
	}
}

// TestGitDashCOntoANonCriticalWorktreeIsAllowed proves a -C commit targeting a non-critical branch
// is allowed even when the session checkout sits on a critical one.
func TestGitDashCOntoANonCriticalWorktreeIsAllowed(t *testing.T) {
	registerFakeHost()
	session := gitRepo(t, "main")
	other := gitRepo(t, "chore/komodo-line-prep")
	command := "git -C " + other + " commit -m x"
	decision := CheckCommand(command, session, DefaultPolicy())
	if decision.Deny {
		t.Fatalf("a -C commit on a non-critical worktree is refused: %v", decision.Findings)
	}
}

// TestCDThenGitIsJudgedByTheNewDirsBranch proves a cd followed by a git call is judged by the
// branch of the directory the cd moved into, not the session checkout's branch.
func TestCDThenGitIsJudgedByTheNewDirsBranch(t *testing.T) {
	registerFakeHost()
	session := gitRepo(t, "feat/x")
	other := gitRepo(t, "main")
	command := "cd " + other + " && git commit -m x"
	decision := CheckCommand(command, session, DefaultPolicy())
	if !decision.Deny {
		t.Fatalf("a cd then commit on main is allowed; want it refused")
	}
}

// TestSwitchCreateThenMergeIsJudgedByTheNewBranch proves a compound switch -c then merge is judged
// by the branch the switch creates, not the session checkout's branch.
func TestSwitchCreateThenMergeIsJudgedByTheNewBranch(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git switch -c fix/x && git merge origin/main"}},
		DefaultPolicy(), "main")
	if decision.Deny {
		t.Fatalf("a merge onto a freshly switched branch is refused: %v", decision.Findings)
	}
}

// TestCheckoutCreateThenMergeIsJudgedByTheNewBranch proves checkout -b behaves the same as
// switch -c for the branch a later merge in the same command is judged against.
func TestCheckoutCreateThenMergeIsJudgedByTheNewBranch(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git checkout -b fix/y && git merge origin/main"}},
		DefaultPolicy(), "main")
	if decision.Deny {
		t.Fatalf("a merge onto a freshly checked-out branch is refused: %v", decision.Findings)
	}
}

// TestSwitchOntoMainThenMergeIsStillRefused proves the tracking still refuses a merge onto main
// once a compound switches back onto it.
func TestSwitchOntoMainThenMergeIsStillRefused(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git switch main && git merge feat/x"}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatalf("a merge onto main after switching is allowed; want it refused")
	}
}
