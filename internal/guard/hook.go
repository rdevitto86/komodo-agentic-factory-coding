package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/git"
	"komodo/internal/mount"
)

// ExitDeny is the exit code a host reads as a refusal when it cannot read the JSON.
const ExitDeny = 2

// refusalLimit is how many identical refusals one session may hit before the guard ends it (REQ-37).
const refusalLimit = 3

// CurrentBranch is the branch a directory is on, or the empty string.
func CurrentBranch(dir string) string {
	return git.Or(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// Hook reads one payload, writes the denial the matching host reads, and returns the exit code.
func Hook(toolkitRoot string, stdin io.Reader, stdout, stderr io.Writer) int {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintln(stderr, "guard: unreadable payload; allowing")
		return 0
	}
	var request Request
	if err := json.Unmarshal(raw, &request); err != nil {
		fmt.Fprintln(stderr, "guard: unparseable payload; allowing")
		return 0
	}
	if request.Cwd == "" {
		request.Cwd, _ = os.Getwd()
	}
	root := WorktreeRoot(request.Cwd)
	decision := Check(request, Load(toolkitRoot, root), CurrentBranch(request.Cwd))
	if !decision.Deny {
		return 0
	}
	reason := Reason(decision.Findings)
	var sessionPayload struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(raw, &sessionPayload)
	blocked, err := recordRefusal(root, sessionPayload.SessionID, strings.Join(decision.Findings, "|"))
	if err != nil {
		fmt.Fprintf(stderr, "guard: %v; allowing\n", err)
		return 0
	}
	if blocked {
		reason = blockedReason(decision.Findings)
	}
	if tools, ok := hostGuard(request.ToolName); ok && tools.Deny != nil {
		if out := tools.Deny(reason, blocked); out != nil {
			if _, err := stdout.Write(out); err == nil {
				return 0
			}
		}
	}
	fmt.Fprintln(stderr, reason)
	return ExitDeny
}

// hostGuard returns the registered mount whose write or shell tool names this request's tool.
func hostGuard(toolName string) (mount.GuardTools, bool) {
	for _, tools := range mount.GuardHosts() {
		if tools.WriteTools[toolName] || (tools.ShellTool != "" && tools.ShellTool == toolName) || tools.SpawnTools[toolName] {
			return tools, true
		}
	}
	return mount.GuardTools{}, false
}

// blockedReason renders the denial once a session has hit the same refusal refusalLimit times,
// so the model never tries variant after variant of a rule it has already lost.
func blockedReason(findings []string) string {
	return Reason(findings) + fmt.Sprintf(
		"\n\nThis is refusal %d of the same rule. The session ends here, blocked.", refusalLimit)
}

// recordRefusal counts one more refusal of rule under session in the run folder, reporting
// whether this is the refusalLimit-th; err is an I/O failure the caller must allow the call for.
func recordRefusal(root, sessionID, rule string) (bool, error) {
	if root == "" || sessionID == "" {
		return false, nil
	}
	path := filepath.Join(root, ".komodo", "runs", "guard-refusals", sessionID+".json")
	counts := map[string]int{}
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, &counts) != nil {
			counts = map[string]int{}
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	counts[rule]++
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	data, err := json.Marshal(counts)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, err
	}
	return counts[rule] >= refusalLimit, nil
}
