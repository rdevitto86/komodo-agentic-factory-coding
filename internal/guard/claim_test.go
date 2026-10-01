package guard

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/proc"
)

// init makes every claim test's host unknown, so only a test that fakes one exercises the host path.
func init() {
	hostProcess = func() (proc.Process, bool) { return proc.Process{}, false }
	processAlive = func(proc.Process) (bool, bool) { return false, false }
}

// fakeHost makes the claiming session's host me, and reads every process in live as running.
func fakeHost(t *testing.T, me proc.Process, live ...proc.Process) {
	t.Helper()
	oldHost, oldAlive := hostProcess, processAlive
	t.Cleanup(func() { hostProcess, processAlive = oldHost, oldAlive })
	hostProcess = func() (proc.Process, bool) { return me, true }
	processAlive = func(process proc.Process) (bool, bool) {
		for _, running := range live {
			if running == process {
				return true, true
			}
		}
		return false, true
	}
}

// claimRepo builds a real git repository on branch, so rev-parse --git-common-dir resolves.
func claimRepo(t *testing.T, branch string) string {
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

// editPayload is one hook call an Edit tool made, which the write-tool half of a claim gates.
func editPayload(root, sessionID string) string {
	return `{"hook_event_name":"PreToolUse","tool_name":"Edit","session_id":"` + sessionID +
		`","cwd":"` + root + `","tool_input":{"file_path":"` + filepath.Join(root, "a.go") + `"}}`
}

func TestHookClaimsABranchOnAWriteCall(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want the first session's write allowed: %s", code, errOut.String())
	}
	found, err := os.ReadFile(filepath.Join(root, ".git", "komodo-claims", "feat_x.json"))
	if err != nil {
		t.Fatalf("no claim file written: %v", err)
	}
	if !strings.Contains(string(found), "session-a") {
		t.Fatalf("claim = %s, want session-a", found)
	}
}

func TestHookRefusesAnotherSessionsLiveClaim(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want session-a's first write allowed", code)
	}
	out.Reset()
	errOut.Reset()
	denial := runHook(t, root, editPayload(root, "session-b"))
	if !strings.Contains(denial.Reason, "session-a") {
		t.Fatalf("reason = %q, want it to name session-a", denial.Reason)
	}
	if !strings.Contains(denial.Reason, "feat/x") {
		t.Fatalf("reason = %q, want it to name the branch", denial.Reason)
	}
	if !strings.Contains(denial.Reason, "your own branch") && !strings.Contains(denial.Reason, "wait") {
		t.Fatalf("reason = %q, want a way forward: switch or wait", denial.Reason)
	}
}

func TestHookLetsTheSameSessionRenewItsOwnClaim(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	for attempt := 1; attempt <= 2; attempt++ {
		var out, errOut strings.Builder
		if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
			t.Fatalf("attempt %d: exit = %d, want the same session's own claim renewed: %s", attempt, code, errOut.String())
		}
	}
}

func TestHookTakesAStaleClaimAndReportsItInsteadOfSilently(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	path := filepath.Join(root, ".git", "komodo-claims", "feat_x.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old, err := json.Marshal(claim{Session: "session-a", Branch: "feat/x", Started: time.Now().UTC().Add(-13 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-b")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want a stale claim taken, not refused: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "session-a") || !strings.Contains(errOut.String(), "stale") {
		t.Fatalf("stderr = %q, want a notice naming the old session as stale", errOut.String())
	}
	found, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(found), "session-b") {
		t.Fatalf("claim = %s, want session-b to now hold it", found)
	}
}

func TestHookSkipsClaimsForALineSession(t *testing.T) {
	registerFakeHost()
	t.Setenv(RoleEnv, "builder")
	root := claimRepo(t, "feat/x")
	for _, session := range []string{"builder-1", "builder-2"} {
		var out, errOut strings.Builder
		if code := Hook(root, strings.NewReader(editPayload(root, session)), &out, &errOut); code != 0 {
			t.Fatalf("session %s: exit = %d, want a line session's write never gated by a claim: %s", session, code, errOut.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "komodo-claims")); !os.IsNotExist(err) {
		t.Fatalf("a line session must never write a claim file: %v", err)
	}
}

// commitPayload is one hook call a git commit made, which the shell half of a claim gates too.
func commitPayload(root, sessionID string) string {
	return `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"` + sessionID +
		`","cwd":"` + root + `","tool_input":{"command":"git commit -m x"}}`
}

func TestHookClaimsABranchOnAGitCommitCall(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(commitPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want the first session's commit allowed: %s", code, errOut.String())
	}
	denial := runHook(t, root, commitPayload(root, "session-b"))
	if !strings.Contains(denial.Reason, "session-a") {
		t.Fatalf("reason = %q, want it to name session-a", denial.Reason)
	}
}

func TestHookFailsOpenOnAClaimIOError(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	// A file where the claims directory belongs makes MkdirAll fail underneath it.
	blocker := filepath.Join(root, ".git", "komodo-claims")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want a claim I/O error to fail open: %s", code, errOut.String())
	}
}

func TestHookOnlyGatesAClaimableCall(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Read","session_id":"session-a",` +
		`"cwd":"` + root + `","tool_input":{"file_path":"` + filepath.Join(root, "a.go") + `"}}`
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(payload), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want a read-only call allowed: %s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "komodo-claims")); !os.IsNotExist(err) {
		t.Fatal("a read-only call must never claim a branch")
	}
}

var (
	hostA = proc.Process{PID: 100, Start: "Wed Sep 30 10:00:00 2026"}
	hostB = proc.Process{PID: 200, Start: "Wed Sep 30 11:00:00 2026"}
)

func TestHookHandsAClaimToANewSessionOfTheSameHostProcess(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	fakeHost(t, hostA, hostA)
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want session-a's first write allowed", code)
	}
	errOut.Reset()
	if code := Hook(root, strings.NewReader(editPayload(root, "session-b")), &out, &errOut); code != 0 || out.Len() != 0 {
		t.Fatalf("exit = %d, out = %q; want a cleared session on the same host to take its own claim", code, out.String())
	}
	if !strings.Contains(errOut.String(), "same host process") {
		t.Fatalf("stderr = %q, want a notice saying why the claim moved", errOut.String())
	}
}

func TestHookTakesAClaimWhoseHostProcessEnded(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	fakeHost(t, hostA, hostA)
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want session-a's first write allowed", code)
	}
	fakeHost(t, hostB, hostB)
	errOut.Reset()
	if code := Hook(root, strings.NewReader(editPayload(root, "session-b")), &out, &errOut); code != 0 || out.Len() != 0 {
		t.Fatalf("exit = %d, out = %q; want a dead host's claim taken", code, out.String())
	}
	if !strings.Contains(errOut.String(), "outlived its host process") {
		t.Fatalf("stderr = %q, want a notice that the old host ended", errOut.String())
	}
}

func TestHookRefusesALiveHostsClaimAndNamesTheReleaseCommand(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	fakeHost(t, hostA, hostA, hostB)
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want session-a's first write allowed", code)
	}
	fakeHost(t, hostB, hostA, hostB)
	denial := runHook(t, root, editPayload(root, "session-b"))
	for _, want := range []string{"still running", "komodo guard release feat/x"} {
		if !strings.Contains(denial.Reason, want) {
			t.Fatalf("reason = %q, want %q", denial.Reason, want)
		}
	}
}

func TestHookKeysAWriteOnTheBranchOfTheWorktreeItTargets(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want session-a's first write allowed", code)
	}
	tree := filepath.Join(t.TempDir(), "tree")
	if result, err := exec.Command("git", "-C", root, "worktree", "add", "-q", "-b", "chore/y", tree).CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v: %s", err, result)
	}
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Edit","session_id":"session-b","cwd":"` + root +
		`","tool_input":{"file_path":"` + filepath.Join(tree, "new", "a.go") + `"}}`
	if code := Hook(root, strings.NewReader(payload), &out, &errOut); code != 0 || out.Len() != 0 {
		t.Fatalf("exit = %d, out = %q; want a write into another worktree held to that worktree's branch", code, out.String())
	}
}

func TestReleaseClaimFreesTheBranchForAnotherSession(t *testing.T) {
	registerFakeHost()
	root := claimRepo(t, "feat/x")
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(editPayload(root, "session-a")), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want session-a's first write allowed", code)
	}
	if session, err := ReleaseClaim(root, "feat/x"); err != nil || session != "session-a" {
		t.Fatalf("ReleaseClaim = %q, %v; want session-a released", session, err)
	}
	if code := Hook(root, strings.NewReader(editPayload(root, "session-b")), &out, &errOut); code != 0 || out.Len() != 0 {
		t.Fatalf("exit = %d, out = %q; want session-b free after the release", code, out.String())
	}
	if session, err := ReleaseClaim(root, "feat/none"); err != nil || session != "" {
		t.Fatalf("ReleaseClaim on no claim = %q, %v; want nothing released and no error", session, err)
	}
}

func TestPruneClaimsFreesOnlyADeadOrStaleClaim(t *testing.T) {
	root := claimRepo(t, "feat/x")
	fakeHost(t, hostA, hostA)
	for _, item := range []claim{
		{Session: "live", Branch: "feat/live", Started: time.Now().UTC(), Host: &hostA},
		{Session: "dead", Branch: "feat/dead", Started: time.Now().UTC(), Host: &hostB},
		{Session: "old", Branch: "feat/old", Started: time.Now().UTC().Add(-13 * time.Hour)},
	} {
		path, err := claimPath(root, item.Branch)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(item)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	done := strings.Join(PruneClaims(root), "\n")
	if !strings.Contains(done, "feat/dead") || !strings.Contains(done, "feat/old") || strings.Contains(done, "feat/live") {
		t.Fatalf("pruned = %q, want feat/dead and feat/old freed and feat/live kept", done)
	}
	if _, found, _ := readClaim(root, "feat/live"); !found {
		t.Fatal("a live host's claim must survive a prune")
	}
}
