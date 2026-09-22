package line

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
)

// PlanTask is one task as intake prints it.
type PlanTask struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Priority  string   `json:"priority"`
	Type      string   `json:"type"`
	Owner     string   `json:"owner"`
	Files     []string `json:"files"`
	Dirs      []string `json:"dirs"`
	DependsOn []string `json:"depends_on,omitempty"`
	Done      bool     `json:"done"`
}

// Plan is what intake prints: the group, its waves, and the machine resolved per role.
type Plan struct {
	Group    string     `json:"group"`
	Title    string     `json:"title"`
	Type     string     `json:"type"`
	Version  string     `json:"version"`
	Mode     string     `json:"mode"`
	Base     string     `json:"base"`
	Branch   string     `json:"branch"`
	Worktree string     `json:"worktree"`
	Tasks    []PlanTask `json:"tasks"`
	Waves    [][]string `json:"waves"`
	Skipped  []string   `json:"skipped,omitempty"`
	Roles    []Role     `json:"roles"`
}

// Next builds the plan for the next ready group, or for the group holding the named task.
func Next(root, needle string) (*Plan, error) {
	parsed, group, ok, err := groupFor(root, needle)
	if err != nil || !ok {
		return nil, err
	}
	plan, err := buildPlan(root, parsed, group, false)
	if err != nil || plan == nil {
		return nil, err
	}
	return plan, nil
}

// PlanForGroup builds a group's plan including the tasks that already closed, which is what step walks.
func PlanForGroup(root, groupID string) (*Plan, error) {
	parsed, group, ok, err := groupFor(root, groupID)
	if err != nil || !ok {
		return nil, err
	}
	return buildPlan(root, parsed, group, true)
}

// groupFor reads BACKLOG.md and picks the group a needle names, or the next ready one.
func groupFor(root, needle string) (backlog.Backlog, backlog.Group, bool, error) {
	path, err := backlog.Find(root)
	if err != nil {
		return backlog.Backlog{}, backlog.Group{}, false, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return backlog.Backlog{}, backlog.Group{}, false, err
	}
	group, ok := pick(parsed, needle)
	return parsed, group, ok, nil
}

// buildPlan renders one group as intake prints it, optionally keeping the tasks that closed.
func buildPlan(root string, parsed backlog.Backlog, group backlog.Group, includeClosed bool) (*Plan, error) {
	var done []string
	var tasks []backlog.Task
	for _, task := range group.Tasks {
		if task.Owner() != "agent" || task.Status == "REFINEMENT" {
			continue
		}
		if !includeClosed && !task.Ready() {
			continue
		}
		if HasResult(root, task.ID) || !task.Open() {
			done = append(done, task.ID)
		}
		tasks = append(tasks, task)
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	plan := &Plan{
		Group: group.ID, Title: group.Title, Type: group.Type(),
		Version: group.Version(), Mode: group.Mode(), Skipped: done,
		Base: DefaultBase(root), Branch: BranchName(group.Type(), group.Slug()),
	}
	plan.Worktree = filepath.Join(StateDir, "wt", group.ID)
	for _, task := range tasks {
		plan.Tasks = append(plan.Tasks, PlanTask{
			ID: task.ID, Title: task.Title, Status: task.Status, Priority: task.Priority,
			Type: task.Type(), Owner: task.Owner(), Files: task.Files(), Dirs: task.Dirs(),
			DependsOn: task.DependsOn(), Done: contains(done, task.ID),
		})
	}
	skip := done
	if includeClosed {
		skip = nil
	}
	waves, err := planWaves(group, tasks, skip)
	if err != nil {
		return nil, err
	}
	plan.Waves = waves
	roles, err := LoadRoles(root)
	if err != nil {
		return nil, err
	}
	plan.Roles = roles
	return plan, nil
}

// planWaves splits a group into waves, or into one wave when the group runs single.
func planWaves(group backlog.Group, tasks []backlog.Task, done []string) ([][]string, error) {
	if group.Mode() == "single" {
		var wave []string
		for _, task := range tasks {
			if !contains(done, task.ID) {
				wave = append(wave, task.ID)
			}
		}
		if len(wave) == 0 {
			return nil, nil
		}
		return [][]string{wave}, nil
	}
	waves, err := Waves(tasks, done)
	if err != nil {
		return nil, err
	}
	out := make([][]string, 0, len(waves))
	for _, wave := range waves {
		ids := make([]string, 0, len(wave))
		for _, task := range wave {
			ids = append(ids, task.ID)
		}
		out = append(out, ids)
	}
	return out, nil
}

// pick returns the named group, the group of a named task, or the next ready group.
func pick(parsed backlog.Backlog, needle string) (backlog.Group, bool) {
	if needle == "" {
		return parsed.NextGroup()
	}
	if task, ok := parsed.Task(needle); ok {
		return parsed.Group(task.GroupID)
	}
	return parsed.Group(needle)
}

// Start cuts the group branch in its own worktree from the base and records the choice.
func Start(root string, plan *Plan, base string) (RunState, error) {
	if base != "" {
		plan.Base = base
	}
	if err := Fetch(root, plan.Base); err != nil {
		return RunState{}, err
	}
	path := filepath.Join(root, plan.Worktree)
	if _, err := os.Stat(path); err != nil {
		if err := AddWorktree(root, plan.Branch, plan.Base, path); err != nil {
			return RunState{}, err
		}
	}
	state := RunState{
		Run:     fmt.Sprintf("%s-%d", plan.Group, time.Now().Unix()),
		Group:   plan.Group,
		Base:    plan.Base,
		Branch:  plan.Branch,
		Started: time.Now().UTC(),
	}
	state.Worktree = path
	if err := Book(root).TruncateRun(); err != nil {
		return state, err
	}
	if err := SaveRun(root, state); err != nil {
		return state, err
	}
	Stamp(root, ledger.Entry{Run: state.Run, Group: state.Group, Station: "intake", Outcome: "started"})
	return state, nil
}

// contains reports whether the slice holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
