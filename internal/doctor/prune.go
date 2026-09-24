package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/guard"
	"komodo/internal/line"
)

// Prune removes stale worktrees, deletes merged branches, and settles a shipped run origin has merged.
func Prune(root, base string) ([]string, error) {
	var done []string
	out, err := git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		path, found := strings.CutPrefix(strings.TrimSpace(line), "worktree ")
		if !found || !strings.Contains(path, filepath.Join(".komodo", "wt")) {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if _, err := git(root, "worktree", "remove", "--force", path); err == nil {
			done = append(done, "removed worktree "+rel(root, path))
		}
	}
	if _, err := git(root, "worktree", "prune"); err == nil {
		done = append(done, "pruned the worktree list")
	}
	done = append(done, settleShippedRun(root, base)...)
	policy := guard.Load(root, root)
	merged := lines(git(root, "branch", "--merged", base, "--format=%(refname:short)"))
	for _, branch := range merged {
		if branch == base || branch == "" || strings.HasPrefix(branch, "*") || policy.IsCritical(branch) {
			continue
		}
		if _, err := git(root, "branch", "-d", branch); err == nil {
			done = append(done, "deleted merged branch "+branch)
		}
	}
	return done, nil
}

// settleShippedRun sweeps worktrees origin has merged, skipping while a run is open.
func settleShippedRun(root, base string) []string {
	if _, err := git(root, "fetch", "--quiet", "origin", base); err != nil {
		return nil
	}
	remote := "origin/" + base
	if line.RunIsOpen(root) {
		return nil
	}
	var done []string
	for _, worktree := range stateWorktrees(root) {
		if status, err := git(worktree.path, "status", "--porcelain"); err != nil || status != "" {
			continue
		}
		if _, err := git(root, "merge-base", "--is-ancestor", worktree.branch, remote); err != nil {
			continue
		}
		if _, err := git(root, "worktree", "remove", "--force", worktree.path); err != nil {
			continue
		}
		done = append(done, "removed worktree "+rel(root, worktree.path))
		if _, err := git(root, "branch", "-D", worktree.branch); err == nil {
			done = append(done, "deleted merged branch "+worktree.branch)
		}
	}
	return done
}

// stateWorktree is one worktree under the state directory and the branch it has checked out.
type stateWorktree struct{ path, branch string }

// stateWorktrees lists the worktrees under .komodo/wt that exist on disk and hold a branch.
func stateWorktrees(root string) []stateWorktree {
	out, err := git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	var found []stateWorktree
	for _, block := range strings.Split(out, "\n\n") {
		var current stateWorktree
		for _, field := range strings.Split(block, "\n") {
			if path, ok := strings.CutPrefix(field, "worktree "); ok {
				current.path = path
			}
			if ref, ok := strings.CutPrefix(field, "branch refs/heads/"); ok {
				current.branch = ref
			}
		}
		if current.branch != "" && strings.Contains(current.path, filepath.Join(".komodo", "wt")) && exists(current.path) {
			found = append(found, current)
		}
	}
	return found
}
