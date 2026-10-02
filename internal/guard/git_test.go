package guard

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"komodo/internal/lease"
	"komodo/internal/proc"
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

// TestSwitchDetachOntoMainThenMergeIsAllowed proves --detach is tracked as HEAD, not the ref it
// names, so a merge after detaching onto main is judged on no branch, not main.
func TestSwitchDetachOntoMainThenMergeIsAllowed(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git switch --detach main && git merge feat/x"}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("a merge after a detached switch is refused: %v", decision.Findings)
	}
}

// linkedWorktree adds a detached linked worktree of root at a new path, so a checkout or switch
// run there is judged as a linked worktree would be.
func linkedWorktree(t *testing.T, root string) string {
	t.Helper()
	path := t.TempDir() + "-wt"
	cmd := exec.Command("git", "worktree", "add", "--detach", path)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add --detach %s: %v: %s", path, err, out)
	}
	return path
}

// TestCheckoutOntoAnExistingBranchInALinkedWorktreeIsRefused proves the guard refuses attaching a
// branch a linked worktree does not already hold (decision 0012).
func TestCheckoutOntoAnExistingBranchInALinkedWorktreeIsRefused(t *testing.T) {
	registerFakeHost()
	root := gitRepo(t, "main")
	cmd := exec.Command("git", "branch", "feat/y")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch feat/y: %v: %s", err, out)
	}
	linked := linkedWorktree(t, root)
	decision := CheckCommand("git checkout feat/y", linked, DefaultPolicy())
	if !decision.Deny {
		t.Fatalf("a checkout onto an existing branch in a linked worktree is allowed; want it refused")
	}
	if !containsAny(decision.Findings, "komodo worktree add") {
		t.Fatalf("findings = %v, want it to name komodo worktree add", decision.Findings)
	}
}

// TestCheckoutInTheMainCheckoutStaysFree proves the same checkout run in the main checkout is
// never refused; only a linked worktree pins a branch.
func TestCheckoutInTheMainCheckoutStaysFree(t *testing.T) {
	registerFakeHost()
	root := gitRepo(t, "main")
	cmd := exec.Command("git", "branch", "feat/y")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch feat/y: %v: %s", err, out)
	}
	decision := CheckCommand("git checkout feat/y", root, DefaultPolicy())
	if decision.Deny {
		t.Fatalf("a checkout in the main checkout is refused: %v", decision.Findings)
	}
}

// TestSwitchDetachInALinkedWorktreeIsAllowed proves a detached switch never attaches a branch,
// so it is free even in a linked worktree.
func TestSwitchDetachInALinkedWorktreeIsAllowed(t *testing.T) {
	registerFakeHost()
	root := gitRepo(t, "main")
	cmd := exec.Command("git", "branch", "feat/y")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch feat/y: %v: %s", err, out)
	}
	linked := linkedWorktree(t, root)
	decision := CheckCommand("git switch --detach feat/y", linked, DefaultPolicy())
	if decision.Deny {
		t.Fatalf("a detached switch in a linked worktree is refused: %v", decision.Findings)
	}
}

// leased takes a live lease on branch in repo, held by this test process.
func leased(t *testing.T, repo, branch string) {
	t.Helper()
	holder, ok := proc.Of(os.Getpid())
	if !ok {
		t.Skip("this platform cannot name a process by its start time")
	}
	if err := lease.Take(repo, "TG-1", branch, holder, time.Now()); err != nil {
		t.Fatal(err)
	}
}

// TestASessionsPushOrTipWriteToALeasedBranchIsRefused proves the orchestrator cannot push a branch a live
// builder holds, nor move its refs/komodo tip, and that the refusal names the group, pid and lapse.
func TestASessionsPushOrTipWriteToALeasedBranchIsRefused(t *testing.T) {
	registerFakeHost()
	t.Setenv(RoleEnv, "")
	t.Setenv(lease.RunEnv, "")
	repo := gitRepo(t, "feat/me")
	leased(t, repo, "feat/held")
	for _, command := range []string{
		"git push origin HEAD:refs/heads/feat/held",
		"git push origin feat/held",
		"git update-ref refs/komodo/feat/held HEAD",
	} {
		decision := CheckCommand(command, repo, DefaultPolicy())
		if !decision.Deny {
			t.Fatalf("%q is allowed; want it refused", command)
		}
		text := strings.Join(decision.Findings, "\n")
		if !strings.Contains(text, "TG-1") || !strings.Contains(text, strconv.Itoa(os.Getpid())) || !strings.Contains(text, "until") {
			t.Fatalf("%q findings = %q; want the group, pid and lapse time", command, text)
		}
	}
	for _, command := range []string{"git push origin feat/free", "git update-ref refs/komodo/feat/free HEAD"} {
		if decision := CheckCommand(command, repo, DefaultPolicy()); decision.Deny {
			t.Fatalf("%q refused: %v", command, decision.Findings)
		}
	}
}

// TestTheLeasesOwnRunAndAnExpiredLeasePassTheGuard proves the holder's run and a lapsed lease may push.
func TestTheLeasesOwnRunAndAnExpiredLeasePassTheGuard(t *testing.T) {
	registerFakeHost()
	t.Setenv(RoleEnv, "")
	repo := gitRepo(t, "feat/me")
	leased(t, repo, "feat/held")
	t.Setenv(lease.RunEnv, strconv.Itoa(os.Getpid()))
	if decision := CheckCommand("git push origin feat/held", repo, DefaultPolicy()); decision.Deny {
		t.Fatalf("the lease's own run is refused: %v", decision.Findings)
	}
	t.Setenv(lease.RunEnv, "")
	holder, _ := proc.Of(os.Getpid())
	if err := lease.Take(repo, "TG-1", "feat/held", holder, time.Now().Add(-lease.TTL-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if decision := CheckCommand("git push origin feat/held", repo, DefaultPolicy()); decision.Deny {
		t.Fatalf("an expired lease refuses: %v", decision.Findings)
	}
}
