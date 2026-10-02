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

// Prune clears stale worktrees, landed refs/komodo tips and old runs, and only lists merged local branches; confirm gates the sweep.
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
		done = append(done, "merged local branch "+branch+"; komodo never deletes it, run git branch -d "+branch+" to")
	}
	done = append(done, pruneRuns(root, open)...)
	return done, nil
}

// settleShippedRun sweeps each clean worktree whose branch origin has merged, then each landed
// refs/komodo tip no worktree holds, skipping an open run's own. Unconfirmed, it only lists what it would take.
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
	held := map[string]bool{}
	for _, worktree := range stateWorktrees(root) {
		branch := TrackedOf(worktree)
		held[branch] = true
		if running[branch] || running[filepath.Clean(worktree.Path)] {
			continue
		}
		if status, err := git.Run(worktree.Path, "status", "--porcelain"); err != nil || status != "" {
			continue
		}
		tip := worktree.Head
		if ref := git.Or(root, "rev-parse", "--verify", "--quiet", line.TipRef(branch)); ref != "" {
			tip = ref
		}
		if !landed(root, branch, tip, remote) {
			continue
		}
		if !confirm {
			done = append(done, "would remove worktree "+rel(root, worktree.Path)+" and its ref "+line.TipRef(branch)+
				"; rerun with --confirm to delete")
			continue
		}
		if _, err := git.Run(root, "worktree", "remove", "--force", worktree.Path); err != nil {
			continue
		}
		done = append(done, "removed worktree "+rel(root, worktree.Path))
		if _, err := git.Run(root, "update-ref", "-d", line.TipRef(branch)); err == nil && worktree.Detached {
			done = append(done, "removed ref "+line.TipRef(branch))
		}
	}
	for _, ref := range lines(git.Run(root, "for-each-ref", "--format=%(refname)", "refs/komodo/")) {
		branch := strings.TrimPrefix(ref, "refs/komodo/")
		if branch == "" || held[branch] || running[branch] {
			continue
		}
		if !landed(root, branch, ref, remote) {
			continue
		}
		if !confirm {
			done = append(done, "would remove ref "+ref+"; rerun with --confirm to delete")
			continue
		}
		if _, err := git.Run(root, "update-ref", "-d", ref); err == nil {
			done = append(done, "removed ref "+ref)
		}
	}
	return done
}

// TrackedOf is the branch a worktree holds or, detached, the branch its komodo.branch config names.
func TrackedOf(worktree git.Worktree) string {
	if worktree.Branch != "" {
		return worktree.Branch
	}
	return worktree.Tracked
}

// landed reports whether branch was pushed (origin holds a remote-tracking ref for it) and tip is in remote.
func landed(root, branch, tip, remote string) bool {
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch); err != nil {
		return false
	}
	_, err := git.Run(root, "merge-base", "--is-ancestor", tip, remote)
	return err == nil
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

// stateWorktrees lists the worktrees under .komodo/wt that exist on disk and hold or track a branch.
func stateWorktrees(root string) []git.Worktree {
	worktrees, err := git.Worktrees(root)
	if err != nil {
		return nil
	}
	var found []git.Worktree
	for _, current := range worktrees {
		if TrackedOf(current) != "" && strings.Contains(current.Path, filepath.Join(".komodo", "wt")) && exists(current.Path) {
			found = append(found, current)
		}
	}
	return found
}
