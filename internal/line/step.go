package line

import (
	"fmt"
	"os"
	"path/filepath"

	"komodo/internal/backlog"
)

// Action is the one next thing a session should do. The station order lives here and nowhere else.
type Action struct {
	Action   string   `json:"action"`
	Why      string   `json:"why"`
	Command  string   `json:"command,omitempty"`
	Role     string   `json:"role,omitempty"`
	Brief    string   `json:"brief,omitempty"`
	Task     string   `json:"task,omitempty"`
	Wave     int      `json:"wave,omitempty"`
	Machine  string   `json:"machine,omitempty"`
	Skills   []string `json:"skills"`
	Facets   []string `json:"facets"`
	Commands []string `json:"commands"`
	Until    string   `json:"until,omitempty"`
}

// Step reads the run's state and the results on disk and returns the one next action.
func Step(root, needle string) (*Action, error) {
	plan, err := Next(root, needle)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		state, err := LoadRun(root)
		if err != nil {
			return &Action{Action: "done", Why: "nothing is ready", Skills: []string{}, Facets: []string{}, Commands: []string{}}, nil
		}
		plan, err = PlanForGroup(root, state.Group)
		if err != nil || plan == nil {
			return &Action{Action: "done", Why: "nothing is ready", Skills: []string{}, Facets: []string{}, Commands: []string{}}, err
		}
	}
	state, runErr := LoadRun(root)
	if runErr != nil || state.Group != plan.Group {
		return action(root, plan, Action{
			Action:  "run",
			Command: "komodo next --start " + plan.Group + " --base " + plan.Base,
			Why:     fmt.Sprintf("%s has not been cut yet, from %s", plan.Group, plan.Base),
		}), nil
	}
	full, err := PlanForGroup(root, state.Group)
	if err != nil {
		return nil, err
	}
	if full != nil {
		plan = full
	}
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	for index, wave := range plan.Waves {
		for _, taskID := range wave {
			if HasResult(root, taskID) {
				continue
			}
			briefPath := filepath.Join(StateDir, "briefs", taskID+".md")
			if _, err := os.Stat(filepath.Join(root, briefPath)); err != nil {
				return action(root, plan, Action{
					Action: "run", Command: "komodo brief " + taskID, Task: taskID, Wave: index + 1,
					Why: taskID + " has no brief yet",
				}), nil
			}
			return action(root, plan, Action{
				Action: "spawn", Role: "builder", Brief: briefPath, Task: taskID, Wave: index + 1,
				Why: taskID + " has a brief and no result",
			}), nil
		}
		for _, taskID := range wave {
			task, ok := parsed.Task(taskID)
			if ok && task.Open() {
				return action(root, plan, Action{
					Action: "run", Command: "komodo close " + taskID, Task: taskID, Wave: index + 1,
					Why: taskID + " has a result and is still open",
				}), nil
			}
		}
		if !waveMerged(root, plan, index) {
			return action(root, plan, Action{
				Action: "run", Command: fmt.Sprintf("komodo close --wave %d", index+1), Wave: index + 1,
				Why: fmt.Sprintf("wave %d is closed and not merged", index+1),
			}), nil
		}
	}
	if !reviewed(root, plan) {
		return action(root, plan, Action{
			Action: "spawn", Role: "reviewer", Brief: "komodo diff",
			Why: "every wave is merged and the diff is unreviewed",
		}), nil
	}
	if !shipped(root, plan, parsed) {
		return action(root, plan, Action{
			Action: "run", Command: "komodo close --group",
			Why: "the review is in and the group is not shipped",
		}), nil
	}
	return action(root, plan, Action{Action: "done", Why: plan.Group + " is shipped"}), nil
}

// action fills the machine, skills, facets, and commands a station resolved.
func action(root string, plan *Plan, next Action) *Action {
	next.Skills = []string{}
	next.Facets = []string{}
	next.Commands = []string{}
	if command := VerifyCommand(filepath.Join(root, plan.Worktree)); command != "" {
		next.Commands = append(next.Commands, command)
	}
	if next.Role != "" {
		for _, role := range plan.Roles {
			if role.Name == next.Role {
				next.Machine = role.Machine
				if next.Machine == "" {
					next.Machine = role.Tier
				}
			}
		}
		if next.Machine == "ollama" {
			next.Action = "run"
			next.Command = "komodo machine " + next.Task
			next.Role = ""
		}
	}
	return &next
}

// waveMerged reports whether QC already merged this wave into the group branch.
func waveMerged(root string, plan *Plan, index int) bool {
	entries, err := Book(root).Read("line.jsonl")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Station == "qc" && entry.Wave == index+1 && entry.Outcome == "done" {
			return true
		}
	}
	return false
}

// reviewed reports whether the reviewer already returned a result for this group.
func reviewed(root string, plan *Plan) bool {
	return HasResult(root, plan.Group+"-review")
}

// shipped reports whether every task in the group is closed out.
func shipped(root string, plan *Plan, parsed backlog.Backlog) bool {
	entries, err := Book(root).Read("line.jsonl")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Group == plan.Group {
			return true
		}
	}
	return false
}
