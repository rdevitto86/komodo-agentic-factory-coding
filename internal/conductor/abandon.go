package conductor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/line"
)

// abandonedState is the state an abandoned group's note names when its run saved no state.json.
const abandonedState = "Abandoned"

// Abandon removes a group's worktree, branch and run record on purpose, and writes a blocker note saying so
// into the root's backlog, with every open task BLOCKED; it refuses while a live run holds the group.
func Abandon(root, group string, at time.Time) error {
	if err := line.CheckLock(root, group); err != nil {
		return err
	}
	run, err := line.LoadRunFor(root, group)
	if err != nil {
		return fmt.Errorf("%s has no run to abandon: %w", group, err)
	}
	if guard.Load(root, root).IsCritical(run.Branch) {
		return fmt.Errorf("%s runs on %s, a critical ref; nothing was removed", group, run.Branch)
	}
	path, err := backlog.Find(root)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	state := abandonedState
	if saved, err := LoadState(StatePath(root, group)); err == nil && saved.Current != "" {
		state = string(saved.Current)
	}
	noted, err := backlog.AddNote(string(data), group, backlog.BlockerNote{
		At: at, Run: run.Run, State: state,
		Items: []string{"abandoned on purpose with `komodo abandon`; its worktree and branch are removed"},
		Needs: "a person to rework the group and set it READY",
	})
	if err != nil {
		return err
	}
	worktree := run.Worktree
	if worktree == "" {
		worktree = filepath.Join(line.StateDir, "wt", group)
	}
	if worktree = line.WorktreePath(root, worktree); worktree != root {
		if _, err := os.Stat(worktree); err == nil {
			if _, err := git.Run(root, "worktree", "remove", "--force", worktree); err != nil {
				return err
			}
		}
	}
	if _, err := git.Run(root, "worktree", "prune"); err != nil {
		return err
	}
	if run.Branch != "" {
		if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+run.Branch); err == nil {
			if _, err := git.Run(root, "branch", "-D", run.Branch); err != nil {
				return err
			}
		}
	}
	if err := os.WriteFile(path, []byte(noted), 0o644); err != nil {
		return err
	}
	return os.RemoveAll(line.RunDir(root, group))
}
