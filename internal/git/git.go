// Package git runs git commands and parses their output for the rest of the toolkit.
package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"komodo/internal/proc"
)

// Timeout bounds how long one git command may run before its process group is killed; a test lowers it.
var Timeout = proc.DefaultTimeout

// waitDelay bounds how long a killed git command's output pipes may stay open after it exits.
const waitDelay = 5 * time.Second

// Worktree is one entry of worktree list --porcelain.
type Worktree struct {
	Path     string
	Branch   string
	Head     string
	Detached bool
	Tracked  string
}

// Run runs one git command in dir and returns its trimmed stdout, or an error naming the
// command and stderr; a hung git is killed, process group included, once Timeout passes.
func Run(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = WithoutRepoPointers(os.Environ())
	stdout, stderr := proc.NewBoundedWriter(proc.MaxOutput), proc.NewBoundedWriter(proc.MaxOutput)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	err := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("git %s: timed out after %s: %s", strings.Join(args, " "), Timeout, strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Or runs one git command in dir and returns its output, or nothing when it fails.
func Or(dir string, args ...string) string {
	out, err := Run(dir, args...)
	if err != nil {
		return ""
	}
	return out
}

// Worktrees lists the repo's worktrees, main first, from worktree list --porcelain, with each
// detached worktree's Tracked branch read from its own komodo.branch config.
func Worktrees(dir string) ([]Worktree, error) {
	out, err := Run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	worktrees := ParseWorktrees(out)
	for index := range worktrees {
		if worktrees[index].Detached {
			worktrees[index].Tracked = Or(worktrees[index].Path, "config", "--worktree", "--get", "komodo.branch")
		}
	}
	return worktrees, nil
}

// TrackedBranch is the branch dir's HEAD moves with, or, on a detached HEAD, the branch its
// worktree's komodo.branch config names.
func TrackedBranch(dir string) string {
	if branch := Or(dir, "symbolic-ref", "--short", "HEAD"); branch != "" {
		return branch
	}
	return Or(dir, "config", "--worktree", "--get", "komodo.branch")
}

// ParseWorktrees reads worktree list --porcelain output into its entries, in order.
func ParseWorktrees(out string) []Worktree {
	var worktrees []Worktree
	for _, block := range strings.Split(out, "\n\n") {
		var current Worktree
		for _, field := range strings.Split(block, "\n") {
			if path, ok := strings.CutPrefix(field, "worktree "); ok {
				current.Path = path
			}
			if ref, ok := strings.CutPrefix(field, "branch refs/heads/"); ok {
				current.Branch = ref
			}
			if head, ok := strings.CutPrefix(field, "HEAD "); ok {
				current.Head = head
			}
			if field == "detached" {
				current.Detached = true
			}
		}
		if current.Path != "" {
			worktrees = append(worktrees, current)
		}
	}
	return worktrees
}
