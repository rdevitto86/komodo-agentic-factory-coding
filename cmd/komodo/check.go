package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"komodo/internal/backlog"
	"komodo/internal/check"
	"komodo/internal/line"
)

// checkUsage names the three checks and the argument each takes.
const checkUsage = "usage: komodo check task <task> | scope <task|group> | findings <result.json> [--base ref]"

// runCheck is the one entry point hooks and agents call: a task's checks, a scope, or a review's findings.
func runCheck(root string, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, checkUsage)
		exit(2)
	}
	target, rest := splitPositional(args[1:], "base")
	set := flag.NewFlagSet("check", flag.ExitOnError)
	base := set.String("base", "", "the ref the worktree forked from; defaults to the group's base")
	_ = set.Parse(rest)
	if target == "" {
		fmt.Fprintln(os.Stderr, checkUsage)
		exit(2)
	}
	var problems []string
	switch args[0] {
	case "task":
		problems = checkTask(root, target, *base)
	case "scope":
		problems = checkScope(root, target, *base)
	case "findings":
		problems = checkFindings(root, target, *base)
	default:
		fmt.Fprintf(os.Stderr, "komodo check: unknown check %q\n%s\n", args[0], checkUsage)
		exit(2)
	}
	for _, problem := range problems {
		fmt.Println(problem)
	}
	fmt.Printf("%d problem(s)\n", len(problems))
	if len(problems) > 0 {
		exit(1)
	}
}

// checkTask reruns one task's done_when commands, then its scope, in the worktree.
func checkTask(root, taskID, base string) []string {
	_, parsed := load(root)
	task, ok := parsed.Task(taskID)
	if !ok {
		fail(fmt.Errorf("no task %s", taskID))
	}
	group := check.Group{Worktree: root, Base: checkBase(root, parsed, task.GroupID, base), Files: task.Files()}
	return check.Run(group, "", "", task.DoneWhen())
}

// checkScope names every edit outside a task's files, or outside every file its group's tasks declare.
func checkScope(root, target, base string) []string {
	_, parsed := load(root)
	if task, ok := parsed.Task(target); ok {
		return check.Scope(root, checkBase(root, parsed, task.GroupID, base), task.Files())
	}
	group, ok := parsed.Group(target)
	if !ok {
		fail(fmt.Errorf("no task or group %s", target))
	}
	var files []string
	for _, task := range group.Tasks {
		files = append(files, task.Files()...)
	}
	return check.Scope(root, checkBase(root, parsed, group.ID, base), files)
}

// checkFindings names every finding in a review result whose file and line are not a line the diff adds.
func checkFindings(root, path, base string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	var result struct {
		Findings []line.Finding `json:"findings"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fail(fmt.Errorf("%s is not a review result: %w", path, err))
	}
	if base == "" {
		base = line.DefaultBase(root)
	}
	diff, err := check.Diff(root, base)
	if err != nil {
		fail(err)
	}
	added := map[string]map[int]bool{}
	for _, change := range check.ParseAddedLines(diff) {
		if added[change.File] == nil {
			added[change.File] = map[int]bool{}
		}
		added[change.File][change.Line] = true
	}
	var problems []string
	for _, finding := range result.Findings {
		if !added[finding.File][finding.Line] {
			problems = append(problems, fmt.Sprintf("findings: %s:%d %s is not on a changed line",
				finding.File, finding.Line, finding.Title))
		}
	}
	return problems
}

// checkBase is the explicit base, else the base the line cuts the group from, its epic's branch included.
func checkBase(root string, parsed backlog.Backlog, groupID, base string) string {
	if base != "" {
		return base
	}
	if group, ok := parsed.Group(groupID); ok {
		return line.GroupBase(root, parsed, group)
	}
	return line.DefaultBase(root)
}
