package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/git"
	"komodo/internal/mount"
)

// ClaimStale is how long a branch claim stands before a session must report it stale, not trust it.
const ClaimStale = 12 * time.Hour

// claim names the session and time that last claimed a shared branch.
type claim struct {
	Session string    `json:"session"`
	Branch  string    `json:"branch"`
	Started time.Time `json:"started"`
}

// stale reports whether the claim is older than ClaimStale as of now.
func (c claim) stale(now time.Time) bool { return now.Sub(c.Started) > ClaimStale }

// errClaimed reports another session's live claim on the branch a write targeted.
var errClaimed = errors.New("another session holds a live claim on this branch")

// claimVerbs are the git subcommands that write a branch's history, the ones a claim gates.
var claimVerbs = map[string]bool{"commit": true, "merge": true, "push": true, "reset": true, "rebase": true}

// ClaimDecision is what a session's claim on the current branch allowed, refused, or reported.
type ClaimDecision struct {
	Deny    bool
	Finding string
	Notice  string
}

// ClaimCheck enforces one branch's session claim for the orchestrator tier only; a line session
// skips it, since the conductor already serializes a group branch across its own sessions.
func ClaimCheck(request Request, root, branch, sessionID string) ClaimDecision {
	if IsLineSession() || root == "" || branch == "" || !claimableCall(request) {
		return ClaimDecision{}
	}
	stale, err := writeClaim(root, branch, sessionID)
	if err != nil {
		if errors.Is(err, errClaimed) {
			existing, _, readErr := readClaim(root, branch)
			if readErr != nil {
				return ClaimDecision{}
			}
			return ClaimDecision{Deny: true, Finding: fmt.Sprintf(
				"%s claimed %s at %s; switch to your own branch, or wait for it to finish or go stale",
				existing.Session, branch, existing.Started.Format(time.RFC3339))}
		}
		// A claim the guard cannot read or write never blocks the call it would have gated.
		return ClaimDecision{}
	}
	if stale != nil {
		return ClaimDecision{Notice: fmt.Sprintf(
			"%s's claim on %s from %s was stale; %s now holds it",
			stale.Session, branch, stale.Started.Format(time.RFC3339), sessionID)}
	}
	return ClaimDecision{}
}

// claimableCall reports whether request is a write tool call, or a shell call running a git verb
// claimVerbs names, looking through wrappers, shell -c, and eval the way the rest of the guard does.
func claimableCall(request Request) bool {
	for _, tools := range mount.GuardHosts() {
		if tools.WriteTools[request.ToolName] {
			return true
		}
		if tools.ShellTool != "" && request.ToolName == tools.ShellTool &&
			claimableCommand(stringField(request.ToolInput, tools.CommandField)) {
			return true
		}
	}
	return false
}

// claimableCommand reports whether command runs git commit, merge, push, reset, or rebase.
func claimableCommand(command string) bool {
	for _, item := range nested(command) {
		if len(item.words) == 0 || commandName(item.words[0]) != "git" {
			continue
		}
		args := skipGlobalFlags(item.words[1:])
		if len(args) > 0 && claimVerbs[args[0]] {
			return true
		}
	}
	return false
}

// claimPath is where branch's claim file lives, under the git common dir so every worktree of the
// same repository sees it, not only the one that wrote it.
func claimPath(root, branch string) (string, error) {
	dir, err := commonDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "komodo-claims", strings.ReplaceAll(branch, "/", "_")+".json"), nil
}

// commonDir is the directory git shares across every worktree of one repository.
func commonDir(root string) (string, error) {
	out, err := git.Run(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) {
		return out, nil
	}
	return filepath.Join(root, out), nil
}

// readClaim reads branch's claim file, and reports whether one exists.
func readClaim(root, branch string) (claim, bool, error) {
	path, err := claimPath(root, branch)
	if err != nil {
		return claim{}, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return claim{}, false, nil
	}
	if err != nil {
		return claim{}, false, err
	}
	var found claim
	if err := json.Unmarshal(data, &found); err != nil {
		return claim{}, false, err
	}
	return found, true, nil
}

// writeClaim claims branch for session, refusing a live claim another session holds; a stale claim
// is overwritten, and returned so the caller reports it, never taking it silently.
func writeClaim(root, branch, session string) (staleClaim *claim, err error) {
	if session == "" {
		return nil, fmt.Errorf("a claim names the session; got none")
	}
	existing, found, err := readClaim(root, branch)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if found && existing.Session != session {
		if !existing.stale(now) {
			return nil, errClaimed
		}
		staleClaim = &existing
	}
	path, err := claimPath(root, branch)
	if err != nil {
		return staleClaim, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return staleClaim, err
	}
	data, err := json.Marshal(claim{Session: session, Branch: branch, Started: now})
	if err != nil {
		return staleClaim, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return staleClaim, err
	}
	return staleClaim, nil
}
