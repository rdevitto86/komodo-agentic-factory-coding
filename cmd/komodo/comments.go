package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
	apply := set.Bool("fix", false, "rewrite every finding with one mechanical rule")
	paths, rest := commentsArgs(args, "require")
	_ = set.Parse(rest)
	if len(paths) == 0 {
		paths = comments.TrackedFiles(root)
	} else if err := verifyPaths(root, paths); err != nil {
		fail(err)
	}
	if *apply {
		runCommentsFix(root, paths)
		return
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

// runCommentsFix rewrites every mechanical finding in the named files and reports how many it applied.
func runCommentsFix(root string, paths []string) {
	fixed := 0
	for _, path := range paths {
		full := filepath.Join(root, path)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			fail(err)
		}
		rewritten, count := comments.Fix(string(data), path)
		if count == 0 {
			continue
		}
		if err := os.WriteFile(full, []byte(rewritten), info.Mode()); err != nil {
			fail(err)
		}
		fixed += count
	}
	fmt.Printf("%d fix(es) applied\n", fixed)
}

