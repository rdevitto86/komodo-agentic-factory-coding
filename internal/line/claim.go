package line

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/git"
)

// ClaimStale is how long a claim stands before a session must report it stale instead of trusting it.
const ClaimStale = 12 * time.Hour

// Claim names the session and time that last wrote to a shared branch.
type Claim struct {
	Session string    `json:"session"`
	Branch  string    `json:"branch"`
	Started time.Time `json:"started"`
}

// Stale reports whether the claim is older than ClaimStale as of now.
func (c Claim) Stale(now time.Time) bool {
	return now.Sub(c.Started) > ClaimStale
}

// ErrClaimed reports that another session holds a live claim on the branch a write targeted.
var ErrClaimed = errors.New("another session holds a live claim on this branch")

// ClaimPath is where branch's claim file lives, under the git common dir so every worktree of the
// same repository sees it, not only the one that wrote it.
func ClaimPath(root, branch string) (string, error) {
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

// ReadClaim reads branch's claim file, and reports whether one exists.
func ReadClaim(root, branch string) (Claim, bool, error) {
	path, err := ClaimPath(root, branch)
	if err != nil {
		return Claim{}, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Claim{}, false, nil
	}
	if err != nil {
		return Claim{}, false, err
	}
	var claim Claim
	if err := json.Unmarshal(data, &claim); err != nil {
		return Claim{}, false, err
	}
	return claim, true, nil
}

// WriteClaim claims branch for session, refusing a live claim another session holds; a stale claim
// is overwritten, and returned so the caller reports it, never taking it silently.
func WriteClaim(root, branch, session string) (stale *Claim, err error) {
	if session == "" {
		return nil, fmt.Errorf("a claim names the session; got none")
	}
	existing, found, err := ReadClaim(root, branch)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if found && existing.Session != session {
		if !existing.Stale(now) {
			return nil, fmt.Errorf("%w: %s claimed %s at %s", ErrClaimed, existing.Session, branch, existing.Started.Format(time.RFC3339))
		}
		stale = &existing
	}
	path, err := ClaimPath(root, branch)
	if err != nil {
		return stale, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return stale, err
	}
	data, err := json.Marshal(Claim{Session: session, Branch: branch, Started: now})
	if err != nil {
		return stale, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return stale, err
	}
	return stale, nil
}

// ReleaseClaim removes branch's claim file, once the session holding it finishes with the branch.
func ReleaseClaim(root, branch string) error {
	path, err := ClaimPath(root, branch)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
