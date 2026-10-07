package gate

import (
	"os"
	"path/filepath"

	"komodo/internal/git"
)

// CleanCheckout is root when it is a clean tree at commit, else a temporary detached worktree there;
// cleanup removes any worktree it made.
func CleanCheckout(root, commit string) (dir string, cleanup func(), err error) {
	noop := func() {}
	want, err := git.Run(root, "rev-parse", "--verify", commit+"^{commit}")
	if err != nil {
		return "", noop, err
	}
	status, err := git.Run(root, "status", "--porcelain")
	if err != nil {
		return "", noop, err
	}
	if status == "" && git.Or(root, "rev-parse", "HEAD") == want {
		return root, noop, nil
	}
	parent, err := os.MkdirTemp("", "komodo-gate-")
	if err != nil {
		return "", noop, err
	}
	dir = filepath.Join(parent, filepath.Base(root))
	// No hook runs for the checkout itself, so it never rebuilds or gates anything.
	if _, err := git.Run(root, "-c", "core.hooksPath="+os.DevNull, "worktree", "add", "--detach", dir, want); err != nil {
		_ = os.RemoveAll(parent)
		return "", noop, err
	}
	return dir, func() {
		_, _ = git.Run(root, "worktree", "remove", "--force", dir)
		_ = os.RemoveAll(parent)
		_, _ = git.Run(root, "worktree", "prune")
	}, nil
}
