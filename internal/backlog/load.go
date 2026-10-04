package backlog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GroupFilesDir is where one file per task group lives, the grammar komodo/rules/backlog.md names.
const GroupFilesDir = "docs/backlog"

// LoadRoot loads the queue at root: every docs/backlog/ group file, merged into one Backlog, empty
// when root holds none.
func LoadRoot(root string) (Backlog, error) {
	paths, err := groupFilePaths(root)
	if err != nil {
		return Backlog{}, err
	}
	return loadGroupFiles(paths)
}

// Exists reports whether root holds a queue: at least one docs/backlog/ group file.
func Exists(root string) bool {
	paths, err := groupFilePaths(root)
	return err == nil && len(paths) > 0
}

// groupFilePaths lists every *.md file directly under root's docs/backlog/, sorted, nil when the
// directory is absent.
func groupFilePaths(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, GroupFilesDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		paths = append(paths, filepath.Join(root, GroupFilesDir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

// loadGroupFiles reads and merges every group file into one Backlog, synthesizing one epic per
// group file's own epic and version so Lint's version-matches-epic check has something to compare.
func loadGroupFiles(paths []string) (Backlog, error) {
	var out Backlog
	seenEpics := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return Backlog{}, err
		}
		file := ParseGroupFile(string(data))
		for _, problem := range file.Problems {
			out.Problems = append(out.Problems, path+": "+problem)
		}
		if file.ID == "" {
			continue
		}
		out.Groups = append(out.Groups, groupFileToGroup(file))
		if file.EpicID != "" && file.Version != "" && !seenEpics[file.EpicID] {
			seenEpics[file.EpicID] = true
			out.Epics = append(out.Epics, Epic{ID: file.EpicID, Title: "Ships as `" + file.Version + "`"})
		}
	}
	return out, nil
}

// groupFileToGroup converts one parsed group file into the Group shape the line already runs on.
func groupFileToGroup(file GroupFile) Group {
	var fields Fields
	fields.Set("type", file.Type)
	fields.Set("version", file.Version)
	fields.Set("depends_on", toAnyList(file.DependsOn))
	if file.Mode != "" {
		fields.Set("mode", file.Mode)
	}
	if file.Base != "" {
		fields.Set("base", file.Base)
	}
	group := Group{ID: file.ID, Title: file.Title, Fields: fields, EpicID: file.EpicID}
	for _, task := range file.Tasks {
		group.Tasks = append(group.Tasks, groupTaskToTask(file, task))
	}
	return group
}

// groupTaskToTask converts one group file checkbox task into the Task shape the line already runs
// on: done once ticked, else its own status and priority when set, else its group's.
func groupTaskToTask(file GroupFile, task GroupTask) Task {
	status, priority := file.Status, file.Priority
	if task.Status != "" {
		status = task.Status
	}
	if task.Priority != "" {
		priority = task.Priority
	}
	if task.Done {
		status = "DONE"
	}
	var fields Fields
	fields.Set("files", toAnyList(task.Files))
	if len(task.Checks) > 0 {
		fields.Set("done_when", toAnyList(task.Checks))
	}
	if len(task.DependsOn) > 0 {
		fields.Set("depends_on", toAnyList(task.DependsOn))
	}
	if len(task.Context) > 0 {
		fields.Set("context", toAnyList(task.Context))
	}
	if task.Owner != "" {
		fields.Set("owner", task.Owner)
	}
	if task.Tier != "" {
		fields.Set("tier", task.Tier)
	}
	if len(task.Facets) > 0 {
		fields.Set("facets", toAnyList(task.Facets))
	}
	return Task{
		ID: task.ID, Title: task.Title, Priority: priority, Status: status,
		Fields: fields, Heading: task.Line, BlockStart: task.Line, BlockEnd: task.Line, GroupID: file.ID,
	}
}

// toAnyList widens a string slice into the []any a Fields list value holds.
func toAnyList(list []string) []any {
	out := make([]any, len(list))
	for index, item := range list {
		out[index] = item
	}
	return out
}
