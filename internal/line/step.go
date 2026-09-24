package line

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/detect"
	"komodo/internal/facet"
	"komodo/internal/git"
	"komodo/internal/ledger"
	"komodo/internal/mount"
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
	plan, err := PlanForStation(root, needle)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return &Action{Action: "done", Why: "nothing is ready", Skills: []string{}, Facets: []string{}, Commands: []string{}}, nil
	}
	if next := paused(root, plan); next != nil {
		return next, nil
	}
	state, runErr := LoadRun(root)
	if runErr != nil || state.Group != plan.Group {
		return action(root, plan, Action{
			Action:  "run",
			Command: "komodo next --start " + plan.Group + " --base " + plan.Base,
			Why:     fmt.Sprintf("%s has not been cut yet, from %s", plan.Group, plan.Base),
		}), nil
	}
	full, err := planForGroup(root, state.Group)
	if err != nil {
		return nil, err
	}
	if full != nil {
		plan = full
	}
	if next := paused(root, plan); next != nil {
		return next, nil
	}
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	group, _ := parsed.Group(plan.Group)
	blocked := map[string]bool{}
	for index, wave := range plan.Waves {
		for _, taskID := range wave {
			if blocked[taskID] {
				continue
			}
			attempt := LoadAttempt(root, taskID)
			if HasResult(root, taskID) && (attempt.Count == 0 || repairResultReady(root, taskID)) {
				if group.Mode() == "single" {
					// A single-mode task closes before the next briefs; they share one worktree.
					if task, ok := parsed.Task(taskID); ok && task.Open() {
						return action(root, plan, Action{
							Action: "run", Command: "komodo close " + taskID + " --gate", Task: taskID, Wave: index + 1,
							Why: taskID + " has a result and is still open",
						}), nil
					}
				}
				continue
			}
			if attempt.Count > plan.Profile.Repairs {
				blocked[taskID] = true
				for _, dependent := range BlockedBy(group.Tasks, taskID) {
					blocked[dependent] = true
				}
				continue
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
			worktree := filepath.Join(StateDir, "wt", taskID)
			if group.Mode() == "single" {
				// A single-mode group shares one builder and one worktree, matching brief.go.
				worktree = filepath.Join(StateDir, "wt", plan.Group)
			}
			return actionForTier(root, plan, Action{
				Action: "spawn", Role: "builder", Brief: briefPath, Task: taskID, Wave: index + 1,
				Worktree: worktree,
				Why:      why,
			}, tier), nil
		}
		for _, taskID := range wave {
			if blocked[taskID] {
				continue
			}
			task, ok := parsed.Task(taskID)
			if ok && task.Open() {
				return action(root, plan, Action{
					Action: "run", Command: "komodo close " + taskID + " --gate", Task: taskID, Wave: index + 1,
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
	if !reviewed(root, plan) && !reviewSkippable(root, plan) {
		briefPath, err := reviewBrief(root, plan)
		if err != nil {
			return nil, err
		}
		return action(root, plan, Action{
			Action: "spawn", Role: "reviewer", Brief: briefPath,
			Task: plan.Group + "-review", Worktree: plan.Worktree,
			Why: "every wave is merged and the diff is unreviewed",
		}), nil
	}
	stampReview(root, plan)
	blocking, _ := SplitFindings(ReviewFindings(root, plan.Group), plan.Profile.SeverityFloor)
	if len(blocking) > 0 {
		return repairReview(root, plan, blocking), nil
	}
	if _, err := os.Stat(filepath.Join(root, StateDir, "ship.json")); err == nil {
		return action(root, plan, Action{Action: "done",
			Why: plan.Group + " is committed and handed off; the launcher pushes and opens the pull request on exit"}), nil
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
		next.Skills = skillsFor(root, plan, next.Role, next.Task)
		next.Facets = facetsFor(root, WorktreePath(root, next.Worktree), next.Task)
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
		local := mount.LocalMachine()
		if next.Machine == mount.LocalName && (!local.Allowed(matched.Tools) || !fitsLocal(root, next.Brief)) {
			if local.Allowed(matched.Tools) {
				next.Why += "; the brief is larger than the local machine's window, so a remote machine reads it"
			}
			next.Machine = matched.Tier
			if remote := plan.Profile.Tiers.Machine(matched.Tier); remote.Provider != "" && !remote.Local() {
				next.Machine = remote.Provider + "/" + remote.Model
			} else if remote, ok := plan.Profile.Tiers.FirstRemote(); ok {
				next.Machine = remote.Provider + "/" + remote.Model
			}
		}
		if next.Machine == mount.LocalName {
			next.Action = "run"
			next.Command = "komodo machine --role " + next.Role + " " + next.Task
			next.Role = ""
		}
	}
	return &next
}

// fitsLocal reports whether a brief on disk fits the local machine's window; no brief always fits.
func fitsLocal(root, brief string) bool {
	if brief == "" {
		return true
	}
	info, err := os.Stat(filepath.Join(root, brief))
	if err != nil {
		return true
	}
	return mount.LocalMachine().Fits(int(info.Size()))
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

// repairReview walks one fix round for blocking findings: brief, builder spawn, then close --fix,
// and stops for a person once the profile's review_repairs rounds are spent.
func repairReview(root string, plan *Plan, blocking []Finding) *Action {
	rounds := FixRounds(root, plan.Group)
	if rounds >= plan.Profile.ReviewRepairs {
		titles := make([]string, 0, len(blocking))
		for _, finding := range blocking {
			titles = append(titles, fmt.Sprintf("%s:%d %s", finding.File, finding.Line, finding.Title))
		}
		return action(root, plan, Action{
			Action: "done",
			Why: fmt.Sprintf(
				"the review left %d finding(s) at or above %s after %d fix round(s): %s; fix them on %s, then komodo step",
				len(blocking), plan.Profile.SeverityFloor, rounds, strings.Join(titles, "; "), plan.Branch,
			),
		})
	}
	taskID := plan.Group + "-fix"
	if staleFixBrief(root, plan.Group) {
		return action(root, plan, Action{
			Action: "run", Command: "komodo brief --review " + plan.Group, Task: taskID,
			Why: fmt.Sprintf("the review left %d finding(s) at or above %s; fix round %d needs a brief",
				len(blocking), plan.Profile.SeverityFloor, rounds+1),
		})
	}
	if !fixResultReady(root, plan.Group) {
		return action(root, plan, Action{
			Action: "spawn", Role: "builder", Brief: filepath.Join(StateDir, "briefs", taskID+".md"),
			Task: taskID, Worktree: plan.Worktree,
			Why: fmt.Sprintf("fix round %d has a brief and no result", rounds+1),
		})
	}
	return action(root, plan, Action{
		Action: "run", Command: "komodo close --fix " + plan.Group, Task: taskID,
		Why: fmt.Sprintf("fix round %d has a result to gate and commit", rounds+1),
	})
}

// FixRounds counts the review fix rounds the ledger holds for a group, passed or failed.
func FixRounds(root, groupID string) int {
	entries, err := Book(root).Read("line.jsonl")
	if err != nil {
		return 0
	}
	rounds := 0
	for _, entry := range entries {
		if entry.Station == "fix" && entry.Group == groupID {
			rounds++
		}
	}
	return rounds
}

// staleFixBrief reports whether a group has no fix brief, or one older than its review or last failed fix.
func staleFixBrief(root, groupID string) bool {
	brief, err := os.Stat(filepath.Join(root, StateDir, "briefs", groupID+"-fix.md"))
	if err != nil {
		return true
	}
	if _, path, err := ReadResultFile(root, groupID+"-review"); err == nil {
		if review, err := os.Stat(path); err == nil && brief.ModTime().Before(review.ModTime()) {
			return true
		}
	}
	return staleBrief(root, groupID+"-fix")
}

// fixResultReady reports whether the builder wrote a fix result after the current fix brief.
func fixResultReady(root, groupID string) bool {
	_, path, err := ReadResultFile(root, groupID+"-fix")
	if err != nil {
		return false
	}
	result, err := os.Stat(path)
	if err != nil {
		return false
	}
	brief, err := os.Stat(filepath.Join(root, StateDir, "briefs", groupID+"-fix.md"))
	if err != nil {
		return false
	}
	return result.ModTime().After(brief.ModTime())
}

// stampReview records the review station once per result: the seconds from the review brief to
// its result, and how many findings it returned, so a spawned review is timed like a local one.
func stampReview(root string, plan *Plan) {
	taskID := plan.Group + "-review"
	_, path, err := ReadResultFile(root, taskID)
	if err != nil {
		return
	}
	result, err := os.Stat(path)
	if err != nil {
		return
	}
	if entries, err := Book(root).Read("line.jsonl"); err == nil {
		for _, entry := range entries {
			if entry.Station == "review" && entry.Group == plan.Group && !entry.At.Before(result.ModTime()) {
				return
			}
		}
	}
	entry := ledger.Entry{Group: plan.Group, Task: taskID, Station: "review", Role: "reviewer",
		Findings: len(ReviewFindings(root, plan.Group)), Outcome: "done"}
	if brief, err := os.Stat(filepath.Join(root, StateDir, "briefs", taskID+".md")); err == nil {
		entry.Seconds = result.ModTime().Sub(brief.ModTime()).Seconds()
	}
	fillUsage(root, taskID, result.ModTime(), &entry)
	Stamp(root, entry)
}

// staleReview reports whether the branch moved after the review, which a repair always does;
// ship's own status-and-changelog commit is excluded, since it never invalidates a review already past it.
func staleReview(root string, plan *Plan) bool {
	_, path, err := ReadResultFile(root, plan.Group+"-review")
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	log, err := git.Run(WorktreePath(root, plan.Worktree), "log", "--format=%cI%x09%s")
	if err != nil {
		return false
	}
	shipSubject := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
	for _, entry := range strings.Split(log, "\n") {
		stamp, subject, found := strings.Cut(entry, "\t")
		if !found || subject == shipSubject {
			continue
		}
		committed, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			return false
		}
		return committed.After(info.ModTime())
	}
	return false
}

// shipped reports whether every task in the group is closed out.
func shipped(root string, plan *Plan, parsed backlog.Backlog) bool {
	entries, err := Book(root).Read("line.jsonl")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Group == plan.Group && entry.Outcome == "done" {
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

// repairResultReady reports whether a repair already wrote its result after a brief that carries
// the latest failure, so the task is ready to close rather than spawn a builder again.
func repairResultReady(root, taskID string) bool {
	if staleBrief(root, taskID) {
		return false
	}
	_, path, err := ReadResultFile(root, taskID)
	if err != nil {
		return false
	}
	result, err := os.Stat(path)
	if err != nil {
		return false
	}
	brief, err := os.Stat(filepath.Join(root, StateDir, "briefs", taskID+".md"))
	if err != nil {
		return false
	}
	return result.ModTime().After(brief.ModTime())
}

// paused is the wait action a station returns once the profile has closed its window for a new
// wave, carrying when it reopens, or nil while the plan still has work to walk.
func paused(root string, plan *Plan) *Action {
	if plan.WaitUntil == "" {
		return nil
	}
	return action(root, plan, Action{
		Action: "done", Until: plan.WaitUntil,
		Why: fmt.Sprintf("%s is paused until the window resets at %s", plan.Group, plan.WaitUntil),
	})
}

// reviewBrief fills the reviewer role from the group's diff, tasks, and standards, writes it to
// .komodo/briefs/<group>-review.md in the root and the group worktree, and returns that path.
func reviewBrief(root string, plan *Plan) (string, error) {
	definition, err := LoadRole(root, "reviewer")
	if err != nil {
		return "", err
	}
	input, err := DiffFor(root, plan)
	if err != nil {
		return "", err
	}
	slots := map[string]string{
		"group_id": plan.Group, "title": plan.Title,
		"tasks": input.Tasks, "standards": input.Standards, "diff": input.Diff, "base": plan.Base,
	}
	text, err := Fill(definition.Body, slots)
	if err != nil {
		return "", err
	}
	taskID := plan.Group + "-review"
	result := filepath.Join(StateDir, "results", taskID+".json")
	text = strings.TrimSpace(text) + resultLine(result, SchemaText(root, "reviewer"))
	briefPath := filepath.Join(StateDir, "briefs", taskID+".md")
	for _, base := range []string{root, WorktreePath(root, plan.Worktree)} {
		full := filepath.Join(base, briefPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			return "", err
		}
	}
	return briefPath, nil
}

// reviewSkippable reports whether the diff sits at or under the profile's review-skip-lines cap.
func reviewSkippable(root string, plan *Plan) bool {
	if plan.Profile.ReviewSkipLines <= 0 {
		return false
	}
	input, err := DiffFor(root, plan)
	if err != nil {
		return false
	}
	return input.Lines <= plan.Profile.ReviewSkipLines
}

// skillsFor names the standards a role reads for one task's own files, or every task's files in
// the plan when the id names no task, which is what a review spawn's pseudo-task does.
func skillsFor(root string, plan *Plan, role, taskID string) []string {
	standards, err := LoadStandards(root)
	if err != nil {
		return []string{}
	}
	selected := StandardsFor(standards, taskFiles(plan, taskID), role)
	names := make([]string, 0, len(selected))
	for _, standard := range selected {
		names = append(names, standard.Name)
	}
	return names
}

// taskFiles is one task's own files, or every task's files in the plan when the id names none.
func taskFiles(plan *Plan, taskID string) []string {
	for _, task := range plan.Tasks {
		if task.ID == taskID {
			return task.Files
		}
	}
	var files []string
	for _, task := range plan.Tasks {
		files = append(files, task.Files...)
	}
	return files
}

// facetsFor names the facets a worktree's tree detects, plus what the task itself declares.
func facetsFor(root, worktree, taskID string) []string {
	tree, _ := detect.Detect(worktree)
	var declared []string
	if path, err := backlog.Find(root); err == nil {
		if parsed, err := backlog.Load(path); err == nil {
			if task, ok := parsed.Task(taskID); ok {
				declared = task.Facets()
			}
		}
	}
	names, err := facet.Select(root, tree, declared)
	if err != nil || names == nil {
		return []string{}
	}
	return names
}
