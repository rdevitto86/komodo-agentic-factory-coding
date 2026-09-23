// Package backlog parses, lints, and rewrites the BACKLOG.md grammar.
package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Statuses are the status tokens a task heading may carry.
var Statuses = []string{"REFINEMENT", "READY", "IN_PROGRESS", "BLOCKED", "DONE"}

// Priorities are the priority letters a task heading may carry.
var Priorities = []string{"C", "H", "M", "L"}

// Types are the conventional-commit types a task or group may declare.
var Types = []string{"feat", "fix", "chore", "docs", "test", "refactor", "perf", "build", "ci"}

// Owners are who may execute a task.
var Owners = []string{"agent", "human"}

// Tiers are the machine sizes a task's tier key may override the role's tier with.
var Tiers = []string{"light", "standard", "heavy"}

// Modes are how a group's tasks are run.
var Modes = []string{"parallel", "single"}

var (
	epicHeading  = regexp.MustCompile(`^##\s+\[(EPIC-[\w.]+)\]\s*(.*?)\s*$`)
	groupHeading = regexp.MustCompile(`^###\s+\[(TG-[\w.]+)\]\s*(.*?)\s*$`)
	taskHeading  = regexp.MustCompile(`^####\s+\[(TSK-[\w.]+)\]\s+(.+?)\s*\[P:\s*([A-Z])\]\s*\[([A-Z_]+)\]\s*$`)
	taskLike     = regexp.MustCompile(`^####\s+\[TSK-`)
	fenceOpen    = regexp.MustCompile("^```(?:yaml|yml)\\s*$")
	fenceClose   = regexp.MustCompile("^```\\s*$")
	versionRe    = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	slugRe       = regexp.MustCompile(`[^a-z0-9]+`)
	commandHint  = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*` +
		`(!|go|npm|pnpm|bun|npx|python3?|py\b|pytest|make|task|just|cdk|tsc|cargo|dotnet|mvn|gradle|zig|swift` +
		`|bash|sh|grep|rg|sed|awk|find|test\b|git\b|komodo\b|\./|/|"|'|[A-Za-z]:[\\/])`)
)

var legacyStatus = map[string]string{"WIP": "IN_PROGRESS", "TODO": "READY"}

// Task is one task heading plus its fenced block, with the line positions a rewrite needs.
type Task struct {
	ID         string
	Title      string
	Priority   string
	Status     string
	Fields     Fields
	Heading    int
	BlockStart int
	BlockEnd   int
	GroupID    string
}

// Files are the paths a task declares it will touch.
func (t Task) Files() []string { return t.Fields.List("files") }

// Dirs are the scopes a task owns: a declared directory itself, a declared file's directory.
func (t Task) Dirs() []string {
	var seen []string
	for _, path := range t.Files() {
		clean := strings.ReplaceAll(path, "\\", "/")
		dir := clean
		if filepath.Ext(clean) != "" {
			dir = filepath.Dir(clean)
		}
		if dir == "" {
			dir = "."
		}
		if !contains(seen, dir) {
			seen = append(seen, dir)
		}
	}
	return seen
}

// DoneWhen are the shell commands whose zero exit proves the task done.
func (t Task) DoneWhen() []string { return t.Fields.List("done_when") }

// DependsOn are the task ids that must be DONE before this one may start.
func (t Task) DependsOn() []string { return t.Fields.List("depends_on") }

// Context are the paths, with an optional anchor, a machine reads before starting.
func (t Task) Context() []string { return t.Fields.List("context") }

// Tier is the machine size that overrides the role's tier for one task, empty when unset.
func (t Task) Tier() string { return t.Fields.String("tier") }

// Timeout is the wall clock one done_when command may take, as a Go duration, or empty for the default.
func (t Task) Timeout() string { return t.Fields.String("timeout") }

// Facets are the facet names one task adds to detection, beyond what the tree and the repo override.
func (t Task) Facets() []string { return t.Fields.List("facets") }

// Owner is who executes a task, agent unless a person must act.
func (t Task) Owner() string {
	if owner := t.Fields.String("owner"); owner != "" {
		return owner
	}
	return "agent"
}

// Type is the conventional-commit type, feat unless the block names another.
func (t Task) Type() string {
	if value := t.Fields.String("type"); value != "" {
		return value
	}
	return "feat"
}

// Open reports whether the task still has work to run, planned or not.
func (t Task) Open() bool { return t.Status != "DONE" && t.Status != "BLOCKED" }

// Ready reports whether the task is planned enough for the line to run it.
func (t Task) Ready() bool { return t.Status == "READY" || t.Status == "IN_PROGRESS" }

// Group is a task group heading, its optional block, and its tasks in file order.
type Group struct {
	ID      string
	Title   string
	Fields  Fields
	Tasks   []Task
	Heading int
	EpicID  string
}

// Mode is parallel unless the group asks for one machine over the whole group.
func (g Group) Mode() string {
	if value := g.Fields.String("mode"); value != "" {
		return value
	}
	return "parallel"
}

// Type is the branch and commit type for the group, feat by default.
func (g Group) Type() string {
	if value := g.Fields.String("type"); value != "" {
		return value
	}
	return "feat"
}

// Base is the branch this group's work is cut from, empty when the remote's default branch is right.
func (g Group) Base() string { return g.Fields.String("base") }

// Version is the version this group ships, written into the changelog heading and the tag.
func (g Group) Version() string { return g.Fields.String("version") }

// Slug is a kebab-case branch fragment derived from the group title.
func (g Group) Slug() string {
	text := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(g.Title), "-"), "-")
	if len(text) > 40 {
		text = text[:40]
	}
	text = strings.TrimRight(text, "-")
	if text == "" {
		return strings.ToLower(g.ID)
	}
	return text
}

// OpenTasks are the tasks that are neither DONE nor BLOCKED.
func (g Group) OpenTasks() []Task {
	var out []Task
	for _, task := range g.Tasks {
		if task.Open() {
			out = append(out, task)
		}
	}
	return out
}

// ReadyTasks are the open tasks past refinement, in file order.
func (g Group) ReadyTasks() []Task {
	var out []Task
	for _, task := range g.Tasks {
		if task.Ready() {
			out = append(out, task)
		}
	}
	return out
}

// Backlog is the whole parsed file: groups in order, plus every problem the parser saw.
type Backlog struct {
	Groups   []Group
	Problems []string
	Lines    []string
}

// Tasks are every task across every group, in file order.
func (b Backlog) Tasks() []Task {
	var out []Task
	for _, group := range b.Groups {
		out = append(out, group.Tasks...)
	}
	return out
}

// Task returns the task with this exact id.
func (b Backlog) Task(id string) (Task, bool) {
	for _, task := range b.Tasks() {
		if task.ID == id {
			return task, true
		}
	}
	return Task{}, false
}

// Group returns the group whose id matches, else the first whose id or title contains the needle.
func (b Backlog) Group(needle string) (Group, bool) {
	for _, group := range b.Groups {
		if group.ID == needle {
			return group, true
		}
	}
	lowered := strings.ToLower(needle)
	for _, group := range b.Groups {
		if strings.Contains(strings.ToLower(group.ID), lowered) || strings.Contains(strings.ToLower(group.Title), lowered) {
			return group, true
		}
	}
	return Group{}, false
}

// NextGroup is the first group in file order holding at least one ready agent task.
func (b Backlog) NextGroup() (Group, bool) {
	for _, group := range b.Groups {
		for _, task := range group.Tasks {
			if task.Ready() && task.Owner() == "agent" {
				return group, true
			}
		}
	}
	return Group{}, false
}

// Find locates BACKLOG.md at the repo root or under docs/.
func Find(root string) (string, error) {
	for _, candidate := range []string{"BACKLOG.md", filepath.Join("docs", "BACKLOG.md")} {
		path := filepath.Join(root, candidate)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("no BACKLOG.md at %s or %s/docs", root, root)
}

// readBlock reads the fenced yaml block directly under a heading and returns its bounds.
func readBlock(lines []string, start int) (Fields, int, int, error) {
	index := start
	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}
	if index >= len(lines) || !fenceOpen.MatchString(lines[index]) {
		return Fields{}, -1, -1, nil
	}
	open := index
	index++
	var body []string
	for index < len(lines) && !fenceClose.MatchString(lines[index]) {
		body = append(body, lines[index])
		index++
	}
	if index >= len(lines) {
		return Fields{}, open, -1, fmt.Errorf("unterminated yaml block opened at line %d", open+1)
	}
	fields, err := ParseFields(strings.Join(body, "\n"))
	if err != nil {
		return Fields{}, open, index, fmt.Errorf("line %d: %v", open+1, err)
	}
	return fields, open, index, nil
}

// Parse reads BACKLOG.md text into groups and tasks without judging their content.
func Parse(text string) Backlog {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	parsed := Backlog{Lines: lines}
	epicID := ""
	current := -1
	index := 0
	for index < len(lines) {
		line := lines[index]
		if match := epicHeading.FindStringSubmatch(line); match != nil {
			epicID = match[1]
			index++
			continue
		}
		if match := groupHeading.FindStringSubmatch(line); match != nil {
			group := Group{ID: match[1], Title: match[2], Heading: index, EpicID: epicID}
			fields, _, end, err := readBlock(lines, index+1)
			if err != nil {
				parsed.Problems = append(parsed.Problems, err.Error())
			}
			if end >= 0 && err == nil && fields.values != nil {
				group.Fields = fields
				index = end + 1
			} else {
				index++
			}
			parsed.Groups = append(parsed.Groups, group)
			current = len(parsed.Groups) - 1
			continue
		}
		if match := taskHeading.FindStringSubmatch(line); match != nil {
			status := strings.ToUpper(match[4])
			if mapped, ok := legacyStatus[status]; ok {
				status = mapped
			}
			task := Task{
				ID: match[1], Title: strings.TrimSpace(match[2]), Priority: match[3],
				Status: status, Heading: index, BlockStart: -1, BlockEnd: -1,
			}
			fields, start, end, err := readBlock(lines, index+1)
			if err != nil {
				parsed.Problems = append(parsed.Problems, err.Error())
			}
			if end >= 0 && err == nil && fields.values != nil {
				task.Fields = fields
				task.BlockStart, task.BlockEnd = start, end
				index = end + 1
			} else {
				index++
			}
			if current < 0 {
				parsed.Problems = append(parsed.Problems,
					fmt.Sprintf("line %d: task %s appears before any ### [TG-] heading", task.Heading+1, task.ID))
				parsed.Groups = append(parsed.Groups, Group{ID: "TG-ORPHAN", Title: "Orphaned tasks", Heading: task.Heading})
				current = len(parsed.Groups) - 1
			}
			task.GroupID = parsed.Groups[current].ID
			parsed.Groups[current].Tasks = append(parsed.Groups[current].Tasks, task)
			continue
		}
		if taskLike.MatchString(line) {
			parsed.Problems = append(parsed.Problems,
				fmt.Sprintf("line %d: heading does not match the task pattern: %q", index+1, line))
		}
		index++
	}
	return parsed
}

// Load reads and parses one BACKLOG.md file.
func Load(path string) (Backlog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Backlog{}, err
	}
	return Parse(string(data)), nil
}

// contains reports whether the slice already holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
