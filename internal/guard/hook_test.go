package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pushPayload is one hook call denied for the same reason every time: a push to main.
func pushPayload(root, sessionID string) string {
	return `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"` + sessionID +
		`","cwd":"` + root + `","tool_input":{"command":"git push origin main"}}`
}

// hookDenialReason runs one hook call through a host that accepts a JSON denial and returns
// the reason it carried.
func hookDenialReason(t *testing.T, root, payload string) string {
	t.Helper()
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(payload), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0 with a JSON denial", code)
	}
	var decoded struct {
		Hook struct {
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("output is not JSON: %q", out.String())
	}
	return decoded.Hook.Reason
}

// TestHookEndsTheSessionOnTheThirdIdenticalRefusal proves the guard's own refusal limit (REQ-37):
// a session denied the same rule three times is told the session ends here, blocked.
func TestHookEndsTheSessionOnTheThirdIdenticalRefusal(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	payload := pushPayload(root, "session-a")
	for attempt := 1; attempt <= 2; attempt++ {
		if reason := hookDenialReason(t, root, payload); strings.Contains(reason, "blocked") {
			t.Fatalf("attempt %d ended the session early: %q", attempt, reason)
		}
	}
	reason := hookDenialReason(t, root, payload)
	if !strings.Contains(reason, "blocked") {
		t.Fatalf("the third identical refusal did not end the session: %q", reason)
	}
}

// TestHookRefusalNamesTheWayForward proves every refusal, blocked or not, still names the
// allowed alternative, so a session ended as blocked is not left without an answer.
func TestHookRefusalNamesTheWayForward(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	payload := pushPayload(root, "session-b")
	for attempt := 1; attempt <= 3; attempt++ {
		if reason := hookDenialReason(t, root, payload); !strings.Contains(reason, "open a pull request") {
			t.Fatalf("attempt %d named no way forward: %q", attempt, reason)
		}
	}
}

// TestHookFailsOpenWhenItCannotRecordARefusal proves a guard error on its own bookkeeping
// allows the call and logs it, rather than denying a call the counting step could not track.
func TestHookFailsOpenWhenItCannotRecordARefusal(t *testing.T) {
	registerFakeHost()
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
