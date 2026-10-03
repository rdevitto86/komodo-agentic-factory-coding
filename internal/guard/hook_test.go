package guard

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// pushPayload is one hook call denied for the same reason every time: a push to main.
func pushPayload(root, sessionID string) string {
	return `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"` + sessionID +
		`","cwd":"` + root + `","tool_input":{"command":"git push origin main"}}`
}

// hookDenial is one hook call's decoded JSON denial: its reason and any stop signal.
type hookDenial struct {
	Reason     string
	Continue   *bool
	StopReason string
}

// runHook runs one hook call through a host that accepts a JSON denial and decodes it.
func runHook(t *testing.T, root, payload string) hookDenial {
	t.Helper()
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(payload), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0 with a JSON denial", code)
	}
	var decoded struct {
		Hook struct {
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
		Continue   *bool  `json:"continue"`
		StopReason string `json:"stopReason"`
	}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("output is not JSON: %q", out.String())
	}
	return hookDenial{Reason: decoded.Hook.Reason, Continue: decoded.Continue, StopReason: decoded.StopReason}
}

// hookDenialReason runs one hook call through a host that accepts a JSON denial and returns
// the reason it carried.
func hookDenialReason(t *testing.T, root, payload string) string {
	t.Helper()
	return runHook(t, root, payload).Reason
}

// TestHookEndsTheSessionOnTheThirdIdenticalRefusal proves the guard's own refusal limit (REQ-37):
// a session denied the same rule three times is told the session ends here, blocked.
func TestHookEndsTheSessionOnTheThirdIdenticalRefusal(t *testing.T) {
	registerFakeHost()
	t.Setenv(RoleEnv, "builder")
	root := worktree(t)
	payload := pushPayload(root, "session-a")
	for attempt := 1; attempt <= 2; attempt++ {
		if denial := runHook(t, root, payload); denial.Continue != nil && !*denial.Continue {
			t.Fatalf("attempt %d ended the session early: %+v", attempt, denial)
		}
	}
	denial := runHook(t, root, payload)
	if denial.Continue == nil || *denial.Continue {
		t.Fatalf("the third identical refusal did not set continue false: %+v", denial)
	}
	if denial.StopReason == "" {
		t.Fatalf("the third identical refusal carried no stopReason: %+v", denial)
	}
	if !strings.Contains(denial.Reason, "blocked") {
		t.Fatalf("the third identical refusal did not name itself blocked: %q", denial.Reason)
	}
}

// pushPayloadWithForce is one hook call denied for the same push-to-main rule, with an extra
// force-rewrite finding the plain push never carries.
func pushPayloadWithForce(root, sessionID string) string {
	return `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"` + sessionID +
		`","cwd":"` + root + `","tool_input":{"command":"git push -f origin main"}}`
}

// TestHookCountsARefusalByItsRuleNotEveryFindingTogether proves a forced push to main still adds
// to the plain push-to-main count, rather than starting a key of its own for the extra finding.
func TestHookCountsARefusalByItsRuleNotEveryFindingTogether(t *testing.T) {
	registerFakeHost()
	t.Setenv(RoleEnv, "builder")
	root := worktree(t)
	plain := pushPayload(root, "session-mix")
	forced := pushPayloadWithForce(root, "session-mix")
	for _, payload := range []string{plain, forced} {
		if denial := runHook(t, root, payload); denial.Continue != nil && !*denial.Continue {
			t.Fatalf("a refusal ended the session before the limit: %+v", denial)
		}
	}
	denial := runHook(t, root, plain)
	if denial.Continue == nil || *denial.Continue {
		t.Fatalf("the third refusal of the same rule, split across two commands, did not end the session: %+v", denial)
	}
}

// TestHookNeverEndsTheOrchestratorsSession proves the refusal limit is the line's: a session with
// no role is refused every time, but never told to stop.
func TestHookNeverEndsTheOrchestratorsSession(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	payload := pushPayload(root, "session-orchestrator")
	for attempt := 1; attempt <= refusalLimit+1; attempt++ {
		denial := runHook(t, root, payload)
		if denial.Continue != nil && !*denial.Continue {
			t.Fatalf("attempt %d ended the orchestrator's session: %+v", attempt, denial)
		}
		if !strings.Contains(denial.Reason, "open a pull request") {
			t.Fatalf("attempt %d was not refused with the way forward: %q", attempt, denial.Reason)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".komodo", "runs", "guard-refusals", "session-orchestrator.json")); !os.IsNotExist(err) {
		t.Fatalf("the orchestrator's refusals were counted: %v", err)
	}
}

// TestHookRefusalNamesTheWayForward proves every refusal, blocked or not, still names the
// allowed alternative, so a session ended as blocked is not left without an answer.
func TestHookRefusalNamesTheWayForward(t *testing.T) {
	registerFakeHost()
	t.Setenv(RoleEnv, "builder")
	root := worktree(t)
	payload := pushPayload(root, "session-b")
	for attempt := 1; attempt <= 3; attempt++ {
		if reason := hookDenialReason(t, root, payload); !strings.Contains(reason, "open a pull request") {
			t.Fatalf("attempt %d named no way forward: %q", attempt, reason)
		}
	}
}

// TestASessionIDThatClimbsOutIsNeverAPath proves a session ID with a separator or .. records nothing.
func TestASessionIDThatClimbsOutIsNeverAPath(t *testing.T) {
	root := worktree(t)
	for _, id := range []string{"../../../../escaped", "../escaped", "a/b", ".."} {
		if last, err := recordRefusal(root, id, []string{"push"}); last || err != nil {
			t.Fatalf("recordRefusal(%q) = %v, %v, want nothing recorded", id, last, err)
		}
	}
	for _, path := range []string{
		filepath.Join(filepath.Dir(root), "escaped.json"),
		filepath.Join(root, ".komodo", "runs", "escaped.json"),
		filepath.Join(root, ".komodo", "runs", "guard-refusals", "a"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("a refusal count landed at %s: %v", path, err)
		}
	}
}

// TestConcurrentRefusalsNeverLoseAnIncrement proves recordRefusal holds a lock across its own
// read and write, so parallel calls for one session and rule all land, none lost to a stale read.
func TestConcurrentRefusalsNeverLoseAnIncrement(t *testing.T) {
	root := worktree(t)
	const calls = 20
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := recordRefusal(root, "session-concurrent", []string{"rule"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	data, err := os.ReadFile(filepath.Join(root, ".komodo", "runs", "guard-refusals", "session-concurrent.json"))
	if err != nil {
		t.Fatal(err)
	}
	var counts map[string]int
	if err := json.Unmarshal(data, &counts); err != nil {
		t.Fatal(err)
	}
	if counts["rule"] != calls {
		t.Fatalf("count = %d, want all %d concurrent calls counted", counts["rule"], calls)
	}
}

// runGit runs one git command in dir, failing the test on any error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// TestCurrentBranchFallsBackToTheTrackedBranchInADetachedWorktree proves CurrentBranch reads a
// detached worktree's komodo.branch config, since rev-parse --abbrev-ref names no branch there.
func TestCurrentBranchFallsBackToTheTrackedBranchInADetachedWorktree(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "commit", "--allow-empty", "-q", "-m", "seed")
	runGit(t, root, "config", "extensions.worktreeConfig", "true")
	path := t.TempDir() + "-wt"
	runGit(t, root, "worktree", "add", "--detach", path)
	if got := CurrentBranch(path); got != "HEAD" {
		t.Fatalf("CurrentBranch before any komodo.branch config = %q, want HEAD", got)
	}
	runGit(t, path, "config", "--worktree", "komodo.branch", "feat/x")
	if got := CurrentBranch(path); got != "feat/x" {
		t.Fatalf("CurrentBranch = %q, want the worktree's tracked branch feat/x", got)
	}
}

// TestHookFailsOpenWhenItCannotRecordARefusal proves a guard error on its own bookkeeping
// allows the call and logs it, rather than denying a call the counting step could not track.
func TestHookFailsOpenWhenItCannotRecordARefusal(t *testing.T) {
	registerFakeHost()
	t.Setenv(RoleEnv, "builder")
	root := worktree(t)
	// A file where the refusals directory belongs makes MkdirAll fail underneath it.
	blocker := filepath.Join(root, ".komodo", "runs")
	if err := os.MkdirAll(filepath.Dir(blocker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := pushPayload(root, "session-c")
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(payload), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0 when the guard cannot record its own refusal", code)
	}
	if !strings.Contains(errOut.String(), "allowing") {
		t.Fatalf("stderr = %q, want a log of the guard's own failure", errOut.String())
	}
}
