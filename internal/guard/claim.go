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
	"komodo/internal/proc"
)

// ClaimStale is how long a branch claim stands before a session must report it stale, not trust it.
const ClaimStale = 12 * time.Hour

// claim names the session, time, and host process that last claimed a shared branch.
type claim struct {
	Session string        `json:"session"`
	Branch  string        `json:"branch"`
	Started time.Time     `json:"started"`
	Host    *proc.Process `json:"host,omitempty"`
}

// hostProcess and processAlive find and test a session's host process; tests swap them for a fake.
var (
	hostProcess  = proc.Host
	processAlive = proc.Alive
)

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
	taken, reason, err := writeClaim(root, branch, sessionID)
	if err != nil {
		if errors.Is(err, errClaimed) {
			existing, _, readErr := readClaim(root, branch)
			if readErr != nil {
				return ClaimDecision{}
			}
			return ClaimDecision{Deny: true, Finding: refusal(existing)}
		}
		// A claim the guard cannot read or write never blocks the call it would have gated.
		return ClaimDecision{}
	}
	if taken != nil {
		return ClaimDecision{Notice: fmt.Sprintf(
			"%s's claim on %s from %s %s; %s now holds it",
			taken.Session, branch, taken.Started.Format(time.RFC3339), reason, sessionID)}
	}
	return ClaimDecision{}
}

// refusal names a live claim's owner, whether its host still runs, and every way past it.
func refusal(existing claim) string {
	owner := "komodo cannot tell whether its session is still running"
	if existing.Host != nil {
		if alive, known := processAlive(*existing.Host); known && alive {
			owner = fmt.Sprintf("its host process %d is still running", existing.Host.PID)
		}
	}
	return fmt.Sprintf("%s claimed %s at %s and %s; switch to your own branch, wait until %s when it goes stale, "+
		"or release it with `komodo guard release %s`",
		existing.Session, existing.Branch, existing.Started.Format(time.RFC3339), owner,
		existing.Started.Add(ClaimStale).Format(time.RFC3339), existing.Branch)
}

// takeover says why a session may take a claim another session holds, or "" when it may not; host is
// the claiming session's host process, nil when unknown.
func takeover(existing claim, now time.Time, host *proc.Process) string {
	if existing.stale(now) {
		return "was stale"
	}
	if existing.Host == nil {
		return ""
	}
	if host != nil && *host == *existing.Host {
		return "was left by an earlier session of the same host process"
	}
	if alive, known := processAlive(*existing.Host); known && !alive {
		return "outlived its host process"
	}
	return ""
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

// writeClaim claims branch for session, refusing a live claim another session holds; a claim it takes
// over is returned with the reason, so the caller reports it, never taking it silently.
func writeClaim(root, branch, session string) (taken *claim, reason string, err error) {
	if session == "" {
		return nil, "", fmt.Errorf("a claim names the session; got none")
	}
	existing, found, err := readClaim(root, branch)
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	host := existing.Host
	if !found || existing.Session != session {
		host = nil
		if process, ok := hostProcess(); ok {
			host = &process
		}
	}
	if found && existing.Session != session {
		reason = takeover(existing, now, host)
		if reason == "" {
			return nil, "", errClaimed
		}
		taken = &existing
	}
	path, err := claimPath(root, branch)
	if err != nil {
		return taken, reason, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return taken, reason, err
	}
	data, err := json.Marshal(claim{Session: session, Branch: branch, Started: now, Host: host})
	if err != nil {
		return taken, reason, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return taken, reason, err
	}
	return taken, reason, nil
}

// ReleaseClaim removes branch's claim and returns the session that held it, or "" when none did.
func ReleaseClaim(root, branch string) (string, error) {
	existing, found, err := readClaim(root, branch)
	if err != nil || !found {
		return "", err
	}
	path, err := claimPath(root, branch)
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return existing.Session, nil
}

// PruneClaims removes every claim that is stale or whose host process has ended, and names each one.
func PruneClaims(root string) []string {
	dir, err := commonDir(root)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(dir, "komodo-claims"))
	if err != nil {
		return nil
	}
	now := time.Now().UTC()
	var done []string
	for _, entry := range entries {
		path := filepath.Join(dir, "komodo-claims", entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var found claim
		if json.Unmarshal(data, &found) != nil {
			continue
		}
		reason := takeover(found, now, nil)
		if reason == "" {
			continue
		}
		if os.Remove(path) == nil {
			done = append(done, fmt.Sprintf("released the claim on %s, which %s", found.Branch, reason))
		}
	}
	return done
}

// claimDir is the directory whose branch a call's claim keys on: a write tool's target, else the cwd,
// so a write into another worktree is held to that worktree's branch.
func claimDir(request Request) string {
	for _, tools := range mount.GuardHosts() {
		if !tools.WriteTools[request.ToolName] {
			continue
		}
		for _, field := range tools.PathFields {
			path := stringField(request.ToolInput, field)
			if path == "" {
				continue
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(request.Cwd, path)
			}
			return existingDir(filepath.Dir(path))
		}
	}
	return request.Cwd
}

// existingDir walks up from dir to the nearest directory on disk, since a write may create its parents.
func existingDir(dir string) string {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}
