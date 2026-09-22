package backlog

import (
	"fmt"
	"strings"
)

// Lint returns every problem that would stop the line running this backlog deterministically.
func Lint(parsed Backlog) []string {
	problems := append([]string(nil), parsed.Problems...)
	seen := map[string]int{}
	ids := map[string]bool{}
	for _, task := range parsed.Tasks() {
		ids[task.ID] = true
	}
	for _, group := range parsed.Groups {
		if !contains(Modes, group.Mode()) {
			problems = append(problems, fmt.Sprintf("%s: mode must be one of %s", group.ID, strings.Join(Modes, "|")))
		}
		if !contains(Types, group.Type()) {
			problems = append(problems, fmt.Sprintf("%s: type must be one of %s", group.ID, strings.Join(Types, "|")))
		}
		switch version := group.Version(); {
		case version == "":
			problems = append(problems, fmt.Sprintf("%s: no version; a group declares the version it ships as `version: x.y.z`", group.ID))
		case !versionRe.MatchString(version):
			problems = append(problems, fmt.Sprintf("%s: version %q is not x.y.z", group.ID, version))
		}
		if line, dup := seen[group.ID]; dup {
			problems = append(problems, fmt.Sprintf("%s: duplicate group id (lines %d and %d)", group.ID, line+1, group.Heading+1))
		}
		seen[group.ID] = group.Heading
	}
	for _, task := range parsed.Tasks() {
		where := fmt.Sprintf("%s (line %d)", task.ID, task.Heading+1)
		if _, dup := seen[task.ID]; dup {
			problems = append(problems, where+": duplicate task id")
		}
		seen[task.ID] = task.Heading
		if !contains(Statuses, task.Status) {
			problems = append(problems, fmt.Sprintf("%s: status must be one of %s", where, strings.Join(Statuses, "|")))
		}
		if !contains(Priorities, task.Priority) {
			problems = append(problems, fmt.Sprintf("%s: priority must be one of %s", where, strings.Join(Priorities, "|")))
		}
		if !contains(Owners, task.Owner()) {
			problems = append(problems, where+": owner must be agent or human")
		}
		if !contains(Types, task.Type()) {
			problems = append(problems, fmt.Sprintf("%s: type must be one of %s", where, strings.Join(Types, "|")))
		}
		if tier := task.Tier(); tier != "" && !contains(Tiers, tier) {
			problems = append(problems, fmt.Sprintf("%s: tier must be one of %s", where, strings.Join(Tiers, "|")))
		}
		for _, dep := range task.DependsOn() {
			if !ids[dep] {
				problems = append(problems, fmt.Sprintf("%s: depends_on names unknown task %s", where, dep))
			}
		}
		if task.Owner() != "agent" || !task.Ready() {
			continue
		}
		if task.BlockStart < 0 {
			problems = append(problems, where+": agent task has no yaml block")
			continue
		}
		if len(task.Files()) == 0 {
			problems = append(problems, where+": agent task declares no files")
		}
		if len(task.DoneWhen()) == 0 {
			problems = append(problems, where+": agent task declares no done_when commands")
		}
		for _, command := range task.DoneWhen() {
			if !commandHint.MatchString(strings.TrimSpace(command)) {
				problems = append(problems, fmt.Sprintf("%s: done_when entry does not look like a command: %q", where, command))
			}
		}
	}
	return problems
}
