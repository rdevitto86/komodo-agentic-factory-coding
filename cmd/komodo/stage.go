package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/profile"
	"komodo/internal/run"
)

// stageUsage is the one line a malformed stage command prints.
const stageUsage = "usage: komodo stage <build|review|ship> [group] [--budget 90m]"

// runStage runs one stage ad hoc on a group, in its open worktree or else on the current branch;
// it exits non-zero when the stage fails, its builder blocks, or its review finds a fix.
func runStage(root string, args []string) {
	set := flag.NewFlagSet("stage", flag.ExitOnError)
	budget := set.Duration("budget", run.GroupBudget, "wall-clock budget for the stage")
	positional, rest := splitFlags(args, "budget")
	_ = set.Parse(rest)
	if len(positional) == 0 || len(positional) > 2 {
		fail(errors.New(stageUsage))
	}
	stage, err := conductor.ParseStage(positional[0])
	if err != nil {
		fail(err)
	}
	target := ""
	if len(positional) == 2 {
		target = positional[1]
	}
	plan, err := line.PlanForGroup(root, target)
	if err != nil {
		fail(err)
	}
	if plan == nil {
		fail(fmt.Errorf("nothing is ready for %q", target))
	}
	// A group with no open worktree runs its stage on the branch checked out at the root.
	worktree := line.WorktreePath(root, plan.Worktree)
	if _, err := os.Stat(worktree); err != nil {
		plan.Worktree, worktree = "", root
	}
	driver, err := stageDriver(root, worktree, plan, stage)
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *budget)
	defer cancel()
	final, err := driver.RunStage(ctx, stage, conductor.State{Group: plan.Group, Worktree: worktree, Branch: plan.Branch})
	if err != nil {
		fail(err)
	}
	switch {
	case final.Escalate:
		fmt.Printf("%s %s: blocked\n", plan.Group, stage)
		exit(1)
	case len(final.Fixes) > 0:
		fmt.Printf("%s %s: %d finding(s) to fix\n", plan.Group, stage, len(final.Fixes))
		for _, fix := range final.Fixes {
			fmt.Printf("  %s\n", fix)
		}
		exit(1)
	default:
		fmt.Printf("%s %s: done\n", plan.Group, stage)
	}
}

// stageHost resolves the profile's host over worktree; tests swap it for a fake.
var stageHost = func(root, worktree string) (mount.Contract, error) {
	host, ok := mount.Get(profile.Select(root).Host)
	if !ok || host.Contract == nil {
		return nil, errors.New("no mount is installed here; run komodo install")
	}
	return host.Contract(root, worktree), nil
}

// stageStations builds the stations a stage runs with no model; tests swap them for fakes.
var stageStations = func(root, worktree string, plan *line.Plan) conductor.Stations {
	return &conductor.Line{Root: root, Plan: plan, Client: pr.New(worktree)}
}

// stageReviewer builds the reviewer's request from the group's git diff; tests swap it for a fixed one.
var stageReviewer = run.ReviewerRequest

// stageDriver wires the conductor for one ad hoc stage: the profile's host over worktree, the request the stage
// starts, and a Save that leaves the group's state.json to its run.
func stageDriver(root, worktree string, plan *line.Plan, stage conductor.Stage) (*conductor.Driver, error) {
	host, err := stageHost(root, worktree)
	if err != nil {
		return nil, err
	}
	driver := &conductor.Driver{
		Host:          host,
		Stations:      stageStations(root, worktree, plan),
		Ledger:        line.Book(root),
		SeverityFloor: plan.Profile.SeverityFloor,
		Save:          func(conductor.State) error { return nil },
	}
	switch stage {
	case conductor.StageBuild:
		driver.Builder, err = run.BuilderRequest(root, plan)
	case conductor.StageReview:
		driver.Reviewer, err = stageReviewer(root, plan)
	}
	return driver, err
}
