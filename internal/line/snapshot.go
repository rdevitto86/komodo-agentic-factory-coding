package line

import (
	"fmt"
	"os"
	"path/filepath"

	"komodo/internal/backlog"
)

// TaskState is what the disk held for one task when the snapshot was read.
type TaskState struct {
	HasResult         bool
	Attempts          int
	RepairResultReady bool
	StaleBrief        bool
	Open              bool
	Tier              string
}

// Closeable reports whether the task's result is ready to close rather than a failure awaiting repair.
func (t TaskState) Closeable() bool {
	return t.HasResult && (t.Attempts == 0 || t.RepairResultReady)
}

// WaveState is what the ledger held for one wave when the snapshot was read.
type WaveState struct {
	Merged bool
}

// Snapshot is the run's state read once from disk, which Next decides from without reading again.
type Snapshot struct {
	Plan            *Plan
	Paused          *Action
	Cut             bool
	Group           backlog.Group
	Tasks           map[string]TaskState
	Waves           []WaveState
	Reviewed        bool
	ReviewSkippable bool
	Blocking        []Finding
	Handoff         bool
	Shipped         bool
}

// LoadSnapshot reads the plan, the run record, and every task, wave, review, and ship file once.
func LoadSnapshot(root, needle string) (Snapshot, error) {
	plan, err := PlanForStation(root, needle)
	if err != nil || plan == nil {
		return Snapshot{}, err
	}
	snap := Snapshot{Plan: plan, Paused: pausedAction(plan)}
	if snap.Paused != nil {
		return snap, nil
	}
	state, runErr := LoadRun(root)
	if runErr != nil || state.Group != plan.Group {
		return snap, nil
	}
	snap.Cut = true
	full, err := planForGroup(root, state.Group)
	if err != nil {
		return Snapshot{}, err
	}
	if full != nil {
		plan = full
		snap.Plan = full
	}
	if snap.Paused = pausedAction(plan); snap.Paused != nil {
		return snap, nil
	}
	path, err := backlog.Find(root)
	if err != nil {
		return Snapshot{}, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Group, _ = parsed.Group(plan.Group)
	snap.Tasks = map[string]TaskState{}
	for _, wave := range plan.Waves {
		for _, taskID := range wave {
			snap.Tasks[taskID] = loadTaskState(root, parsed, taskID)
		}
	}
	merged := true
	for index := range plan.Waves {
		wave := WaveState{Merged: waveMerged(root, plan, index)}
		snap.Waves = append(snap.Waves, wave)
		merged = merged && wave.Merged
	}
	if !merged {
		// Next never reaches the review until every wave merged, so the diff is not read before then.
		return snap, nil
	}
	snap.Reviewed = reviewed(root, plan)
	if !snap.Reviewed {
		snap.ReviewSkippable = reviewSkippable(root, plan)
	}
	snap.Blocking, _ = SplitFindings(ReviewFindings(root, plan.Group), plan.Profile.SeverityFloor)
	_, err = os.Stat(filepath.Join(root, StateDir, "ship.json"))
	snap.Handoff = err == nil
	snap.Shipped = shipped(root, plan, parsed)
	return snap, nil
}

// loadTaskState reads one task's result, attempts, brief age, and backlog status.
func loadTaskState(root string, parsed backlog.Backlog, taskID string) TaskState {
	state := TaskState{
		HasResult:         HasResult(root, taskID),
		Attempts:          LoadAttempt(root, taskID).Count,
		RepairResultReady: repairResultReady(root, taskID),
		StaleBrief:        staleBrief(root, taskID),
	}
	if task, ok := parsed.Task(taskID); ok {
		state.Open = task.Open()
		state.Tier = task.Tier()
	}
	return state
}

// pausedAction is the bare wait action once the profile closed its window, or nil while work remains.
func pausedAction(plan *Plan) *Action {
	if plan.WaitUntil == "" {
		return nil
	}
	return &Action{
		Action: "done", Until: plan.WaitUntil,
		Why: fmt.Sprintf("%s is paused until the window resets at %s", plan.Group, plan.WaitUntil),
	}
}

// Next decides the one next action from a snapshot alone: no disk, no git, no ledger.
func Next(snap Snapshot) Action {
	next, _ := decide(snap)
	return next
}

// decide is Next, also reporting whether the walk passed the review, which is when Step stamps it.
func decide(snap Snapshot) (Action, bool) {
	plan := snap.Plan
	if plan == nil {
		return Action{Action: "done", Why: "nothing is ready", Skills: []string{}, Facets: []string{}, Commands: []string{}}, false
	}
	if snap.Paused != nil {
		return *snap.Paused, false
	}
	if !snap.Cut {
		return Action{
			Action:  "run",
			Command: "komodo next --start " + plan.Group + " --base " + plan.Base,
			Why:     fmt.Sprintf("%s has not been cut yet, from %s", plan.Group, plan.Base),
		}, false
	}
	single := snap.Group.Mode() == "single"
	blocked := map[string]bool{}
	for index, wave := range plan.Waves {
		// A parallel wave spawns once every task in it has a brief, all in one action.
		var spawns []Action
		for _, taskID := range wave {
			if blocked[taskID] {
				continue
			}
			task := snap.Tasks[taskID]
			if task.Closeable() {
				if single && task.Open {
					// A single-mode task closes before the next briefs; they share one worktree.
					return closeAction(taskID, index), false
				}
				continue
			}
			if task.Attempts > plan.Profile.Repairs {
				blocked[taskID] = true
				for _, dependent := range BlockedBy(snap.Group.Tasks, taskID) {
					blocked[dependent] = true
				}
				continue
			}
			if task.StaleBrief {
				why := taskID + " has no brief yet"
				if task.Attempts > 0 {
					why = fmt.Sprintf("%s failed and needs a repair brief carrying the failure", taskID)
				}
				return Action{
					Action: "run", Command: "komodo brief " + taskID, Task: taskID, Wave: index + 1,
					Why: why,
				}, false
			}
			spawn := builderSpawn(plan, single, task, taskID, index)
			if single {
				return spawn, false
			}
			spawns = append(spawns, spawn)
		}
		switch {
		case len(spawns) == 1:
			return spawns[0], false
		case len(spawns) > 1:
			return Action{
				Action: "spawn", Wave: index + 1, Spawns: spawns,
				Why: fmt.Sprintf("wave %d has %d briefed tasks and no results; spawn them together", index+1, len(spawns)),
			}, false
		}
		for _, taskID := range wave {
			if !blocked[taskID] && snap.Tasks[taskID].Open {
				return closeAction(taskID, index), false
			}
		}
		if index >= len(snap.Waves) || !snap.Waves[index].Merged {
			return Action{
				Action: "run", Command: fmt.Sprintf("komodo close --wave %d", index+1), Wave: index + 1,
				Why: fmt.Sprintf("wave %d is closed and not merged", index+1),
			}, false
		}
	}
	if !snap.Reviewed && !snap.ReviewSkippable {
		taskID := plan.Group + "-review"
		return Action{
			Action: "spawn", Role: "reviewer", Brief: filepath.Join(StateDir, "briefs", taskID+".md"),
			Task: taskID, Worktree: plan.Worktree,
			Why: "every wave is merged and the diff is unreviewed",
		}, false
	}
	if len(snap.Blocking) > 0 {
		return Action{
			Action: "done",
			Why: fmt.Sprintf("the review left %d finding(s) at or above %s; fix them on %s, then komodo step",
				len(snap.Blocking), plan.Profile.SeverityFloor, plan.Branch),
		}, true
	}
	if snap.Handoff {
		return Action{Action: "done",
			Why: plan.Group + " is committed and handed off; the launcher pushes and opens the pull request on exit"}, true
	}
	if !snap.Shipped {
		return Action{
			Action: "run", Command: "komodo close --group",
			Why: "the review is in and the group is not shipped",
		}, true
	}
	return Action{Action: "done", Why: plan.Group + " is shipped"}, true
}

// closeAction closes a task whose result is in and whose backlog status is still open.
func closeAction(taskID string, index int) Action {
	return Action{
		Action: "run", Command: "komodo close " + taskID + " --gate", Task: taskID, Wave: index + 1,
		Why: taskID + " has a result and is still open",
	}
}

// builderSpawn is the builder spawn for one task with a current brief and no result to close.
func builderSpawn(plan *Plan, single bool, task TaskState, taskID string, index int) Action {
	why := taskID + " has a brief and no result"
	if task.Attempts > 0 {
		why = fmt.Sprintf("%s has a repair brief and %d failed attempt(s)", taskID, task.Attempts)
	}
	worktree := filepath.Join(StateDir, "wt", taskID)
	if single {
		// A single-mode group shares one builder and one worktree, matching brief.go.
		worktree = filepath.Join(StateDir, "wt", plan.Group)
	}
	return Action{
		Action: "spawn", Role: "builder", Brief: filepath.Join(StateDir, "briefs", taskID+".md"),
		Task: taskID, Wave: index + 1, Worktree: worktree, Why: why,
	}
}
