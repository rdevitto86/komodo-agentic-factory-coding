package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"komodo/internal/eval"
	"komodo/internal/mount"
	"komodo/internal/run"
)

// runEval reads the golden suite and lists it with --list, runs the eval cases with --cases, or with --runs N
// drives each group N times in fresh clones and prints the report for this platform.
func runEval(root string, args []string) {
	set := flag.NewFlagSet("eval", flag.ExitOnError)
	list := set.Bool("list", false, "print the golden groups and their pinned commits")
	runs := set.Int("runs", 0, "drive each golden group this many times, each in a fresh clone")
	dir := set.String("suite", filepath.Join(root, eval.SuiteDir), "the directory holding the suite")
	host := set.String("host", "", "the mount each clone installs, the default mount when empty")
	work := set.String("work", "", "where the fresh clones go, a new temp directory when empty")
	budget := set.Duration("budget", run.GroupBudget, "how long one run of one group, or one case, may take")
	cases := set.Bool("cases", false, "run every eval case, each in a fresh clone of the suite's first group")
	instructions := set.String("instructions", "",
		"this machine's personal host instructions file, for the canary case; the canary is skipped when empty")
	_ = set.Parse(args)
	suite, err := eval.Load(*dir)
	if err != nil {
		fail(err)
	}
	if *list {
		if err := suite.List(os.Stdout); err != nil {
			fail(err)
		}
		return
	}
	if !*cases && *runs < 1 {
		set.Usage()
		exit(2)
		return
	}
	executable, err := mount.Executable()
	if err != nil {
		fail(err)
	}
	if *work == "" {
		if *work, err = os.MkdirTemp("", "komodo-eval-"); err != nil {
			fail(err)
		}
	}
	if *cases {
		options := eval.CaseOptions{Cases: eval.Cases(), Budget: *budget, Stdout: os.Stdout}
		live := eval.LiveOptions{Suite: suite, Executable: executable, Host: *host, Instructions: *instructions}
		// An interrupt cancels the cases, which unwinds the canary's restore instead of killing the process first.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := eval.LiveCases(ctx, options, live, *work); err != nil {
			fail(err)
		}
		return
	}
	outcomes, err := eval.Run(context.Background(), eval.Options{
		Suite: suite, Runs: *runs, Work: *work, Budget: *budget,
		Line: eval.Launch(executable, *host, *budget), Stdout: os.Stdout,
	})
	eval.Summarize(outcomes).Print(os.Stdout)
	if err != nil {
		fail(err)
	}
}
