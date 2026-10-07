package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/harness"
	"komodo/internal/install"
	"komodo/internal/ledger"
)

// runLint prints every grammar problem, then each advisory note, and exits non-zero only on a problem.
func runLint(root string) {
	runLintGroupFiles(root)
}

// lintProblems returns every grammar problem across the repo's docs/backlog tree, none with no tree at all.
func lintProblems(root string) ([]string, error) {
	problems, _, _, err := groupFileLintProblems(root)
	problems = append(problems, versionProblems(root)...)
	problems = append(problems, install.ScriptProblems(root)...)
	problems = append(problems, install.SkillProblems(root)...)
	problems = append(problems, install.RuleProblems(root)...)
	return append(problems, backlog.LintDecisions(root)...), err
}

// versionProblems names each open group whose version is at or behind the repo's newest tag.
func versionProblems(root string) []string {
	parsed, err := backlog.LoadRoot(root)
	if err != nil {
		return nil
	}
	out, err := git.Run(root, "tag", "--list")
	if err != nil {
		return nil
	}
	return backlog.LintVersions(parsed, strings.Fields(out))
}

// runLintGroupFiles reports every docs/backlog problem, then the notes, with the task and group counts.
func runLintGroupFiles(root string) {
	problems, taskCount, groupCount, err := groupFileLintProblems(root)
	if err != nil {
		fail(err)
	}
	problems = append(problems, versionProblems(root)...)
	problems = append(problems, backlog.LintDecisions(root)...)
	for _, problem := range problems {
		fmt.Println(problem)
	}
	for _, note := range groupFileNotes(root) {
		fmt.Println("note " + note)
	}
	fmt.Printf("%d task(s), %d group(s), %d problem(s)\n", taskCount, groupCount, len(problems))
	if len(problems) > 0 {
		exit(1)
	}
}

// groupFileLintProblems collects every problem across the docs/backlog tree, with the task and group counts:
// the tree's own structure, each assembled group, the epics' agreement, and the whole backlog's rules.
func groupFileLintProblems(root string) (problems []string, taskCount, groupCount int, err error) {
	tree, err := backlog.LoadTree(root)
	if err != nil {
		return nil, 0, 0, err
	}
	files := tree.Files()
	groupIDs := map[string]bool{}
	taskIDs := map[string]bool{}
	groupVersions := map[string]string{}
	for _, file := range files {
		groupIDs[file.ID] = true
		groupVersions[file.ID] = file.Version
		for _, task := range file.Tasks {
			taskIDs[task.ID] = true
		}
	}
	problems = append(problems, backlog.LintTree(tree)...)
	for index, file := range files {
		problems = append(problems, file.Problems...)
		// Filed findings wait in REFINEMENT and no builder works them, so only the rest count toward the cap.
		if built := backlog.BuildableGroupFile(file); built > backlog.MaxGroupTasks {
			problems = append(problems, fmt.Sprintf("%s: %d tasks exceeds limit of %d (suggest a split per REQ-8)", file.ID, built, backlog.MaxGroupTasks))
		}
		text := strings.Join(groupTexts(tree.Groups[index]), "\n")
		problems = append(problems, backlog.LintGroupFile(root, file, text, groupIDs, taskIDs, groupVersions)...)
		taskCount += len(file.Tasks)
	}
	problems = append(problems, backlog.LintGroupFileEpics(files)...)
	problems = append(problems, backlog.LintGroupFileDuplicates(files)...)
	parsed := backlog.FromTree(tree)
	parsed.Problems = nil
	problems = append(problems, backlog.Lint(parsed)...)
	return unique(problems), taskCount, len(files), nil
}

// groupTexts reads every file of a group folder, the group index file first, for the lints that scan raw lines.
func groupTexts(group backlog.GroupDir) []string {
	var texts []string
	for _, path := range group.Paths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		texts = append(texts, string(data))
	}
	return texts
}

// unique keeps the first of each identical problem line, since two lints may name one fault.
func unique(problems []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, problem := range problems {
		if !seen[problem] {
			seen[problem] = true
			out = append(out, problem)
		}
	}
	return out
}

// runList prints the tasks of one group, or of every group.
func runList(root string, args []string) {
	set := flag.NewFlagSet("list", flag.ExitOnError)
	asJSON := set.Bool("json", false, "print JSON")
	status := set.String("status", "", "only tasks with this status")
	needle, rest := splitPositional(args, "status")
	_ = set.Parse(rest)
	// A run keeps live status in .komodo, not its group file, until it ships; list reads what step reads.
	parsed, _, err := harness.LoadBacklog(root)
	if err != nil {
		fail(err)
	}
	groups := parsed.Groups
	if needle != "" {
		group, ok := parsed.Group(needle)
		if !ok {
			fail(fmt.Errorf("no group matching %q", needle))
		}
		groups = []backlog.Group{group}
	}
	type row struct {
		ID       string `json:"id"`
		Group    string `json:"group"`
		Title    string `json:"title"`
		Priority string `json:"priority"`
		Status   string `json:"status"`
		Owner    string `json:"owner"`
	}
	var rows []row
	for _, group := range groups {
		for _, task := range group.Tasks {
			if *status != "" && !strings.EqualFold(task.Status, *status) {
				continue
			}
			rows = append(rows, row{task.ID, group.ID, task.Title, task.Priority, task.Status, task.Owner()})
		}
	}
	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(rows); err != nil {
			fail(err)
		}
		return
	}
	for _, item := range rows {
		fmt.Printf("%-16s %-12s %s %s\n", item.ID, "["+item.Status+"]", "[P: "+item.Priority+"]", item.Title)
	}
	fmt.Printf("%d task(s)\n", len(rows))
}

// groupFilesDir holds every epic folder, group folder and task file.
const groupFilesDir = backlog.GroupFilesDir

// groupFileStatuses are the statuses valid on a group's or an epic's own heading, matching the grammar.
var groupFileStatuses = []string{"REFINEMENT", "READY", "BLOCKED"}

// runBacklog lists every epic and its open groups under docs/backlog: the tree is the list.
func runBacklog(root string) {
	tree, err := backlog.LoadTree(root)
	if err != nil {
		fail(err)
	}
	count := 0
	for _, epic := range tree.Epics {
		fmt.Printf("%-10s %-14s %s  %s\n", epic.ID, "["+epic.Status+"]", epic.Version, epic.Title)
		for _, group := range tree.Groups {
			if group.Epic.ID != epic.ID || group.File.ID == "" {
				continue
			}
			fmt.Printf("  %-10s %-14s %s %s\n", group.File.ID, "["+group.File.Status+"]", "[P: "+group.File.Priority+"]", group.File.Title)
			count++
		}
	}
	for _, flat := range tree.Flat {
		fmt.Printf("flat %s: a group file outside the tree; run komodo migrate\n", relPath(root, flat))
	}
	fmt.Printf("%d group(s)\n", count)
}

// groupFileNotes collects every group's lint notes, which never fail lint, plus one per flat file.
func groupFileNotes(root string) []string {
	tree, err := backlog.LoadTree(root)
	if err != nil {
		return nil
	}
	var notes []string
	for _, file := range tree.Files() {
		notes = append(notes, backlog.NotesGroupFile(file)...)
	}
	for _, flat := range tree.Flat {
		notes = append(notes, relPath(root, flat)+": a group file outside the tree; run komodo migrate, then remove it")
	}
	return notes
}

// epicIDPattern matches an epic id's own shape.
var epicIDPattern = regexp.MustCompile(`^EPIC-[\w.]+$`)

// runBacklogAdd adds an epic folder, a group folder under its epic, or a task file in its group.
func runBacklogAdd(root string, args []string) {
	set := flag.NewFlagSet("add", flag.ExitOnError)
	files := set.String("files", "", "comma-separated paths the task touches")
	var accept repeatedFlag
	set.Var(&accept, "accept", "one acceptance line; repeat --accept for more than one")
	doneWhen := set.String("done-when", "", "comma-separated shell commands whose zero exit proves the task done")
	priority := set.String("priority", "M", "C, H, M, or L")
	status := set.String("status", "REFINEMENT", "the status to open the group or epic in")
	groupType := set.String("type", "feat", "the conventional-commit type")
	version := set.String("version", "", "an epic's version, the one every group under it ships")
	groupsMax := set.Int("groups-max", backlog.DefaultGroupsMax, "an epic's most groups")
	goal := set.String("goal", "", "an epic's goal paragraph")
	mode := set.String("mode", "", "a group's mode: parallel or single")
	next := set.String("next", "", "an epic id; print the next free group id across local branches and exit")
	positional, rest := splitFlags(args, "files", "accept", "done-when", "priority", "status", "type", "version", "groups-max", "goal", "mode", "next")
	_ = set.Parse(rest)
	if *next != "" {
		taken, err := takenGroupIDs(root)
		if err != nil {
			fail(err)
		}
		id, err := backlog.NextGroupID(*next, taken)
		if err != nil {
			fail(err)
		}
		fmt.Println(id)
		return
	}
	if len(positional) < 2 {
		fail(fmt.Errorf("usage: komodo add <epic|group> <title> [--files a,b] [--done-when cmd]"))
	}
	id, title := positional[0], strings.Join(positional[1:], " ")
	if !contains(groupFileStatuses, *status) {
		fail(fmt.Errorf("%q is not a status: use REFINEMENT, READY, or BLOCKED", *status))
	}
	if epicIDPattern.MatchString(id) {
		addEpic(root, backlog.EpicFile{ID: id, Title: title, Status: *status, Version: *version, Type: *groupType, GroupsMax: *groupsMax, Goal: *goal})
		return
	}
	if !backlog.ValidGroupID(id) {
		fail(fmt.Errorf("%q is neither an epic id EPIC-<n> nor a group id TG-<n>.<m>", id))
	}
	if !contains(backlog.Priorities, *priority) {
		fail(fmt.Errorf("%q is not a priority: use C, H, M, or L", *priority))
	}
	task := backlog.GroupTask{Title: title, Files: splitStrings(*files), Accept: accept, Checks: splitStrings(*doneWhen)}
	taskFlagsGiven := len(task.Files) > 0 || len(accept) > 0 || len(task.Checks) > 0
	group, found, err := backlog.Locate(root, id)
	if err != nil {
		fail(err)
	}
	if found {
		if len(task.Files) == 0 {
			fail(fmt.Errorf("task declares no files; pass --files"))
		}
		taskID, _, err := group.AppendTask(task)
		if err != nil {
			fail(err)
		}
		fmt.Println(taskID)
		harness.Stamp(root, ledger.Entry{Station: "add", Task: taskID, Outcome: "added"})
		return
	}
	taken, err := takenGroupIDs(root)
	if err != nil {
		fail(err)
	}
	if taken[id] {
		fail(fmt.Errorf("%s is already taken in the working tree or on a local branch", id))
	}
	if taskFlagsGiven && len(task.Files) == 0 {
		fail(fmt.Errorf("task declares no files; pass --files"))
	}
	file := backlog.GroupFile{ID: id, Title: title, Priority: *priority, Status: *status, Type: *groupType, Mode: *mode}
	if taskFlagsGiven {
		task.ID = "TSK-" + strings.TrimPrefix(id, "TG-") + ".1"
		file.Tasks = append(file.Tasks, task)
	}
	dir, err := backlog.WriteGroup(root, file)
	if err != nil {
		fail(err)
	}
	fmt.Println(id)
	taskID := id
	if taskFlagsGiven {
		taskID = task.ID
	}
	harness.Stamp(root, ledger.Entry{Station: "add", Task: taskID, Outcome: "added"})
	_ = dir
}

// addEpic writes a new epic folder and prints its id.
func addEpic(root string, epic backlog.EpicFile) {
	if epic.Version == "" {
		fail(fmt.Errorf("an epic declares the version it ships; pass --version x.y.z"))
	}
	if _, err := backlog.WriteEpic(root, epic); err != nil {
		fail(err)
	}
	fmt.Println(epic.ID)
	harness.Stamp(root, ledger.Entry{Station: "add", Task: epic.ID, Outcome: "added"})
}

// repeatedFlag collects every occurrence of a flag given more than once, in order.
type repeatedFlag []string

// String joins the collected values for flag's usage output.
func (r *repeatedFlag) String() string { return strings.Join(*r, ", ") }

// Set appends one more occurrence's value.
func (r *repeatedFlag) Set(value string) error {
	*r = append(*r, value)
	return nil
}

// groupDirname is a group folder's name inside the tree, from which its id is read back.
var groupDirname = regexp.MustCompile(`(?:^|/)(tg-[\w.]+)(?:/|$)`)

// takenGroupIDs is every group id already in the working tree, including an untracked folder, plus
// every group id docs/backlog holds on any local branch, so a fresh checkout still sees it.
func takenGroupIDs(root string) (map[string]bool, error) {
	taken := map[string]bool{}
	tree, err := backlog.LoadTree(root)
	if err != nil {
		return nil, err
	}
	for _, group := range tree.Groups {
		if group.File.ID != "" {
			taken[group.File.ID] = true
		}
		taken[strings.ToUpper(filepath.Base(group.Dir))] = true
	}
	branches, err := git.Run(root, "branch", "--list", "--format=%(refname:short)")
	if err != nil {
		return taken, nil // no repo, or no git at all; the working tree is every id there is
	}
	for _, branch := range strings.Split(branches, "\n") {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}
		listing, err := git.Run(root, "ls-tree", "-r", "--name-only", branch, "--", groupFilesDir)
		if err != nil {
			continue // a branch with no docs/backlog yet names none
		}
		for _, name := range strings.Split(listing, "\n") {
			if match := groupDirname.FindStringSubmatch(filepath.ToSlash(name)); match != nil {
				taken[strings.ToUpper(match[1])] = true
			}
		}
	}
	return taken, nil
}

// splitStrings turns a comma-separated flag into a plain string slice, dropping empty parts.
func splitStrings(value string) []string {
	var items []string
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

// atoiOr parses a count, falling back when the text is not one.
func atoiOr(text string, fallback int) int {
	if n, err := strconv.Atoi(text); err == nil {
		return n
	}
	return fallback
}
