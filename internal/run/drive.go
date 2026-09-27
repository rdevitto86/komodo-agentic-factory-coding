package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/profile"
)

// Drive cuts one group's worktree when no run is open for it, then drives it through the
// conductor to Shipped, resuming a saved state.json instead of starting fresh; it exits non-zero unless it reached Shipped.
func Drive(options Options) (int, error) {
	root := options.Root
	// A group resumed past Prepare has its tasks DONE already, so its open run's plan keeps closed tasks.
	plan, err := line.PlanForGroup(root, options.Target)
	if err != nil {
		return 1, err
	}
	if plan == nil {
		return 1, errors.New("nothing is ready")
	}
	runState, err := cutIfNeeded(root, plan)
	if err != nil {
		return 1, err
	}
	worktree := line.WorktreePath(root, plan.Worktree)
	contract, err := driverContract(root, worktree)
	if err != nil {
		return 1, err
	}
	driver, err := newDriver(root, plan, runState.Run, contract, options.PR)
	if err != nil {
		return 1, err
	}
	budget := options.Budget
	if budget <= 0 {
		budget = GroupBudget
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	statePath := conductor.StatePath(root, plan.Group)
	final, err := driveState(ctx, driver, statePath, plan, worktree)
	if saveErr := conductor.SaveState(statePath, final); saveErr != nil && err == nil {
		err = saveErr
	}
	if err != nil {
		return 1, err
	}
	if final.Current != conductor.Shipped {
		return 1, fmt.Errorf("%s stopped at %s, not Shipped", plan.Group, final.Current)
	}
	return 0, nil
}

// driveState resumes a group from its saved state.json, or starts it fresh at Ready with its
// slot already taken, since a targeted run never waits on another group's slot.
func driveState(
	ctx context.Context, driver *conductor.Driver, statePath string, plan *line.Plan, worktree string,
) (conductor.State, error) {
	if saved, err := conductor.LoadState(statePath); err == nil {
		return driver.Resume(ctx, saved)
	}
	fresh := conductor.State{
		Group: plan.Group, Worktree: worktree, Branch: plan.Branch, Current: conductor.Ready, SlotFree: true,
	}
	return driver.Drive(ctx, fresh)
}

// cutIfNeeded returns the group's own run, cutting its branch and worktree when none is open
// yet, and fills the plan's base, branch and worktree from whichever run it finds.
func cutIfNeeded(root string, plan *line.Plan) (line.RunState, error) {
	if state, err := line.LoadRunFor(root, plan.Group); err == nil && state.Group == plan.Group {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
		return state, nil
	}
	return line.Start(root, plan, "", false)
}

// driverContract resolves the profile's host and builds its session driver over worktree.
func driverContract(root, worktree string) (mount.Contract, error) {
	selected := profile.Select(root)
	host, ok := mount.Get(selected.Host)
	if !ok || host.Contract == nil {
		return nil, errors.New("no mount is installed here; run komodo install")
	}
	return host.Contract(root, worktree), nil
}

// newDriver builds the conductor's driver for one group: its host, its requests, its wrapped
// stations, and the shared ledger, saving every move to the group's own state.json.
func newDriver(root string, plan *line.Plan, run string, contract mount.Contract, client *pr.Client) (*conductor.Driver, error) {
	builder, err := BuilderRequest(root, plan)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = pr.New(line.WorktreePath(root, plan.Worktree))
	}
	return &conductor.Driver{
		Host:     contract,
		Stations: &conductor.Line{Root: root, Plan: plan, Client: client},
		Ledger:   line.Book(root),
		Run:      run,
		Builder:  builder,
		// The reviewer's brief carries the diff, so it is built at review, after the build is committed.
		Review: func() (mount.StartRequest, error) {
			if err := line.CommitBuild(root, plan); err != nil {
				return mount.StartRequest{}, fmt.Errorf("committing the build for review: %w", err)
			}
			return ReviewerRequest(root, plan)
		},
		SeverityFloor: plan.Profile.SeverityFloor,
		Repairs:       plan.Profile.ReviewRepairs,
		Save: func(s conductor.State) error {
			return conductor.SaveState(conductor.StatePath(root, plan.Group), s)
		},
		WriteReview: func(group string, result mount.Result) error {
			return writeReview(root, group, result)
		},
	}, nil
}

// writeReview saves the reviewer's whole result, findings with their files and titles, where
// ShipGroup reads it, so its findings gate and the findings it files match what the reviewer said.
func writeReview(root, group string, result mount.Result) error {
	data, err := json.Marshal(result.Value)
	if err != nil {
		return err
	}
	path := line.ResultPath(root, group+"-review")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
