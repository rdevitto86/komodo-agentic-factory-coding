package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"komodo/internal/harness"
)

// runWorktree is komodo's one exposed git-worktree action: cutting one for ad hoc work.
func runWorktree(root string, args []string) {
	if len(args) == 0 {
		fail(fmt.Errorf("usage: komodo worktree add <branch> [--from <ref>]"))
	}
	switch args[0] {
	case "add":
		runWorktreeAdd(root, args[1:])
	default:
		fail(fmt.Errorf("unknown worktree command %q: add", args[0]))
	}
}

// runWorktreeAdd cuts a detached worktree at .komodo/wt/<slug> tracking branch, printing its path
// and the push that lands branch's tip on origin.
func runWorktreeAdd(root string, args []string) {
	set := flag.NewFlagSet("worktree add", flag.ExitOnError)
	from := set.String("from", "", "the ref to start the branch from, the remote-tracked base when empty")
	_ = set.Parse(args)
	if set.NArg() == 0 {
		fail(fmt.Errorf("usage: komodo worktree add <branch> [--from <ref>]"))
	}
	branch := set.Arg(0)
	// Flags may follow the branch too; the flag package stops at the first positional argument.
	_ = set.Parse(set.Args()[1:])
	if set.NArg() > 0 {
		fail(fmt.Errorf("usage: komodo worktree add <branch> [--from <ref>]; unexpected %q", set.Arg(0)))
	}
	start := *from
	switch {
	case start != "":
	case harness.OnOrigin(root, branch):
		start = "origin/" + branch
	default:
		start = harness.StartRef(root, harness.DefaultBase(root))
	}
	path := filepath.Join(root, harness.StateDir, "wt", worktreeSlug(branch))
	if err := harness.AddDetached(root, branch, start, path); err != nil {
		fail(err)
	}
	// A builder's worktree never pushes; a person's does, by the command printed below.
	if err := harness.AllowWorktreePush(path); err != nil {
		fail(err)
	}
	fmt.Println(path)
	fmt.Printf("git -C %s push origin HEAD:refs/heads/%s\n", path, branch)
}

// worktreeSlug is the directory name a branch earns: everything after its first slash, or the
// whole branch when it holds none.
func worktreeSlug(branch string) string {
	if _, slug, ok := strings.Cut(branch, "/"); ok {
		return slug
	}
	return branch
}
