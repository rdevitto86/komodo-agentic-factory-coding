package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ExitDeny is the exit code a host reads as a refusal when it cannot read the JSON.
const ExitDeny = 2

// CurrentBranch is the branch a directory is on, or the empty string.
func CurrentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Hook reads one payload, writes the denial both hosts accept, and returns the exit code.
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
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": Reason(decision.Findings),
		},
	}
	out, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(stderr, Reason(decision.Findings))
		return ExitDeny
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintln(stderr, Reason(decision.Findings))
		return ExitDeny
	}
	return 0
}
