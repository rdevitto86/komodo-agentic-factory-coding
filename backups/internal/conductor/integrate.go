package conductor

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/check"
	"komodo/internal/git"
	"komodo/internal/harness"
	"komodo/internal/pr"
	"komodo/internal/proc"
)

// integrating are the states of a group ready in a run: past its own checks and review, not yet merged.
var integrating = []GroupState{Preparing, Shipping, Shipped}

// integrate test-merges every other group ready in the same run into this group's HEAD and runs the
// integration build there, returning a fix for this group per breakage.
func (l *Harness) integrate(ctx context.Context) ([]string, error) {
	own, err := harness.RunFor(l.Root, l.Plan.Group)
	if err != nil || own.Run == "" {
		return nil, nil
	}
	var others []harness.RunState
	for _, state := range harness.LoadRuns(l.Root) {
		if state.Group == l.Plan.Group || state.Run != own.Run || state.Branch == "" {
			continue
		}
		saved, err := LoadState(StatePath(l.Root, state.Group))
		if err != nil || !slices.Contains(integrating, saved.Current) {
			continue
		}
		others = append(others, state)
	}
	worktree := harness.WorktreePath(l.Root, l.Plan.Worktree)
	commands := append(harness.CompileCommands(l.Root, worktree), harness.VerifyCommand(l.Root, worktree))
	return TestMerge(ctx, worktree, others, commands)
}

// TestMerge merges each ready group's branch into a scratch worktree at worktree's HEAD, then runs commands
// there; each branch that conflicts and each command that fails is a fix for the group at worktree.
func TestMerge(ctx context.Context, worktree string, ready []harness.RunState, commands []string) ([]string, error) {
	if len(ready) == 0 {
		return nil, nil
	}
	scratch, err := os.MkdirTemp("", "komodo-integrate-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	if _, err := git.Run(worktree, "worktree", "add", "--detach", scratch, "HEAD"); err != nil {
		return nil, err
	}
	defer func() { _, _ = git.Run(worktree, "worktree", "remove", "--force", scratch) }()
	var fixes []string
	for _, other := range ready {
		if _, err := git.Run(scratch, "merge", "--no-edit", "--", other.Branch); err != nil {
			conflicts, _ := git.Run(scratch, "diff", "--name-only", "--diff-filter=U")
			_, _ = git.Run(scratch, "merge", "--abort")
			fixes = append(fixes, fmt.Sprintf("integration: the group no longer merges with %s's branch %s; it conflicts in %s",
				other.Group, other.Branch, strings.Join(strings.Fields(conflicts), ", ")))
		}
	}
	if len(fixes) > 0 {
		return fixes, nil
	}
	for _, command := range commands {
		if command == "" {
			continue
		}
		argv := proc.ShellArgv(command)
		if ran := proc.ExecContext(ctx, scratch, check.CommandTimeout, argv[0], argv[1:]...); !ran.OK() {
			fixes = append(fixes, fmt.Sprintf("integration: `%s` %v with every ready group merged in\n%s",
				command, ran.Err(), ran.Output))
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return fixes, nil
}

// StackBase is the branch a group's PR targets: the branch of the first group of its epic it depends on
// that has not merged yet, else its epic's branch.
func StackBase(parsed backlog.Backlog, group backlog.Group, merged func(branch string) bool) string {
	epic := group.EpicBranch()
	for _, id := range group.DependsOn() {
		parent, ok := parsed.Group(id)
		if !ok || parent.EpicBranch() != epic {
			continue
		}
		if branch := parent.Branch(); !merged(branch) {
			return branch
		}
	}
	return epic
}

// mergedInto reports whether branch has merged into epic, or has no tip or pushed branch left to stack on.
func mergedInto(root, branch, epic string) bool {
	tip := harness.TipRef(branch)
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", tip); err != nil {
		tip = "refs/remotes/origin/" + branch
		if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", tip); err != nil {
			return true
		}
	}
	_, err := git.Run(root, "merge-base", "--is-ancestor", tip, harness.StartRef(root, epic))
	return err == nil
}

// Restack takes a merged parent's base into each group stacked on it, pushing and retargeting a pushed
// branch's PR, skipping any group running names, since that lane's worktree is not its own to rebase.
func Restack(root string, client *pr.Client, running []string) ([]string, error) {
	if !backlog.Exists(root) {
		return nil, nil
	}
	parsed, _, err := harness.LoadBacklog(root)
	if err != nil {
		return nil, err
	}
	var moved []string
	for _, state := range harness.LoadRuns(root) {
		if slices.Contains(running, state.Group) {
			continue
		}
		group, ok := parsed.Group(state.Group)
		epic := group.EpicBranch()
		stacked := slices.ContainsFunc(group.DependsOn(), func(id string) bool {
			parent, ok := parsed.Group(id)
			return ok && parent.Branch() == state.Base
		})
		if !ok || epic == "" || !stacked {
			continue
		}
		_ = harness.Fetch(root, epic)
		base := StackBase(parsed, group, func(branch string) bool { return mergedInto(root, branch, epic) })
		if base == state.Base {
			continue
		}
		worktree := harness.WorktreePath(root, state.Worktree)
		pushed, err := restackOnto(root, worktree, state.Branch, base)
		if err != nil {
			return moved, fmt.Errorf("restacking %s onto %s: %w", state.Group, base, err)
		}
		if pushed && client != nil {
			if err := client.Edit(state.Branch, "--base", base); err != nil {
				return moved, fmt.Errorf("retargeting %s's PR to %s: %w", state.Group, base, err)
			}
		}
		state.Base = base
		if err := harness.SaveRun(root, state); err != nil {
			return moved, err
		}
		moved = append(moved, fmt.Sprintf("%s onto %s", state.Group, base))
	}
	return moved, nil
}

// restackOnto takes base into branch's worktree, rebasing a local branch and merging into a pushed one,
// which it then pushes; it reports whether the branch was pushed.
func restackOnto(root, worktree, branch, base string) (bool, error) {
	_ = harness.Fetch(worktree, base)
	target := harness.StartRef(worktree, base)
	pushed := harness.OnOrigin(worktree, branch)
	args, abort := harness.RebaseOrMergeArgs(target, pushed)
	old, tipErr := git.Run(root, "rev-parse", "--verify", "--quiet", harness.TipRef(branch))
	if _, err := git.Run(worktree, args...); err != nil {
		_, _ = git.Run(worktree, abort...)
		return pushed, err
	}
	if !pushed {
		if tipErr == nil {
			return false, harness.Advance(root, branch, worktree, old)
		}
		return false, nil
	}
	if tipErr == nil {
		if err := harness.Advance(root, branch, worktree, old); err != nil {
			return true, err
		}
	}
	return true, harness.PushFromWorktree(root, worktree, branch)
}
