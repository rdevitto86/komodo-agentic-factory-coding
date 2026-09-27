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
