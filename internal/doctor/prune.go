package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
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

// settleShippedRun sweeps merged worktrees and restores flips for the current branch, skipping while a run is open.
func settleShippedRun(root, base string) []string {
	if _, err := git(root, "fetch", "--quiet", "origin", base); err != nil {
		return nil
	}
	remote := "origin/" + base
	open := line.RunIsOpen(root)
	var done []string
	if state, err := line.LoadRun(root); err == nil && state.Branch != "" && !open {
		if _, err := git(root, "merge-base", "--is-ancestor", state.Branch, remote); err == nil {
			if restoreFlips(root, remote) {
				done = append(done, "restored BACKLOG.md; "+remote+" holds its status flips")
			}
		}
	}
	if open {
		return done
	}
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
