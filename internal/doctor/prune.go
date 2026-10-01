package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/line"
)

// keptRuns is how many run folders Prune keeps, the newest by start; a starting value.
const keptRuns = 10

// Prune removes stale worktrees, settles each merged or dropped group, and keeps only the newest run
// folders; confirm must be true before it deletes a branch, else it only lists what it would delete.
func Prune(root, base string, confirm bool) ([]string, error) {
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
	open := line.OpenRuns(root)
	done = append(done, settleShippedRun(root, base, open, confirm)...)
	policy := guard.Load(root, root)
	merged := lines(git.Run(root, "branch", "--merged", base, "--format=%(refname:short)"))
	for _, branch := range merged {
		if branch == base || branch == "" || strings.HasPrefix(branch, "*") || policy.IsCritical(branch) {
			continue
		}
		if !confirm {
			done = append(done, "would delete merged branch "+branch+"; rerun with --confirm to delete")
			continue
		}
		if _, err := git.Run(root, "branch", "-d", branch); err == nil {
			done = append(done, "deleted merged branch "+branch)
		}
	}
	done = append(done, pruneRuns(root, open)...)
	done = append(done, guard.PruneClaims(root)...)
	return done, nil
}

// settleShippedRun sweeps each clean worktree origin has merged or whose pushed branch origin dropped,
// skipping an open run's own worktree. Unconfirmed, it only lists the worktree and branch it would take.
func settleShippedRun(root, base string, open []line.RunState, confirm bool) []string {
	if _, err := git.Run(root, "fetch", "--quiet", "origin", base); err != nil {
		return nil
	}
	remote := "origin/" + base
	running := map[string]bool{}
	for _, state := range open {
		running[state.Branch] = true
		running[filepath.Clean(line.WorktreePath(root, state.Worktree))] = true
	}
	var done []string
	for _, worktree := range stateWorktrees(root) {
		if running[worktree.Branch] || running[filepath.Clean(worktree.Path)] {
			continue
		}
		if status, err := git.Run(worktree.Path, "status", "--porcelain"); err != nil || status != "" {
			continue
		}
		if !landed(root, worktree.Branch, remote) {
			continue
		}
		if !confirm {
			done = append(done, "would remove worktree "+rel(root, worktree.Path)+" and delete branch "+
				worktree.Branch+"; rerun with --confirm to delete")
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

// landed reports whether branch is in remote, or was pushed with an upstream origin has since deleted,
// as a squash merge or an abandoned pull request leaves it.
func landed(root, branch, remote string) bool {
	if _, err := git.Run(root, "merge-base", "--is-ancestor", branch, remote); err == nil {
		return true
	}
	upstream, err := git.Run(root, "config", "--get", "branch."+branch+".merge")
	if err != nil || upstream == "" {
		return false
	}
	heads, err := git.Run(root, "ls-remote", "--heads", "origin", upstream)
	return err == nil && heads == ""
}

// pruneRuns removes every run folder older than the newest keptRuns, never an open run's.
func pruneRuns(root string, open []line.RunState) []string {
	runs := line.LoadRuns(root)
	if len(runs) <= keptRuns {
		return nil
	}
	running := map[string]bool{}
	for _, state := range open {
		running[state.Group] = true
	}
	var done []string
	for _, state := range runs[:len(runs)-keptRuns] {
		if running[state.Group] {
			continue
		}
		dir := line.RunDir(root, state.Group)
		if err := os.RemoveAll(dir); err == nil {
			done = append(done, "removed run folder "+rel(root, dir))
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
