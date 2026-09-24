package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/line"
)

// Prune removes stale worktrees, deletes merged branches, and settles a shipped run origin has merged.
func Prune(root, base string) ([]string, error) {
	var done []string
	worktrees, err := git.Worktrees(root)
	if err != nil {
		return nil, err
	}
	for _, worktree := range worktrees {
		path := worktree.Path
		if !strings.Contains(path, filepath.Join(".komodo", "wt")) {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if _, err := git.Run(root, "worktree", "remove", "--force", path); err == nil {
			done = append(done, "removed worktree "+rel(root, path))
		}
	}
	if _, err := git.Run(root, "worktree", "prune"); err == nil {
		done = append(done, "pruned the worktree list")
	}
	done = append(done, settleShippedRun(root, base)...)
	policy := guard.Load(root, root)
	merged := lines(git.Run(root, "branch", "--merged", base, "--format=%(refname:short)"))
	for _, branch := range merged {
		if branch == base || branch == "" || strings.HasPrefix(branch, "*") || policy.IsCritical(branch) {
			continue
		}
		if _, err := git.Run(root, "branch", "-d", branch); err == nil {
			done = append(done, "deleted merged branch "+branch)
		}
	}
	return done, nil
}

// settleShippedRun sweeps worktrees origin has merged, skipping while a run is open.
func settleShippedRun(root, base string) []string {
	if _, err := git.Run(root, "fetch", "--quiet", "origin", base); err != nil {
		return nil
	}
	remote := "origin/" + base
	if line.RunIsOpen(root) {
		return nil
	}
	var done []string
	for _, worktree := range stateWorktrees(root) {
		if status, err := git.Run(worktree.Path, "status", "--porcelain"); err != nil || status != "" {
			continue
		}
		if _, err := git.Run(root, "merge-base", "--is-ancestor", worktree.Branch, remote); err != nil {
			continue
		}
		if _, err := git.Run(root, "worktree", "remove", "--force", worktree.Path); err != nil {
			continue
		}
		done = append(done, "removed worktree "+rel(root, worktree.Path))
		if _, err := git.Run(root, "branch", "-D", worktree.Branch); err == nil {
			done = append(done, "deleted merged branch "+worktree.Branch)
		}
	}
	return done
}

// stateWorktrees lists the worktrees under .komodo/wt that exist on disk and hold a branch.
func stateWorktrees(root string) []git.Worktree {
	worktrees, err := git.Worktrees(root)
	if err != nil {
		return nil
	}
	var found []git.Worktree
	for _, current := range worktrees {
		if current.Branch != "" && strings.Contains(current.Path, filepath.Join(".komodo", "wt")) && exists(current.Path) {
			found = append(found, current)
		}
	}
	return found
}
