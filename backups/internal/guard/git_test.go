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

// checkCommand judges a shell command the way a real host's hook would, through Check.
func checkCommand(command, cwd string, policy Policy) Decision {
	return Check(Request{
		HookEventName: "PreToolUse", ToolName: "Bash", Cwd: cwd,
		ToolInput: map[string]any{"command": command},
	}, policy, CurrentBranch(cwd))
}

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
	decision := checkCommand(command, session, DefaultPolicy())
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
	decision := checkCommand(command, session, DefaultPolicy())
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
	decision := checkCommand(command, session, DefaultPolicy())
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
	decision := checkCommand("git checkout feat/y", linked, DefaultPolicy())
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
	decision := checkCommand("git checkout feat/y", root, DefaultPolicy())
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
	decision := checkCommand("git switch --detach feat/y", linked, DefaultPolicy())
	if decision.Deny {
		t.Fatalf("a detached switch in a linked worktree is refused: %v", decision.Findings)
	}
}

// TestAShortForceClusterIsStillRefused proves a packed short-option cluster such as -fu is read
// as force, the same as the long spelling, since -f never arrives alone on a common push.
func TestAShortForceClusterIsStillRefused(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git push -fu origin feat/x"}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatal("git push -fu is allowed; want the packed force cluster refused")
	}
	if !containsAny(decision.Findings, "never rewritten") {
		t.Fatalf("findings = %v, want the history-rewrite rule named", decision.Findings)
	}
}

// TestNoVerifyClustersAndAbbreviationsAreStillCaught proves hasNoVerify reads -n packed ahead of
// a value-taking short flag and an unambiguous --no-verify prefix, not only the exact spellings.
func TestNoVerifyClustersAndAbbreviationsAreStillCaught(t *testing.T) {
	registerFakeHost()
	for _, command := range []string{"git commit -nm x", "git commit --no-verif -m x"} {
		decision := Check(
			Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
				ToolInput: map[string]any{"command": command}},
			DefaultPolicy(), "feat/x")
		if !decision.Deny {
			t.Fatalf("%q is allowed; want the no-verify rule to catch it", command)
		}
		if !containsAny(decision.Findings, "skips the gate") {
			t.Fatalf("%q findings = %v, want the no-verify rule named", command, decision.Findings)
		}
	}
}

// TestNoVerifysNNeverMatchesInsideAValue proves a value-taking flag before n in a short cluster
// consumes the rest of the cluster, so -mn never reads as a packed -n.
func TestNoVerifysNNeverMatchesInsideAValue(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git commit -mn"}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("git commit -mn is refused: %v", decision.Findings)
	}
}

// TestBranchForceOntoACriticalStartPointIsAllowed proves git branch -f judges only the branch it
// resets, never a critical ref it merely reads as the new start point.
func TestBranchForceOntoACriticalStartPointIsAllowed(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git branch -f feat/y main"}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("git branch -f feat/y main is refused: %v", decision.Findings)
	}
}

// TestBranchForceOfACriticalRefIsStillRefused proves git branch -f still refuses resetting the
// critical ref itself, when it is the branch being force-moved, not merely the start point.
func TestBranchForceOfACriticalRefIsStillRefused(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git branch -f main feat/y"}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatal("git branch -f main feat/y is allowed; want the force-moved critical ref refused")
	}
}

// TestBranchCopyFromACriticalRefIsAllowed proves git branch -c judges only the destination it
// creates, never the critical ref it copies from.
func TestBranchCopyFromACriticalRefIsAllowed(t *testing.T) {
	registerFakeHost()
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git branch -c main feat/copy"}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("git branch -c main feat/copy is refused: %v", decision.Findings)
	}
}

// TestUnsafeModeStillRefusesACriticalRefDelete proves a critical-ref delete is refused in every
// mode, unsafe included, since unsafe only loosens the plain push-to-critical-ref rule.
func TestUnsafeModeStillRefusesACriticalRefDelete(t *testing.T) {
	registerFakeHost()
	policy := DefaultPolicy()
	policy.Mode = ModeUnsafe
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git push --delete origin main"}},
		policy, "feat/x")
	if !decision.Deny {
		t.Fatal("an unsafe-mode delete of a critical ref is allowed; want it refused in every mode")
	}
}

// TestUnsafeModeStillRefusesAModelsPushToAnEpicBranch proves a harness session's push to an epic
// branch is refused in every mode, unsafe included, since only the conductor ever pushes one.
func TestUnsafeModeStillRefusesAModelsPushToAnEpicBranch(t *testing.T) {
	registerFakeHost()
	t.Setenv(RoleEnv, "builder")
	policy := DefaultPolicy()
	policy.Mode = ModeUnsafe
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git push origin feat/1.0.0-alpha.7"}},
		policy, "feat/x")
	if !decision.Deny {
		t.Fatal("an unsafe-mode push to an epic branch from a harness session is allowed; want it refused")
	}
}

// TestUnsafeModeAllowsAPlainPushToACriticalRef proves unsafe mode still loosens the one rule it
// names: a plain push landing on a critical ref, with no delete and no epic branch involved.
func TestUnsafeModeAllowsAPlainPushToACriticalRef(t *testing.T) {
	registerFakeHost()
	policy := DefaultPolicy()
	policy.Mode = ModeUnsafe
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: worktree(t),
			ToolInput: map[string]any{"command": "git push origin main"}},
		policy, "feat/x")
	if decision.Deny {
		t.Fatalf("an unsafe-mode plain push to main is refused: %v", decision.Findings)
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
		decision := checkCommand(command, repo, DefaultPolicy())
		if !decision.Deny {
			t.Fatalf("%q is allowed; want it refused", command)
		}
		text := strings.Join(decision.Findings, "\n")
		if !strings.Contains(text, "TG-1") || !strings.Contains(text, strconv.Itoa(os.Getpid())) || !strings.Contains(text, "until") {
			t.Fatalf("%q findings = %q; want the group, pid and lapse time", command, text)
		}
	}
	for _, command := range []string{"git push origin feat/free", "git update-ref refs/komodo/feat/free HEAD"} {
		if decision := checkCommand(command, repo, DefaultPolicy()); decision.Deny {
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
	if decision := checkCommand("git push origin feat/held", repo, DefaultPolicy()); decision.Deny {
		t.Fatalf("the lease's own run is refused: %v", decision.Findings)
	}
	t.Setenv(lease.RunEnv, "")
	holder, _ := proc.Of(os.Getpid())
	if err := lease.Take(repo, "TG-1", "feat/held", holder, time.Now().Add(-lease.TTL-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if decision := checkCommand("git push origin feat/held", repo, DefaultPolicy()); decision.Deny {
		t.Fatalf("an expired lease refuses: %v", decision.Findings)
	}
}
