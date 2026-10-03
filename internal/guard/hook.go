package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"komodo/internal/git"
	"komodo/internal/mount"
)

// ExitDeny is the exit code a host reads as a refusal when it cannot read the JSON.
const ExitDeny = 2

// refusalLimit is how many identical refusals one line session may hit before the guard ends it.
const refusalLimit = 3

// lockTimeout bounds how long recordRefusal waits on another process's lock before giving up.
const lockTimeout = 2 * time.Second

// CurrentBranch is the branch a directory is on, falling back to a detached worktree's
// komodo.branch config, or the empty string when neither names one.
func CurrentBranch(dir string) string {
	if branch := git.TrackedBranch(dir); branch != "" {
		return branch
	}
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
	branch := CurrentBranch(request.Cwd)
	decision := Check(request, Load(toolkitRoot, root), branch)
	if !decision.Deny {
		return 0
	}
	reason := Reason(decision.Findings)
	blocked := false
	// Only a line session counts toward the refusal limit; the orchestrator is refused, never ended.
	if IsLineSession() {
		blocked, err = recordRefusal(root, request.SessionID, decision.rules)
		if err != nil {
			fmt.Fprintf(stderr, "guard: %v; allowing\n", err)
			return 0
		}
		if blocked {
			reason = blockedReason(decision.Findings)
		}
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

// recordRefusal counts one more refusal of each rule under session, locked across its own read
// and write; it reports whether any reached refusalLimit, and an I/O failure the caller must allow.
func recordRefusal(root, sessionID string, rules []string) (bool, error) {
	if root == "" || !filepath.IsLocal(sessionID) || sessionID != filepath.Base(sessionID) || len(rules) == 0 {
		return false, nil
	}
	path := filepath.Join(root, ".komodo", "runs", "guard-refusals", sessionID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	unlock, err := lockRefusals(path)
	if err != nil {
		return false, err
	}
	defer unlock()
	counts := map[string]int{}
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, &counts) != nil {
			counts = map[string]int{}
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	blocked := false
	for _, rule := range rules {
		counts[rule]++
		if counts[rule] >= refusalLimit {
			blocked = true
		}
	}
	data, err := json.Marshal(counts)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, err
	}
	return blocked, nil
}

// lockRefusals creates path's own lock file as the exclusive holder, retrying until it succeeds
// or lockTimeout passes, and returns the func that releases it.
func lockRefusals(path string) (func(), error) {
	lock := path + ".lock"
	deadline := time.Now().Add(lockTimeout)
	for {
		file, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			file.Close()
			return func() { os.Remove(lock) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("guard: timed out waiting for the lock on %s", path)
		}
		time.Sleep(time.Millisecond)
	}
}
