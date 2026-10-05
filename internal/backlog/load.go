package backlog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GroupFilesDir holds one folder per epic, one per group, and one file per task.
const GroupFilesDir = "docs/backlog"

// LoadRoot loads the queue at root: every epic, group and task under docs/backlog, merged into one
// Backlog, empty when root holds none.
func LoadRoot(root string) (Backlog, error) {
	tree, err := LoadTree(root)
	if err != nil {
		return Backlog{}, err
	}
	return FromTree(tree), nil
}

// Exists reports whether root holds a queue: at least one group folder under docs/backlog.
func Exists(root string) bool {
	tree, err := LoadTree(root)
	return err == nil && len(tree.Groups) > 0
}

// FromTree converts a loaded tree into the shape the line runs on.
func FromTree(tree Tree) Backlog {
	out := Backlog{Problems: append([]string(nil), tree.Problems...)}
	for _, epic := range tree.Epics {
		out.Epics = append(out.Epics, Epic{ID: epic.ID, Title: epic.Title, Status: epic.Status,
			Goal: epic.Goal, GroupsMax: epic.GroupsMax, version: epic.Version})
	}
	for _, group := range tree.Groups {
		for _, problem := range group.File.Problems {
			out.Problems = append(out.Problems, group.Path+": "+problem)
		}
		if group.File.ID == "" {
			continue
		}
		out.Groups = append(out.Groups, groupFileToGroup(group.File))
	}
	return out
}

// FlatGroupFiles lists every *.md directly under root's docs/backlog, the layout before the tree.
func FlatGroupFiles(root string) ([]string, error) {
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

// LoadFlatGroupFiles parses every flat group file at the given paths, for migrate to move into the tree.
func LoadFlatGroupFiles(paths []string) ([]GroupFile, error) {
	var out []GroupFile
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		file := ParseGroupFile(string(data))
		if file.ID == "" {
			continue
		}
		out = append(out, file)
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
