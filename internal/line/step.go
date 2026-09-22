package line

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/mount/ollama"
)

// Action is the one next thing a session should do. The station order lives here and nowhere else.
type Action struct {
	Action   string   `json:"action"`
	Why      string   `json:"why"`
	Command  string   `json:"command,omitempty"`
	Role     string   `json:"role,omitempty"`
	Brief    string   `json:"brief,omitempty"`
	Worktree string   `json:"worktree,omitempty"`
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
	if runErr == nil && needle == "" && state.Group != plan.Group {
		if open, err := openRun(root, state.Group); err == nil && open != nil {
			plan = open
		}
	}
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
			attempt := LoadAttempt(root, taskID)
			if HasResult(root, taskID) && attempt.Count == 0 {
				continue
			}
			if attempt.Count > plan.Profile.Repairs {
				return action(root, plan, Action{
					Action: "done", Task: taskID, Wave: index + 1,
					Why: fmt.Sprintf("%s is blocked after %d repair(s): %s", taskID, plan.Profile.Repairs, firstLine(attempt.Failure)),
				}), nil
			}
			briefPath := filepath.Join(StateDir, "briefs", taskID+".md")
			if staleBrief(root, taskID) {
				why := taskID + " has no brief yet"
				if attempt.Count > 0 {
					why = fmt.Sprintf("%s failed and needs a repair brief carrying the failure", taskID)
				}
				return action(root, plan, Action{
					Action: "run", Command: "komodo brief " + taskID, Task: taskID, Wave: index + 1,
					Why: why,
				}), nil
			}
			why := taskID + " has a brief and no result"
			if attempt.Count > 0 {
				why = fmt.Sprintf("%s has a repair brief and %d failed attempt(s)", taskID, attempt.Count)
			}
			tier := ""
			if task, ok := parsed.Task(taskID); ok {
				tier = task.Tier()
			}
			return actionForTier(root, plan, Action{
				Action: "spawn", Role: "builder", Brief: briefPath, Task: taskID, Wave: index + 1,
				Worktree: filepath.Join(StateDir, "wt", taskID),
				Why:      why,
			}, tier), nil
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
			Task: plan.Group + "-review", Worktree: plan.Worktree,
			Why: "every wave is merged and the diff is unreviewed",
		}), nil
	}
	blocking, _ := SplitFindings(ReviewFindings(root, plan.Group), plan.Profile.SeverityFloor)
	if len(blocking) > 0 {
		return action(root, plan, Action{
			Action: "done",
			Why: fmt.Sprintf("the review left %d finding(s) at or above %s; fix them on %s, then komodo close --group",
				len(blocking), plan.Profile.SeverityFloor, plan.Branch),
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
	return actionForTier(root, plan, next, "")
}

// actionForTier is action, but a task's own tier key, when set, picks the machine over the role's.
func actionForTier(root string, plan *Plan, next Action, taskTier string) *Action {
	next.Skills = []string{}
	next.Facets = []string{}
	next.Commands = []string{}
	if command := VerifyCommand(WorktreePath(root, plan.Worktree)); command != "" {
		next.Commands = append(next.Commands, command)
	}
	if next.Role == "reviewer" {
		if command := BeforeReviewCommand(WorktreePath(root, plan.Worktree)); command != "" {
			next.Commands = append(next.Commands, command)
		}
	}
	if next.Role != "" {
		var matched Role
		for _, role := range plan.Roles {
			if role.Name == next.Role {
				matched = role
				next.Machine = role.Machine
				if next.Machine == "" {
					next.Machine = role.Tier
				}
				if taskTier != "" && taskTier != role.Tier {
					next.Machine = machineFor(plan.Profile, taskTier)
				}
			}
		}
		if next.Machine == "ollama" && (matched.Session || !ollama.Allowed(matched.Tools)) {
			next.Machine = matched.Tier
			if remote, ok := plan.Profile.Tiers.FirstRemote(); ok {
				next.Machine = remote.Provider + "/" + remote.Model
			}
		}
		if next.Machine == "ollama" {
			next.Action = "run"
			next.Command = "komodo machine --role " + next.Role + " " + next.Task
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
	if !HasResult(root, plan.Group+"-review") {
		return false
	}
	return !staleReview(root, plan)
}

// staleReview reports whether the branch moved after the review, which a repair always does.
func staleReview(root string, plan *Plan) bool {
	_, path, err := ReadResultFile(root, plan.Group+"-review")
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	stamp, err := git(WorktreePath(root, plan.Worktree), "log", "-1", "--format=%cI")
	if err != nil {
		return false
	}
	committed, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return false
	}
	return committed.After(info.ModTime())
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

// staleBrief reports whether a task has no brief, or one written before its last failure.
func staleBrief(root, taskID string) bool {
	brief, err := os.Stat(filepath.Join(root, StateDir, "briefs", taskID+".md"))
	if err != nil {
		return true
	}
	attempt, err := os.Stat(attemptPath(root, taskID))
	if err != nil {
		return false
	}
	return brief.ModTime().Before(attempt.ModTime())
}

// openRun is the run's own group while it still has stations left, so a later ready group cannot steal it.
func openRun(root, groupID string) (*Plan, error) {
	plan, err := PlanForGroup(root, groupID)
	if err != nil || plan == nil {
		return nil, err
	}
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	if shipped(root, plan, parsed) {
		return nil, nil
	}
	return plan, nil
}
