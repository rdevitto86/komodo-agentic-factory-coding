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
	Spawns   []Action `json:"spawns,omitempty"`
}

// Step reads one snapshot, decides with Next, then writes the review stamp and brief it needs; a
// failed lease take stops the builder instead of sending it to write with no lease held.
func Step(root, needle string) (*Action, error) {
	snap, err := LoadSnapshot(root, needle)
	if err != nil {
		return nil, err
	}
	act, err := stepFrom(root, snap)
	if err != nil {
		return act, err
	}
	if snap.Plan != nil {
		if err := takeLeases(root, snap.Plan, act); err != nil {
			return nil, err
		}
	}
	return act, nil
}

// stepFrom decides the next action for one loaded snapshot.
func stepFrom(root string, snap Snapshot) (*Action, error) {
	next, pastReview := decide(snap)
	if snap.Plan == nil {
		return &next, nil
	}
	if next.Role == "reviewer" {
		if _, err := ReviewBrief(root, snap.Plan); err != nil {
			return nil, err
		}
	}
	if pastReview {
		stampReview(root, snap.Plan)
		blocking, _ := SplitFindings(ReviewFindings(root, snap.Plan.Group), snap.Plan.Profile.SeverityFloor)
		if len(blocking) > 0 {
			return repairReview(root, snap.Plan, blocking), nil
		}
	}
	if len(next.Spawns) > 0 {
		return waveSpawn(root, snap, next), nil
	}
	tier := taskTier(snap, next)
	return actionForTier(root, snap.Plan, next, tier), nil
}

// taskTier is the tier a task's action resolves on: its own tier key, else the role's.
func taskTier(snap Snapshot, next Action) string {
	return snap.Tasks[next.Task].Tier
}

// waveSpawn resolves each spawn in a wave like a single one; a spawn that resolves to a local
// command runs alone, since a command is one step, not a spawn.
func waveSpawn(root string, snap Snapshot, next Action) *Action {
	spawns := next.Spawns
	next.Spawns = nil
	wave := actionForTier(root, snap.Plan, next, "")
	for _, spawn := range spawns {
		tier := taskTier(snap, spawn)
		resolved := actionForTier(root, snap.Plan, spawn, tier)
		if resolved.Action != "spawn" {
			return resolved
		}
		wave.Spawns = append(wave.Spawns, *resolved)
	}
	return wave
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
	if command := VerifyCommand(root, WorktreePath(root, plan.Worktree)); command != "" {
		next.Commands = append(next.Commands, command)
	}
	if next.Role == "reviewer" {
		if command := BeforeReviewCommand(root, WorktreePath(root, plan.Worktree)); command != "" {
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
		if entry.Station == "qc" && entry.Group == plan.Group && entry.Wave == index+1 && entry.Outcome == "done" {
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

// staleReview reports whether a group commit past the base, by the author date a rebase keeps, postdates the review;
// ship's own status-and-changelog commit and its credential-note commit are excluded, since neither is new work.
func staleReview(root string, plan *Plan) bool {
	_, path, err := ReadResultFile(root, plan.Group+"-review")
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	dir := WorktreePath(root, plan.Worktree)
	// A merge only brings in the base or the pushed branch, never new group work.
	args := []string{"log", "--no-merges", "--format=%aI%x09%s"}
	if plan.Base != "" {
		base := StartRef(dir, plan.Base)
		if _, err := git.Run(dir, "rev-parse", "--verify", "--quiet", base); err == nil {
			args = append(args, base+"..HEAD")
		}
	}
	log, err := git.Run(dir, args...)
	if err != nil {
		return false
	}
	shipSubject := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
	noteSubject := credentialNoteSubject(plan.Title, plan.Group)
	for _, entry := range strings.Split(log, "\n") {
		stamp, subject, found := strings.Cut(entry, "\t")
		if !found || subject == shipSubject || subject == noteSubject {
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

// shipped reports whether the ledger holds a done ship entry for the group; it reads nothing else.
func shipped(root string, plan *Plan) bool {
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

// ReviewBrief fills the reviewer role from the group's diff, tasks, and standards, writes it to
// .komodo/briefs/<group>-review.md in the root and the group worktree, and returns that path.
func ReviewBrief(root string, plan *Plan) (string, error) {
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
	if parsed, err := backlog.LoadRoot(root); err == nil {
		if task, ok := parsed.Task(taskID); ok {
			declared = task.Facets()
		}
	}
	names, err := facet.Select(root, tree, declared)
	if err != nil || names == nil {
		return []string{}
	}
	return names
}
