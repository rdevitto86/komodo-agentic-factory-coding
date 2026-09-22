package line

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
	"komodo/internal/profile"
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

	Profile   profile.Profile `json:"profile"`
	WaitUntil string          `json:"wait_until,omitempty"`
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
	plan, err := buildPlan(root, parsed, group, true)
	if err != nil || plan == nil {
		return plan, err
	}
	return plan, nil
}

// PlanForRun is the plan for the group a run has open, or the next ready group when none is.
func PlanForRun(root string) (*Plan, error) {
	state, err := LoadRun(root)
	if err != nil || state.Group == "" {
		return Next(root, "")
	}
	plan, err := PlanForGroup(root, state.Group)
	if err != nil || plan == nil {
		return Next(root, "")
	}
	return plan, nil
}

// pinWaves restores the waves the run recorded, so every station numbers them the same way.
func pinWaves(root string, plan *Plan) {
	state, err := LoadRun(root)
	if err != nil || state.Group != plan.Group || len(state.Waves) == 0 {
		return
	}
	plan.Waves = state.Waves
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
		Base: groupBase(root, group), Branch: BranchName(group.Type(), group.Slug()),
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
	chosen := profile.Select(root)
	if path := profile.MachineOverlayPath(); path != "" {
		chosen = profile.Overlay(chosen, path)
	}
	plan.Profile = chosen
	for index := range roles {
		roles[index].Machine = machineFor(chosen, roles[index].Tier)
	}
	plan.Roles = roles
	if chosen.MaxParallel > 0 && plan.Mode != "single" {
		plan.Waves = splitByParallel(plan.Waves, chosen.MaxParallel)
	}
	pinWaves(root, plan)
	if chosen.Paused() && len(plan.Waves) > 0 {
		plan.WaitUntil = chosen.WaitUntil().Format(time.RFC3339)
		plan.Waves = nil
	}
	return plan, nil
}

// machineFor names the machine a tier resolved to, or the tier when no mount is installed.
func machineFor(chosen profile.Profile, tier string) string {
	machine := chosen.Tiers.Machine(tier)
	if machine.Provider == "" {
		return tier
	}
	if machine.Provider == "ollama" {
		return "ollama"
	}
	return machine.Provider + "/" + machine.Model
}

// splitByParallel caps how many tasks a wave may run at once.
func splitByParallel(waves [][]string, limit int) [][]string {
	var out [][]string
	for _, wave := range waves {
		for start := 0; start < len(wave); start += limit {
			end := start + limit
			if end > len(wave) {
				end = len(wave)
			}
			out = append(out, wave[start:end])
		}
	}
	return out
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

// groupBase is the branch a group declares, or the remote's default when it declares none.
func groupBase(root string, group backlog.Group) string {
	if base := group.Base(); base != "" {
		return base
	}
	return DefaultBase(root)
}

// Start cuts the group branch in its own worktree from the base and records the choice.
func Start(root string, plan *Plan, base string) (RunState, error) {
	if base != "" {
		plan.Base = base
	}
	if err := Fetch(root, plan.Base); err != nil {
		return RunState{}, err
	}
	path := WorktreePath(root, plan.Worktree)
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
	state.Waves = plan.Waves
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
