package guard

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
