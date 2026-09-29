package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/ledger"
	"komodo/internal/line"
)

// runLint prints every grammar problem, then each advisory note, and exits non-zero only on a problem.
func runLint(root string) {
	runLintGroupFiles(root)
}

// lintProblems returns every grammar problem across the repo's docs/backlog group files, none with no
// group files at all.
func lintProblems(root string) ([]string, error) {
	problems, _, _, err := groupFileLintProblems(root)
	return append(problems, versionProblems(root)...), err
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

// runLintGroupFiles reports every docs/backlog group file's own problems, plus a group over 12 tasks (REQ-8).
func runLintGroupFiles(root string) {
	problems, taskCount, groupCount, err := groupFileLintProblems(root)
	if err != nil {
		fail(err)
	}
	problems = append(problems, versionProblems(root)...)
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

// groupFileLintProblems collects every problem across every docs/backlog group file, with the task and group counts.
// A file whose heading fails to parse still counts as a group, so a malformed one is never silently dropped.
func groupFileLintProblems(root string) (problems []string, taskCount, groupCount int, err error) {
	names, err := groupFileNames(root)
	if err != nil {
		return nil, 0, 0, err
	}
	var files []backlog.GroupFile
	texts := map[string]string{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, groupFilesDir, name))
		if err != nil {
			return nil, 0, 0, err
		}
		group := backlog.ParseGroupFile(string(data))
		files = append(files, group)
		texts[group.ID] = string(data)
	}
	groupIDs := map[string]bool{}
	taskIDs := map[string]bool{}
	for _, file := range files {
		groupIDs[file.ID] = true
		for _, task := range file.Tasks {
			taskIDs[task.ID] = true
		}
	}
	for _, file := range files {
		problems = append(problems, file.Problems...)
		// Filed findings wait in REFINEMENT and no builder works them, so only the rest count toward the cap.
		if built := backlog.BuildableGroupFile(file); built > 12 {
			problems = append(problems, fmt.Sprintf("%s: %d tasks exceeds limit of 12 (suggest a split per REQ-8)", file.ID, built))
		}
		problems = append(problems, backlog.LintGroupFile(root, file, texts[file.ID], groupIDs, taskIDs)...)
		taskCount += len(file.Tasks)
	}
	problems = append(problems, backlog.LintGroupFileEpics(files)...)
	return problems, taskCount, len(files), nil
}

// runList prints the tasks of one group, or of every group.
func runList(root string, args []string) {
	set := flag.NewFlagSet("list", flag.ExitOnError)
	asJSON := set.Bool("json", false, "print JSON")
	status := set.String("status", "", "only tasks with this status")
	needle, rest := splitPositional(args, "status")
	_ = set.Parse(rest)
	// A run keeps live status in .komodo, not its group file, until it ships; list reads what step reads.
	parsed, _, err := line.LoadBacklog(root)
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

// groupFilesDir is where one file per group lives, named <group-id>-<slug>.md.
const groupFilesDir = "docs/backlog"

// runBacklog lists every open group under docs/backlog: there is no index, so the files are the list.
func runBacklog(root string) {
	names, err := groupFileNames(root)
	if err != nil {
		fail(err)
	}
	count := 0
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, groupFilesDir, name))
		if err != nil {
			fail(err)
		}
		group := backlog.ParseGroupFile(string(data))
		if group.ID == "" {
			continue
		}
		fmt.Printf("%-10s %-14s %s %s\n", group.ID, "["+group.Status+"]", "[P: "+group.Priority+"]", group.Title)
		count++
	}
	fmt.Printf("%d group(s)\n", count)
}

// groupFileNotes collects every group file's lint notes, which never fail lint.
func groupFileNotes(root string) []string {
	names, _ := groupFileNames(root)
	var notes []string
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, groupFilesDir, name))
		if err != nil {
			continue
		}
		notes = append(notes, backlog.NotesGroupFile(backlog.ParseGroupFile(string(data)))...)
	}
	return notes
}

// groupFileNames lists the group files under docs/backlog, sorted, or none when the directory is absent.
func groupFileNames(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, groupFilesDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// findGroupFile returns the path and text of the group file whose heading names groupID, if one exists.
func findGroupFile(root, groupID string) (path, text string, found bool, err error) {
	names, err := groupFileNames(root)
	if err != nil {
		return "", "", false, err
	}
	for _, name := range names {
		candidate := filepath.Join(root, groupFilesDir, name)
		data, err := os.ReadFile(candidate)
		if err != nil {
			return "", "", false, err
		}
		if backlog.ParseGroupFile(string(data)).ID == groupID {
			return candidate, string(data), true, nil
		}
	}
	return "", "", false, nil
}

// runBacklogAdd is add's group-file path: it writes a new group file when groupID has none yet,
// else appends a task to its file.
func runBacklogAdd(root string, args []string) {
	set := flag.NewFlagSet("add", flag.ExitOnError)
	files := set.String("files", "", "comma-separated paths the task touches")
	accept := set.String("accept", "", "comma-separated acceptance lines")
	priority := set.String("priority", "M", "C, H, M, or L")
	status := set.String("status", "REFINEMENT", "the status to open the group in")
	groupType := set.String("type", "feat", "the conventional-commit type")
	version := set.String("version", "", "the version the group ships")
	epic := set.String("epic", "", "the epic id the group belongs to")
	positional, rest := splitFlags(args, "files", "accept", "priority", "status", "type", "version", "epic")
	_ = set.Parse(rest)
	if len(positional) < 2 {
		fail(fmt.Errorf("usage: komodo add <group> <title> [--files a,b]"))
	}
	groupID, title := positional[0], strings.Join(positional[1:], " ")
	path, text, found, err := findGroupFile(root, groupID)
	if err != nil {
		fail(err)
	}
	if found {
		out, id, err := backlog.AppendGroupFileTask(text, title, splitStrings(*files), splitStrings(*accept))
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			fail(err)
		}
		fmt.Println(id)
		line.Stamp(root, ledger.Entry{Station: "add", Task: id, Outcome: "added"})
		return
	}
	var fields backlog.Fields
	fields.Set("type", *groupType)
	fields.Set("version", *version)
	fields.Set("epic", *epic)
	fields.Set("depends_on", []any{})
	out := backlog.RenderGroupFile(groupID, title, *priority, *status, fields)
	dest := filepath.Join(root, groupFilesDir, groupID+"-"+backlog.Slug(title)+".md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(dest, []byte(out), 0o644); err != nil {
		fail(err)
	}
	fmt.Println(groupID)
	line.Stamp(root, ledger.Entry{Station: "add", Task: groupID, Outcome: "added"})
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
