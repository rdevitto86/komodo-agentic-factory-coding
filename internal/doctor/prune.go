package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
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

// settleShippedRun sweeps merged worktrees and restores flips for the current branch, skipping while a run is open.
func settleShippedRun(root, base string) []string {
	if _, err := git.Run(root, "fetch", "--quiet", "origin", base); err != nil {
		return nil
	}
	remote := "origin/" + base
	open := line.RunIsOpen(root)
	var done []string
	if state, err := line.LoadRun(root); err == nil && state.Branch != "" && !open {
		if _, err := git.Run(root, "merge-base", "--is-ancestor", state.Branch, remote); err == nil {
			if restoreFlips(root, remote) {
				done = append(done, "restored BACKLOG.md; "+remote+" holds its status flips")
			}
		}
	}
	if open {
		return done
	}
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

// restoreFlips puts BACKLOG.md back to HEAD when every change in it is a status token remote already holds.
func restoreFlips(root, remote string) bool {
	path, err := backlog.Find(root)
	if err != nil {
		return false
	}
	name := filepath.ToSlash(rel(root, path))
	head, err := gitRaw(root, "show", "HEAD:"+name)
	if err != nil {
		return false
	}
	merged, err := gitRaw(root, "show", remote+":"+name)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) == head {
		return false
	}
	current, before, after := backlog.Parse(string(data)), backlog.Parse(head), backlog.Parse(merged)
	reverted := string(data)
	for _, group := range current.Groups {
		for _, task := range group.Tasks {
			old, ok := before.Task(task.ID)
			if !ok {
				return false
			}
			if old.Status == task.Status {
				continue
			}
			if landed, ok := after.Task(task.ID); !ok || landed.Status != task.Status {
				return false
			}
			if reverted, err = backlog.SetStatus(reverted, task.ID, old.Status); err != nil {
				return false
			}
		}
	}
	if reverted != head {
		return false
	}
	return os.WriteFile(path, []byte(head), 0o644) == nil
}

// gitRaw runs git in root and returns its output untrimmed, for a file's exact bytes.
func gitRaw(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return string(out), err
}
