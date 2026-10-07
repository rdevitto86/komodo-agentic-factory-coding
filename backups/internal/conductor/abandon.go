package conductor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/harness"
)

// abandonedState is the state an abandoned group's note names when its run saved no state.json.
const abandonedState = "Abandoned"

// Abandon removes a group's worktree, tip ref and run record on purpose, and writes a blocker note saying so
// into the root's backlog, with every open task BLOCKED; it refuses while a live run holds the group.
func Abandon(root, group string, at time.Time) error {
	if err := harness.CheckLock(root, group); err != nil {
		return err
	}
	run, err := harness.LoadRunFor(root, group)
	if err != nil {
		return fmt.Errorf("%s has no run to abandon: %w", group, err)
	}
	if guard.Load(root, root).IsCritical(run.Branch) {
		return fmt.Errorf("%s runs on %s, a critical ref; nothing was removed", group, run.Branch)
	}
	state := abandonedState
	if saved, err := LoadState(StatePath(root, group)); err == nil && saved.Current != "" {
		state = string(saved.Current)
	}
	note := backlog.BlockerNote{
		At: at, Run: run.Run, State: state,
		Items: []string{"abandoned on purpose with `komodo abandon`; its worktree and tip are removed"},
		Needs: "a person to rework the group and set it READY",
	}
	dir, before, err := abandonNote(root, group, note)
	if err != nil {
		return err
	}
	// The note is written before the worktree and tip are gone, and restored if removing either fails.
	if err := dir.WriteNote(note); err != nil {
		return err
	}
	if err := removeWorktreeAndTip(root, group, run); err != nil {
		if restoreErr := os.WriteFile(dir.Path, before, 0o644); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}
	return os.RemoveAll(harness.RunDir(root, group))
}

// removeWorktreeAndTip force-removes the group's worktree, prunes it, and drops its tip ref.
func removeWorktreeAndTip(root, group string, run harness.RunState) error {
	worktree := run.Worktree
	if worktree == "" {
		worktree = filepath.Join(harness.StateDir, "wt", group)
	}
	if worktree = harness.WorktreePath(root, worktree); worktree != root {
		if _, err := os.Stat(worktree); err == nil {
			if _, err := git.Run(root, "worktree", "remove", "--force", worktree); err != nil {
				return err
			}
		}
	}
	if _, err := git.Run(root, "worktree", "prune"); err != nil {
		return err
	}
	if run.Branch == "" {
		return nil
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", harness.TipRef(run.Branch)); err != nil {
		return nil
	}
	_, err := git.Run(root, "update-ref", "-d", harness.TipRef(run.Branch))
	return err
}

// abandonNote locates the group folder and proves note renders into its the group index file, returning the prior text.
func abandonNote(root, group string, note backlog.BlockerNote) (backlog.GroupDir, []byte, error) {
	dir, found, err := backlog.Locate(root, group)
	if err != nil {
		return dir, nil, err
	}
	if !found {
		return dir, nil, fmt.Errorf("%s is not in %s", group, backlog.GroupFilesDir)
	}
	before, err := os.ReadFile(dir.Path)
	if err != nil {
		return dir, nil, err
	}
	_, err = backlog.AddGroupFileNote(string(before), note)
	return dir, before, err
}
