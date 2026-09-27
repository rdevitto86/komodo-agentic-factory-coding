package hooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/line"
	"komodo/internal/proc"
)

// checkOutputLimit bounds each failing check's output in a refusal, in bytes.
const checkOutputLimit = 4000

// nestedEnv marks a check the hook started, so a check that reaches the hook again allows instead of recursing.
const nestedEnv = "KOMODO_TASK_CHECKS"

// runTaskChecks runs every done_when of the group this worktree builds and refuses the stop while one fails.
func runTaskChecks(ctx context.Context, in Input) (Outcome, error) {
	if os.Getenv(nestedEnv) != "" {
		return Outcome{Verdict: Allow}, nil
	}
	tasks, err := worktreeTasks(in.Root)
	if err != nil {
		return Outcome{}, err
	}
	seen := map[string]bool{}
	var failures []string
	for _, task := range tasks {
		for _, command := range task.DoneWhen() {
			if seen[command] {
				continue
			}
			seen[command] = true
			timeout := min(line.TaskTimeout(task), remaining(ctx))
			ran := proc.ShellEnv(in.Root, command, timeout, append(os.Environ(), nestedEnv+"=1"))
			if !ran.OK() {
				failures = append(failures, fmt.Sprintf("`%s` failed: %v\n%s",
					command, ran.Err(), line.Clip(ran.Output, checkOutputLimit, "output")))
			}
		}
	}
	if len(failures) == 0 {
		return Outcome{Verdict: Allow}, nil
	}
	return Outcome{
		Verdict: Refuse,
		Message: "A check still fails. Fix it, run it again, then stop.\n\n" + strings.Join(failures, "\n\n"),
	}, nil
}

// worktreeTasks are the tasks of the group or task the worktree is named for, from its own backlog.
func worktreeTasks(root string) ([]backlog.Task, error) {
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	id := filepath.Base(root)
	for _, group := range parsed.Groups {
		if group.ID == id {
			return group.Tasks, nil
		}
	}
	if task, ok := parsed.Task(id); ok {
		return []backlog.Task{task}, nil
	}
	return nil, fmt.Errorf("no group or task %s in %s", id, path)
}
