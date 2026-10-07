package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/gate"
	"komodo/internal/git"
	"komodo/internal/mount"
	"komodo/internal/proc"
	profilepkg "komodo/internal/profile"
)

// Attempt records how often a task has failed and what it failed with.
type Attempt struct {
	Count   int    `json:"count"`
	Failure string `json:"failure"`
	Diff    string `json:"diff,omitempty"`
}

// TaskTimeout is the wall clock a task's commands get: its own timeout key, else the station default.
func TaskTimeout(task backlog.Task) time.Duration {
	if parsed, err := time.ParseDuration(task.Timeout()); err == nil && parsed > 0 {
		return parsed
	}
	return CommandTimeout
}

// CommitBuild commits a group builder's uncommitted work onto the group branch, so review reads it in the branch's diff.
func CommitBuild(root string, plan *Plan) error {
	return CommitBuildContext(context.Background(), root, plan)
}

// CommitBuildContext is CommitBuild under ctx: its commit, and the pre-commit hook it runs, die once
// ctx is done, so a stop mid-hook never lands the commit.
func CommitBuildContext(ctx context.Context, root string, plan *Plan) error {
	worktree := WorktreePath(root, plan.Worktree)
	if _, err := git.Run(worktree, "rev-parse", "--git-dir"); err != nil {
		return nil
	}
	var declared []string
	for _, task := range plan.Tasks {
		declared = append(declared, task.Files...)
	}
	if err := stageWork(worktree, declared); err != nil {
		return err
	}
	if staged, err := git.Run(worktree, "diff", "--cached", "--name-only"); err != nil || strings.TrimSpace(staged) == "" {
		return err
	}
	old, tipErr := git.Run(worktree, "rev-parse", "--verify", TipRef(plan.Branch))
	message := fmt.Sprintf("%s: %s, as built (%s)", plan.Type, plan.Title, plan.Group)
	if err := commitContext(ctx, worktree, message); err != nil {
		return err
	}
	if tipErr != nil {
		return nil
	}
	return Advance(worktree, plan.Branch, worktree, old)
}

// commitContext runs git commit -m message in worktree under ctx, so a stop while its pre-commit hook
// runs kills the hook before it can land the commit.
func commitContext(ctx context.Context, worktree, message string) error {
	clock, cancel := context.WithTimeout(ctx, git.Timeout)
	defer cancel()
	cmd := exec.CommandContext(clock, "git", "commit", "-m", message)
	cmd.Dir = worktree
	cmd.Env = git.WithoutRepoPointers(os.Environ())
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	output := proc.NewBoundedWriter(proc.MaxOutput)
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if errors.Is(clock.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("git commit: timed out after %s: %s", git.Timeout, strings.TrimSpace(output.String()))
	}
	if err != nil {
		return fmt.Errorf("git commit: %v: %s", err, strings.TrimSpace(output.String()))
	}
	return nil
}

// stageWork stages every change in cwd except the state dir and each mount's rendered project copies, unless declared.
func stageWork(cwd string, declared []string) error {
	args := []string{"add", "-A", "--", "."}
	var rescued []string
	for _, excluded := range append([]string{StateDir}, mount.ProjectPaths(cwd)...) {
		covered := false
		var inside []string
		for _, file := range declared {
			file = strings.TrimSuffix(strings.TrimPrefix(strings.ReplaceAll(file, "\\", "/"), "./"), "/")
			switch {
			case file == excluded || strings.HasPrefix(excluded, file+"/"):
				covered = true
			case strings.HasPrefix(file, excluded+"/"):
				inside = append(inside, file)
			}
		}
		if covered || ignoredAndUntracked(cwd, excluded) {
			continue
		}
		args = append(args, ":(exclude,literal)"+excluded)
		rescued = append(rescued, inside...)
	}
	if _, err := git.Run(cwd, args...); err != nil {
		return err
	}
	for _, file := range rescued {
		// A declared file under an excluded path is staged on its own, when git sees a change there.
		if changed, err := git.Run(cwd, "status", "--porcelain", "--", file); err != nil || changed == "" {
			continue
		}
		if _, err := git.Run(cwd, "add", "-A", "--", file); err != nil {
			return err
		}
	}
	return nil
}

// ignoredAndUntracked reports a path git ignores and tracks nothing under, so add -A skips it unexcluded.
// Excluding such a path makes git add exit 1 once it exists on disk.
func ignoredAndUntracked(cwd, path string) bool {
	if _, err := git.Run(cwd, "check-ignore", "-q", "--", path); err != nil {
		return false
	}
	tracked, err := git.Run(cwd, "ls-files", "--", path)
	return err == nil && tracked == ""
}

// isToolkit reports whether this repo is the toolkit, which gates its own commits.
func isToolkit(root string) bool {
	return gate.IsToolkit(root)
}

// gateCommand runs the local gate in the worktree under its own wall clock.
func gateCommand(cwd string) error {
	ran := proc.Exec(cwd, GateTimeout, "go", "run", "./cmd/komodo", "gate")
	if !ran.OK() {
		return fmt.Errorf("%v\n%s", ran.Err(), Clip(ran.Output, 4000, "gate"))
	}
	return nil
}

// attemptPath is where a task's failure record lives.
func attemptPath(root, taskID string) string {
	return filepath.Join(root, StateDir, "attempts", taskID+".json")
}

// LoadAttempt reads what a task failed with last time, if anything.
func LoadAttempt(root, taskID string) Attempt {
	var attempt Attempt
	data, err := os.ReadFile(attemptPath(root, taskID))
	if err != nil {
		return attempt
	}
	_ = json.Unmarshal(data, &attempt)
	return attempt
}

// RepairText is what the failure slot carries into the next brief, the output and the diff.
func RepairText(root, taskID string) string {
	attempt := LoadAttempt(root, taskID)
	if attempt.Count == 0 {
		return ""
	}
	text := attempt.Failure
	if attempt.Diff != "" {
		text += "\n\n# The diff your last attempt left\n" + attempt.Diff
	}
	return text
}

// resolveProfile is the selected profile narrowed by the developer's overlay, which every station shares.
func resolveProfile(root string) profilepkg.Profile {
	chosen := profilepkg.Select(root)
	if path := mount.OverlayPath(); path != "" {
		chosen = profilepkg.Overlay(chosen, path)
	}
	return chosen
}
