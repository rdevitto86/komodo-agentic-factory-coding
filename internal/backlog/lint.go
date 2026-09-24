package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Slug is the anchor form of a heading: lower case, runs of punctuation and space folded to one dash.
func Slug(text string) string {
	return strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(text), "-"), "-")
}

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
		if timeout := task.Timeout(); timeout != "" {
			if parsed, err := time.ParseDuration(timeout); err != nil || parsed <= 0 {
				problems = append(problems, fmt.Sprintf("%s: timeout %q is not a positive duration such as 15m", where, timeout))
			}
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

// Advice that never fails lint: each group whose longest open dependency chain covers most of its open tasks.
func Notes(parsed Backlog) []string {
	var notes []string
	for _, group := range parsed.Groups {
		open := map[string]Task{}
		for _, task := range group.Tasks {
			if task.Status != "DONE" {
				open[task.ID] = task
			}
		}
		if len(open) < 3 {
			continue
		}
		depth := map[string]int{}
		var chain func(string) int
		chain = func(id string) int {
			if known, ok := depth[id]; ok {
				return known
			}
			depth[id] = 1
			longest := 0
			for _, dep := range open[id].DependsOn() {
				if _, ok := open[dep]; ok {
					longest = max(longest, chain(dep))
				}
			}
			depth[id] = longest + 1
			return depth[id]
		}
		longest := 0
		for id := range open {
			longest = max(longest, chain(id))
		}
		if longest*2 > len(open) {
			notes = append(notes, fmt.Sprintf("%s: %d of %d tasks are one chain; split the shared file or drop a dependency to build in parallel",
				group.ID, longest, len(open)))
		}
	}
	return notes
}

// LintContext reports every context anchor whose file exists under root but holds no matching
// heading, so a mistyped anchor fails at lint instead of sending a builder the whole file.
func LintContext(root string, parsed Backlog) []string {
	var problems []string
	for _, task := range parsed.Tasks() {
		if !task.Open() {
			continue
		}
		for _, ref := range task.Context() {
			path, anchor, found := strings.Cut(ref, "#")
			if !found || anchor == "" || strings.ContainsAny(ref, " \t") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				continue
			}
			if !hasHeading(string(data), anchor) {
				problems = append(problems, fmt.Sprintf("%s (line %d): context %s names no heading in %s", task.ID, task.Heading+1, ref, path))
			}
		}
	}
	return problems
}

// hasHeading reports whether a markdown text holds a heading whose slug matches the anchor.
func hasHeading(text, anchor string) bool {
	want := Slug(anchor)
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "#") && Slug(strings.TrimSpace(strings.TrimLeft(line, "#"))) == want {
			return true
		}
	}
	return false
}
