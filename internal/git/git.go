// Package git runs git commands and parses their output for the rest of the toolkit.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Worktree is one entry of worktree list --porcelain.
type Worktree struct {
	Path   string
	Branch string
	Head   string
}

// Run runs one git command in dir and returns its trimmed stdout, or an error naming the command and stderr.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
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

// Worktrees lists the repo's worktrees, main first, from worktree list --porcelain.
func Worktrees(dir string) ([]Worktree, error) {
	out, err := Run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParseWorktrees(out), nil
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
		}
		if current.Path != "" {
			worktrees = append(worktrees, current)
		}
	}
	return worktrees
}
