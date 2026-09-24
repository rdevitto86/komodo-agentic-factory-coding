package main

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"

	"komodo/internal/comments"
)

// commentsArgs strips the check subcommand, if present, then splits what remains into paths and flags.
func commentsArgs(args []string, valueFlags ...string) (paths, rest []string) {
	if len(args) > 0 && args[0] == "check" {
		args = args[1:]
	}
	return splitFlags(args, valueFlags...)
}

// runComments lints the comments in the named files, or in every tracked source file.
func runComments(root string, args []string) {
	set := flag.NewFlagSet("comments", flag.ExitOnError)
	require := set.String("require", "nonobvious", "none, nonobvious, or exported")
	paths, rest := commentsArgs(args, "require")
	_ = set.Parse(rest)
	if len(paths) == 0 {
		paths = trackedFiles(root)
	} else if err := verifyPaths(root, paths); err != nil {
		fail(err)
	}
	problems, err := comments.Check(root, paths, *require)
	if err != nil {
		fail(err)
	}
	for _, problem := range problems {
		fmt.Println(problem)
	}
	fmt.Printf("%d file(s), %d problem(s)\n", len(paths), len(problems))
	if len(problems) > 0 {
		exit(1)
	}
}

// trackedFiles lists what git tracks, which is what the lint walks by default.
func trackedFiles(root string) []string {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}
