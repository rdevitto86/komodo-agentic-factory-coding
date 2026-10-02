package doctor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/lease"
	"komodo/internal/line"
	"komodo/internal/pr"
)

// keptRuns is how many run folders Prune keeps, the newest by start; a starting value.
const keptRuns = 10

// idleFor is how long a worktree's git state must sit unchanged before its pushed work alone lets prune take it.
const idleFor = 24 * time.Hour

// now is the clock the idle and lease checks read; tests swap it.
var now = time.Now

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
	done = append(done, pruneClaims(root)...)
	return done, nil
}

// claimsDir is where the guard kept branch claims until they were removed; nothing reads it now.
const claimsDir = "komodo-claims"

// pruneClaims deletes the shared git dir's dead branch-claim directory.
func pruneClaims(root string) []string {
	common, err := git.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil
	}
	dir := filepath.Join(common, claimsDir)
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return []string{"could not remove the dead branch claims at " + dir + ": " + err.Error()}
	}
	return []string{"removed the dead branch claims at " + dir}
}

// settleShippedRun sweeps clean, unleased, landed worktrees and orphan landed tips, never an open run's; unconfirmed, it lists.
func settleShippedRun(root, base string, open []line.RunState, confirm bool) []string {
	if _, err := git.Run(root, "fetch", "--quiet", "--prune", "origin"); err != nil {
		return []string{"skipped the worktree sweep: origin did not answer"}
	}
	remote := "origin/" + base
	running := map[string]bool{}
	for _, state := range open {
		running[state.Branch] = true
		running[filepath.Clean(line.WorktreePath(root, state.Worktree))] = true
		for _, wave := range state.Waves {
			for _, task := range wave {
				running[line.TaskBranch(task)] = true
				running[filepath.Join(root, line.StateDir, "wt", task)] = true
			}
		}
	}
	var done []string
	held := map[string]bool{}
	for _, worktree := range stateWorktrees(root) {
		branch := TrackedOf(worktree)
		held[branch] = true
		if running[branch] || running[filepath.Clean(worktree.Path)] {
			continue
		}
		if _, leased := lease.Held(root, branch, now()); leased {
			continue
		}
		if status, err := git.Run(worktree.Path, "status", "--porcelain"); err != nil || status != "" {
			continue
		}
		tip := worktree.Head
		if ref := git.Or(root, "rev-parse", "--verify", "--quiet", line.TipRef(branch)); ref != "" {
			tip = ref
		}
		pushed := git.Or(worktree.Path, "config", "--worktree", "--get", "komodo.pushed")
		// A commit neither on origin nor the line's own recorded push is unpushed work, whatever else says landed.
		if !safe(root, pushed, worktree.Head) || !safe(root, pushed, tip) {
			continue
		}
		if !landed(root, branch, tip, remote) && !squashLanded(root, branch, worktree.Head, pushed) &&
			!idlePushed(root, worktree.Path, worktree.Head, tip) {
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
		if !landed(root, branch, ref, remote) && !squashLanded(root, branch, ref, "") {
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

// idlePushed reports a worktree whose git state sat unchanged for idleFor and whose every commit is on origin.
func idlePushed(root, path string, commits ...string) bool {
	admin, err := git.Run(path, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return false
	}
	var touched time.Time
	for _, name := range []string{"HEAD", "index"} {
		if info, err := os.Stat(filepath.Join(admin, name)); err == nil && info.ModTime().After(touched) {
			touched = info.ModTime()
		}
	}
	if touched.IsZero() || now().Sub(touched) < idleFor {
		return false
	}
	for _, commit := range commits {
		if !onOrigin(root, commit) {
			return false
		}
	}
	return true
}

// safe reports whether commit is on origin or is the push the line recorded for this worktree.
func safe(root, pushed, commit string) bool {
	return commit == pushed || onOrigin(root, commit)
}

// onOrigin reports whether some remote-tracking branch of origin contains commit.
func onOrigin(root, commit string) bool {
	return git.Or(root, "for-each-ref", "--count=1", "--contains", commit, "refs/remotes/origin/") != ""
}

// mergedOnForge reports whether the forge holds a merged pull request headed by branch; no forge says no.
var mergedOnForge = func(root, branch string) bool {
	merged, err := pr.New(root).MergedHead(branch)
	return err == nil && merged
}

// squashLanded reports a squash-merged branch: the forge lists its merged pull request, or origin dropped it at the pushed tip.
func squashLanded(root, branch, tip, pushed string) bool {
	if pushed != "" && pushed == tip && originLacks(root, branch) {
		return true
	}
	return mergedOnForge(root, branch)
}

// originLacks reports whether origin answers and holds no branch of that name.
func originLacks(root, branch string) bool {
	cmd := exec.Command("git", "ls-remote", "--exit-code", "--heads", "origin", branch)
	cmd.Dir = root
	err := cmd.Run()
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 2
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
