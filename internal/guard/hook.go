package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"komodo/internal/mount"
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
	if tools, ok := hostGuard(request.ToolName); ok && tools.Deny != nil {
		if out := tools.Deny(reason); out != nil {
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
		if tools.WriteTools[toolName] || (tools.ShellTool != "" && tools.ShellTool == toolName) {
			return tools, true
		}
	}
	return mount.GuardTools{}, false
}
