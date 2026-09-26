package line

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/comments"
	"komodo/internal/git"
	"komodo/internal/ledger"
	"komodo/internal/mount"
	"komodo/internal/proc"
	profilepkg "komodo/internal/profile"
)

// MaxRepairs is how many times one task may come back for a repair when the profile names no count.
const MaxRepairs = 1

// repairLimit is the profile's repair count, which the overlay may lower, else MaxRepairs.
func repairLimit(root string) int {
	if limit := resolveProfile(root).Repairs; limit > 0 {
		return limit
	}
	return MaxRepairs
}

// Outcome is what the output device decided about one task.
type Outcome struct {
	Task     string   `json:"task"`
	Status   string   `json:"status"`
	Attempt  int      `json:"attempt"`
	Problems []string `json:"problems,omitempty"`
	Failure  string   `json:"failure,omitempty"`
}

// Attempt records how often a task has failed and what it failed with.
type Attempt struct {
	Count   int    `json:"count"`
	Failure string `json:"failure"`
	Diff    string `json:"diff,omitempty"`
}

// CloseTask validates a task's result, reruns its checks, and records its status in the run.
func CloseTask(root, taskID string, runGate bool) (*Outcome, error) {
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	task, ok := parsed.Task(taskID)
	if !ok {
		return nil, fmt.Errorf("no task %s in %s", taskID, path)
	}
	cwd := TaskWorktree(root, taskID)
	started := time.Now()
	outcome := &Outcome{Task: taskID}
	problems := checkResult(root, taskID)
	problems = append(problems, runDoneWhen(cwd, task)...)
	problems = append(problems, lintComments(cwd, task)...)
	if runGate && len(problems) == 0 && isToolkit(root) {
		if err := gateCommand(cwd); err != nil {
			problems = append(problems, "gate: "+err.Error())
		}
	}
	outcome.Problems = problems
	stampBuild(root, taskID, started)
	entry := ledger.Entry{Task: taskID, Station: "close", Seconds: Since(started)}
	if len(problems) == 0 {
		if err := commitTask(cwd, task, commitBranch(root, parsed, task)); err != nil {
			outcome.Problems = []string{"commit: " + err.Error()}
			return outcome, nil
		}
		outcome.Status = "DONE"
		clearAttempt(root, taskID)
		entry.Outcome = "done"
		Stamp(root, entry)
		return outcome, RecordStatus(root, taskID, "DONE")
	}
	entry.FailureClass = FailureClass(problems)
	attempt, err := bumpAttempt(root, taskID, strings.Join(problems, "\n"), diffOf(cwd))
	if err != nil {
		return nil, err
	}
	outcome.Attempt = attempt.Count
	outcome.Failure = attempt.Failure
	if attempt.Count > repairLimit(root) {
		outcome.Status = "BLOCKED"
		entry.Outcome = "blocked"
		Stamp(root, entry)
		return outcome, RecordStatus(root, taskID, "BLOCKED")
	}
	outcome.Status = "IN_PROGRESS"
	entry.Outcome = "repair"
	Stamp(root, entry)
	return outcome, RecordStatus(root, taskID, "IN_PROGRESS")
}

// CloseFix reruns the group's done_when, comment lint, and gate on a fix round, commits it, and clears the review.
func CloseFix(root string, plan *Plan) (*Outcome, error) {
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	task, _ := fixTask(parsed, plan)
	cwd := WorktreePath(root, plan.Worktree)
	started := time.Now()
	round := FixRounds(root, plan.Group) + 1
	outcome := &Outcome{Task: task.ID}
	problems := checkResult(root, task.ID)
	for _, planned := range plan.Tasks {
		if groupTask, ok := parsed.Task(planned.ID); ok {
			problems = append(problems, runDoneWhen(cwd, groupTask)...)
		}
	}
	problems = append(problems, lintComments(cwd, task)...)
	if len(problems) == 0 && isToolkit(root) {
		if err := gateCommand(cwd); err != nil {
			problems = append(problems, "gate: "+err.Error())
		}
	}
	outcome.Problems = problems
	entry := ledger.Entry{Group: plan.Group, Task: fmt.Sprintf("%s-%d", task.ID, round), Station: "fix", Role: "builder"}
	fillUsage(root, task.ID, time.Now(), &entry)
	if len(problems) == 0 {
		if err := commitTask(cwd, task, plan.Branch); err != nil {
			outcome.Problems = []string{"commit: " + err.Error()}
			return outcome, nil
		}
		outcome.Status = "DONE"
		clearAttempt(root, task.ID)
		for _, result := range ResultPaths(root, plan.Group+"-review") {
			_ = os.Remove(result)
		}
		entry.Seconds, entry.Outcome = Since(started), "done"
		Stamp(root, entry)
		return outcome, nil
	}
	attempt, err := bumpAttempt(root, task.ID, strings.Join(problems, "\n"), diffOf(cwd))
	if err != nil {
		return nil, err
	}
	outcome.Status, outcome.Attempt, outcome.Failure = "IN_PROGRESS", attempt.Count, attempt.Failure
	entry.Seconds, entry.Outcome, entry.FailureClass = Since(started), "repair", FailureClass(problems)
	Stamp(root, entry)
	return outcome, nil
}

// TaskWorktree is where a task is built: its own worktree, else the run's, else the repo root.
func TaskWorktree(root, taskID string) string {
	path := filepath.Join(root, StateDir, "wt", taskID)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	if state, err := RunFor(root, taskID); err == nil && state.Worktree != "" {
		if info, err := os.Stat(state.Worktree); err == nil && info.IsDir() {
			return state.Worktree
		}
	}
	return root
}

// checkResult validates the result JSON against the builder schema; a task's role is always
// builder, never whatever the untrusted result claims, so it cannot pick a lighter check.
func checkResult(root, taskID string) []string {
	result, err := ReadResult(root, taskID)
	if err != nil {
		return []string{"result: " + err.Error()}
	}
	schema, err := LoadSchema(root, "builder")
	if err != nil {
		return []string{"schema: " + err.Error()}
	}
	return Validate(schema, any(result))
}

// runDoneWhen reruns every done_when command in the worktree under the task's clock and names each failure.
func runDoneWhen(cwd string, task backlog.Task) []string {
	var problems []string
	for _, command := range task.DoneWhen() {
		ran := proc.Shell(cwd, command, TaskTimeout(task))
		if !ran.OK() {
			problems = append(problems, fmt.Sprintf("done_when `%s` failed: %v\n%s",
				command, ran.Err(), Clip(ran.Output, 4000, "output")))
		}
	}
	return problems
}

// TaskTimeout is the wall clock a task's commands get: its own timeout key, else the station default.
func TaskTimeout(task backlog.Task) time.Duration {
	if parsed, err := time.ParseDuration(task.Timeout()); err == nil && parsed > 0 {
		return parsed
	}
	return CommandTimeout
}

// lintComments runs the comment lint over the files a task named.
func lintComments(cwd string, task backlog.Task) []string {
	out, err := comments.Check(cwd, task.Files(), "nonobvious")
	if err != nil {
		return []string{"comments: " + err.Error()}
	}
	return out
}

// commitBranch is the branch cwd must be on to commit a task: its own task branch, or the
// group branch for a single-mode group, since close already commits every task there directly.
func commitBranch(root string, parsed backlog.Backlog, task backlog.Task) string {
	if group, ok := parsed.Group(task.GroupID); ok && group.Mode() == "single" {
		if state, err := LoadRunFor(root, task.GroupID); err == nil && state.Branch != "" {
			return state.Branch
		}
	}
	return TaskBranch(task.ID)
}

// commitTask commits a passing task onto branch, so QC has something to merge, and refuses
// when cwd sits on any other branch, which is someone else's checkout, not the task's own.
func commitTask(cwd string, task backlog.Task, branch string) error {
	if _, err := git.Run(cwd, "rev-parse", "--git-dir"); err != nil {
		return nil
	}
	current, err := git.Run(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if current != branch {
		return fmt.Errorf("cwd is on %s, not %s; refusing to commit onto the wrong checkout", current, branch)
	}
	status, err := git.Run(cwd, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	if err := stageWork(cwd, task.Files()); err != nil {
		return err
	}
	if err := unstageBuilt(cwd, task); err != nil {
		return err
	}
	if staged, err := git.Run(cwd, "diff", "--cached", "--name-only"); err != nil || strings.TrimSpace(staged) == "" {
		return err
	}
	message := fmt.Sprintf("%s: %s (%s)", task.Type(), task.Title, task.ID)
	_, err = git.Run(cwd, "commit", "-m", message)
	return err
}

// stageWork stages every change in cwd except the state dir and each mount's rendered project copies, unless declared.
func stageWork(cwd string, declared []string) error {
	args := []string{"add", "-A", "--", "."}
	var rescued []string
	for _, excluded := range append([]string{StateDir}, mount.ProjectPaths(cwd)...) {
		covered := false
		var inside []string
		for _, file := range declared {
			file = strings.TrimSuffix(strings.TrimPrefix(strings.ReplaceAll(file, "\\", "/"), "./"), "/")
			switch {
			case file == excluded || strings.HasPrefix(excluded, file+"/"):
				covered = true
			case strings.HasPrefix(file, excluded+"/"):
				inside = append(inside, file)
			}
		}
		if covered || ignoredAndUntracked(cwd, excluded) {
			continue
		}
		args = append(args, ":(exclude,literal)"+excluded)
		rescued = append(rescued, inside...)
	}
	if _, err := git.Run(cwd, args...); err != nil {
		return err
	}
	for _, file := range rescued {
		// A declared file under an excluded path is staged on its own, when git sees a change there.
		if changed, err := git.Run(cwd, "status", "--porcelain", "--", file); err != nil || changed == "" {
			continue
		}
		if _, err := git.Run(cwd, "add", "-A", "--", file); err != nil {
			return err
		}
	}
	return nil
}

// ignoredAndUntracked reports a path git ignores and tracks nothing under, so add -A skips it unexcluded.
// Excluding such a path makes git add exit 1 once it exists on disk.
func ignoredAndUntracked(cwd, path string) bool {
	if _, err := git.Run(cwd, "check-ignore", "-q", "--", path); err != nil {
		return false
	}
	tracked, err := git.Run(cwd, "ls-files", "--", path)
	return err == nil && tracked == ""
}

// Built are the regenerated paths a task branch never carries, because they conflict on every merge.
var Built = []string{"bin"}

// unstageBuilt drops the regenerated artifacts from a task's commit, unless the task declares one.
func unstageBuilt(cwd string, task backlog.Task) error {
	for _, path := range Built {
		if declares(task, path) {
			continue
		}
		if _, err := os.Stat(filepath.Join(cwd, path)); err != nil {
			continue
		}
		if _, err := git.Run(cwd, "reset", "--quiet", "HEAD", "--", path); err != nil {
			return err
		}
	}
	return nil
}

// declares reports whether a task's own files list covers a built path.
func declares(task backlog.Task, path string) bool {
	for _, file := range task.Files() {
		clean := strings.ReplaceAll(file, "\\", "/")
		if clean == path || strings.HasPrefix(clean, path+"/") {
			return true
		}
	}
	return false
}

// isToolkit reports whether this repo is the toolkit, which gates its own commits.
func isToolkit(root string) bool {
	_, err := os.Stat(filepath.Join(root, "cmd", "komodo", "main.go"))
	return err == nil
}

// gateCommand runs the local gate in the worktree under its own wall clock.
func gateCommand(cwd string) error {
	ran := proc.Exec(cwd, GateTimeout, "go", "run", "./cmd/komodo", "gate")
	if !ran.OK() {
		return fmt.Errorf("%v\n%s", ran.Err(), Clip(ran.Output, 4000, "gate"))
	}
	return nil
}

// diffOf is the worktree's own diff, which a repair brief carries back to the machine.
func diffOf(cwd string) string {
	out, err := git.Run(cwd, "diff", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

// attemptPath is where a task's failure record lives.
func attemptPath(root, taskID string) string {
	return filepath.Join(root, StateDir, "attempts", taskID+".json")
}

// LoadAttempt reads what a task failed with last time, if anything.
func LoadAttempt(root, taskID string) Attempt {
	var attempt Attempt
	data, err := os.ReadFile(attemptPath(root, taskID))
	if err != nil {
		return attempt
	}
	_ = json.Unmarshal(data, &attempt)
	return attempt
}

// bumpAttempt records one more failure and returns the running count.
func bumpAttempt(root, taskID, failure, diff string) (Attempt, error) {
	attempt := LoadAttempt(root, taskID)
	attempt.Count++
	attempt.Failure = failure
	attempt.Diff = diff
	path := attemptPath(root, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return attempt, err
	}
	data, err := json.MarshalIndent(attempt, "", "  ")
	if err != nil {
		return attempt, err
	}
	return attempt, os.WriteFile(path, append(data, '\n'), 0o644)
}

// clearAttempt drops a task's failure record once it closes.
func clearAttempt(root, taskID string) {
	_ = os.Remove(attemptPath(root, taskID))
}

// RepairText is what the failure slot carries into the next brief, the output and the diff.
func RepairText(root, taskID string) string {
	attempt := LoadAttempt(root, taskID)
	if attempt.Count == 0 {
		return ""
	}
	text := attempt.Failure
	if attempt.Diff != "" {
		text += "\n\n# The diff your last attempt left\n" + attempt.Diff
	}
	return text
}

// fillUsage asks the installed mount what the machine spent, and leaves the fields empty when it cannot say.
func fillUsage(root, taskID string, until time.Time, entry *ledger.Entry) {
	since := briefTime(root, taskID)
	for _, host := range mount.Active() {
		if host.Usage == nil || host.Installed == nil || !host.Installed(root) {
			continue
		}
		usage, ok := host.Usage(root, taskID, since, until)
		if !ok {
			continue
		}
		entry.Host = host.Name
		entry.TokensIn, entry.TokensOut, entry.Turns = usage.TokensIn, usage.TokensOut, usage.Turns
		return
	}
}

// stampBuild records what the machine spent between its brief and this close, which is the build
// itself, naming the machine the profile resolved for the task's tier.
func stampBuild(root, taskID string, closed time.Time) {
	written := briefTime(root, taskID)
	if written.IsZero() {
		return
	}
	entry := ledger.Entry{Task: taskID, Station: "build", Role: "builder", Seconds: closed.Sub(written).Seconds()}
	fillMachine(root, taskID, &entry)
	fillUsage(root, taskID, closed, &entry)
	Stamp(root, entry)
}

// fillMachine names the tier, provider, and model the profile resolves for a task's builder.
func fillMachine(root, taskID string, entry *ledger.Entry) {
	tier := "standard"
	if role, err := LoadRole(root, "builder"); err == nil && role.Tier != "" {
		tier = role.Tier
	}
	if path, err := backlog.Find(root); err == nil {
		if parsed, err := backlog.Load(path); err == nil {
			if task, ok := parsed.Task(taskID); ok && task.Tier() != "" {
				tier = task.Tier()
			}
		}
	}
	machine := resolveProfile(root).Tiers.Machine(tier)
	entry.Tier, entry.Provider, entry.Model = tier, machine.Provider, machine.Model
}

// resolveProfile is the selected profile narrowed by the developer's overlay, which every station shares.
func resolveProfile(root string) profilepkg.Profile {
	chosen := profilepkg.Select(root)
	if path := mount.OverlayPath(); path != "" {
		chosen = profilepkg.Overlay(chosen, path)
	}
	return chosen
}

// briefTime is when the task's brief was written, which opens the window the usage covers.
func briefTime(root, taskID string) time.Time {
	info, err := os.Stat(filepath.Join(root, StateDir, "briefs", taskID+".md"))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
