package line

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"time"

	"komodo/internal/backlog"
	"komodo/internal/comments"
	"komodo/internal/ledger"
	"komodo/internal/mount"
)

// MaxRepairs is how many times one task may come back for a repair before it blocks.
const MaxRepairs = 1

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

// CloseTask validates a task's result, reruns its checks, and flips its status.
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
	cwd := taskWorktree(root, taskID)
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
		if err := commitTask(cwd, task); err != nil {
			outcome.Problems = []string{"commit: " + err.Error()}
			return outcome, nil
		}
		outcome.Status = "DONE"
		clearAttempt(root, taskID)
		entry.Outcome = "done"
		Stamp(root, entry)
		return outcome, writeStatus(path, taskID, "DONE")
	}
	entry.FailureClass = FailureClass(problems)
	attempt, err := bumpAttempt(root, taskID, strings.Join(problems, "\n"), diffOf(cwd))
	if err != nil {
		return nil, err
	}
	outcome.Attempt = attempt.Count
	outcome.Failure = attempt.Failure
	if attempt.Count > MaxRepairs {
		outcome.Status = "BLOCKED"
		entry.Outcome = "blocked"
		Stamp(root, entry)
		return outcome, writeStatus(path, taskID, "BLOCKED")
	}
	outcome.Status = "IN_PROGRESS"
	entry.Outcome = "repair"
	Stamp(root, entry)
	return outcome, writeStatus(path, taskID, "IN_PROGRESS")
}

// taskWorktree is where a task was built, falling back to the repo root.
func taskWorktree(root, taskID string) string {
	path := filepath.Join(root, StateDir, "wt", taskID)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	if state, err := LoadRun(root); err == nil && state.Worktree != "" {
		if info, err := os.Stat(state.Worktree); err == nil && info.IsDir() {
			return state.Worktree
		}
	}
	return root
}

// checkResult validates the result JSON against the role's schema.
func checkResult(root, taskID string) []string {
	result, err := ReadResult(root, taskID)
	if err != nil {
		return []string{"result: " + err.Error()}
	}
	role := "builder"
	if named, ok := result["role"].(string); ok && named != "" {
		role = named
	}
	schema, err := LoadSchema(root, role)
	if err != nil {
		return []string{"schema: " + err.Error()}
	}
	return Validate(schema, any(result))
}

// runDoneWhen reruns every done_when command in the worktree and names each failure.
func runDoneWhen(cwd string, task backlog.Task) []string {
	var problems []string
	for _, command := range task.DoneWhen() {
		cmd := exec.Command("sh", "-c", command)
		cmd.Dir = cwd
		output, err := cmd.CombinedOutput()
		if err != nil {
			problems = append(problems, fmt.Sprintf("done_when `%s` failed: %v\n%s",
				command, err, Clip(strings.TrimSpace(string(output)), 4000, "output")))
		}
	}
	return problems
}

// lintComments runs the comment lint over the files a task named.
func lintComments(cwd string, task backlog.Task) []string {
	out, err := comments.Check(cwd, task.Files(), "nonobvious")
	if err != nil {
		return []string{"comments: " + err.Error()}
	}
	return out
}

// commitTask commits a passing task on its own branch, so QC has something to merge.
func commitTask(cwd string, task backlog.Task) error {
	if _, err := git(cwd, "rev-parse", "--git-dir"); err != nil {
		return nil
	}
	status, err := git(cwd, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	if _, err := git(cwd, "add", "-A"); err != nil {
		return err
	}
	if err := unstageBuilt(cwd, task); err != nil {
		return err
	}
	if staged, err := git(cwd, "diff", "--cached", "--name-only"); err != nil || strings.TrimSpace(staged) == "" {
		return err
	}
	message := fmt.Sprintf("%s: %s (%s)", task.Type(), task.Title, task.ID)
	_, err = git(cwd, "commit", "-m", message)
	return err
}

// Built are the paths a task branch never carries, because every branch regenerates them and they
// then conflict on every merge.
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
		if _, err := git(cwd, "reset", "--quiet", "HEAD", "--", path); err != nil {
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

// gateCommand runs the local gate in the worktree.
func gateCommand(cwd string) error {
	cmd := exec.Command("go", "run", "./cmd/komodo", "gate")
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%s", err, Clip(strings.TrimSpace(string(output)), 4000, "gate"))
	}
	return nil
}

// diffOf is the worktree's own diff, which a repair brief carries back to the machine.
func diffOf(cwd string) string {
	out, err := git(cwd, "diff", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

// writeStatus rewrites one task's status token in BACKLOG.md.
func writeStatus(path, taskID, status string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := backlog.SetStatus(string(data), taskID, status)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
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
	for _, host := range mount.Hosts() {
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

// stampBuild records what the machine spent between its brief and this close, which is the build itself.
func stampBuild(root, taskID string, closed time.Time) {
	written := briefTime(root, taskID)
	if written.IsZero() {
		return
	}
	entry := ledger.Entry{Task: taskID, Station: "build", Seconds: closed.Sub(written).Seconds()}
	fillUsage(root, taskID, closed, &entry)
	Stamp(root, entry)
}

// briefTime is when the task's brief was written, which opens the window the usage covers.
func briefTime(root, taskID string) time.Time {
	info, err := os.Stat(filepath.Join(root, StateDir, "briefs", taskID+".md"))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
