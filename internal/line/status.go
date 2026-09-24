package line

import (
	"encoding/json"
	"os"
	"path/filepath"

	"komodo/internal/backlog"
)

// TaskStatus is one task's live status in the run.
type TaskStatus struct {
	Status string `json:"status"`
}

// statusPath is where a group keeps live task status until ship writes it into BACKLOG.md; a
// task no group names keeps it in the repo's own file.
func statusPath(root, group string) string {
	if group == "" {
		return filepath.Join(root, StateDir, "status.json")
	}
	return filepath.Join(RunDir(root, group), "status.json")
}

// statusFiles maps every live status file to its group: the repo's own under "", then each group's.
func statusFiles(root string) map[string]string {
	migrateRun(root)
	files := map[string]string{statusPath(root, ""): ""}
	entries, err := os.ReadDir(filepath.Join(root, StateDir, RunsDir))
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if entry.IsDir() {
			files[statusPath(root, entry.Name())] = entry.Name()
		}
	}
	return files
}

// loadStatusFile reads one live status file, empty when it is missing.
func loadStatusFile(path string) map[string]TaskStatus {
	statuses := map[string]TaskStatus{}
	data, err := os.ReadFile(path)
	if err != nil {
		return statuses
	}
	_ = json.Unmarshal(data, &statuses)
	return statuses
}

// LoadStatus reads every group's live task status, empty when no run has recorded any.
func LoadStatus(root string) map[string]TaskStatus {
	statuses := map[string]TaskStatus{}
	for path := range statusFiles(root) {
		for taskID, status := range loadStatusFile(path) {
			statuses[taskID] = status
		}
	}
	return statuses
}

// RecordStatus sets one task's live status in its group's status.json, written atomically.
func RecordStatus(root, taskID, status string) error {
	path := statusPath(root, GroupFor(root, taskID))
	statuses := loadStatusFile(path)
	statuses[taskID] = TaskStatus{Status: status}
	return saveStatus(path, statuses)
}

// saveStatus writes one live status file atomically, removing it when none remains.
func saveStatus(path string, statuses map[string]TaskStatus) error {
	if len(statuses) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "status-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// ClearStatus drops the named tasks' live status, once ship has written it into BACKLOG.md.
func ClearStatus(root string, taskIDs []string) error {
	return pruneStatus(root, nil, func(taskID string) bool { return !contains(taskIDs, taskID) })
}

// KeepStatus drops every task's live status except the named ones, so a new run starts clean.
func KeepStatus(root string, taskIDs []string) error {
	return pruneStatus(root, nil, func(taskID string) bool { return contains(taskIDs, taskID) })
}

// pruneStatus keeps only the live statuses keep accepts in every file whose group spare does not
// name, writing nothing when none change.
func pruneStatus(root string, spare map[string]bool, keep func(taskID string) bool) error {
	for path, group := range statusFiles(root) {
		if spare[group] {
			continue
		}
		statuses := loadStatusFile(path)
		changed := false
		for taskID := range statuses {
			if !keep(taskID) {
				delete(statuses, taskID)
				changed = true
			}
		}
		if !changed {
			continue
		}
		if err := saveStatus(path, statuses); err != nil {
			return err
		}
	}
	return nil
}

// OverlayStatus lays the run's live status over a parsed backlog, so a reader sees it as closed.
func OverlayStatus(parsed backlog.Backlog, statuses map[string]TaskStatus) backlog.Backlog {
	for gi := range parsed.Groups {
		for ti := range parsed.Groups[gi].Tasks {
			if status, ok := statuses[parsed.Groups[gi].Tasks[ti].ID]; ok && status.Status != "" {
				parsed.Groups[gi].Tasks[ti].Status = status.Status
			}
		}
	}
	return parsed
}

// LoadBacklog is root's BACKLOG.md with the statuses a group worktree's ship commit holds, then
// the run's live status, laid over it; root's own file is never rewritten mid-run.
func LoadBacklog(root string) (backlog.Backlog, string, error) {
	path, err := backlog.Find(root)
	if err != nil {
		return backlog.Backlog{}, "", err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return backlog.Backlog{}, "", err
	}
	shipped := shippedStatus(root, parsed)
	for id, status := range LoadStatus(root) {
		shipped[id] = status
	}
	return OverlayStatus(parsed, shipped), path, nil
}

// shippedStatus is every DONE or BLOCKED status a recorded run's worktree BACKLOG.md holds for
// one of its own group's tasks root still has open, which is what its ship commit carries until it lands.
func shippedStatus(root string, parsed backlog.Backlog) map[string]TaskStatus {
	statuses := map[string]TaskStatus{}
	for _, state := range LoadRuns(root) {
		group, ok := exactGroup(parsed, state.Group)
		if !ok {
			continue
		}
		worktree := state.Worktree
		if worktree == "" {
			worktree = filepath.Join(StateDir, "wt", state.Group)
		}
		path, err := backlog.Find(WorktreePath(root, worktree))
		if err != nil {
			continue
		}
		committed, err := backlog.Load(path)
		if err != nil {
			continue
		}
		for _, current := range group.Tasks {
			task, ok := committed.Task(current.ID)
			if ok && !task.Open() && current.Open() {
				statuses[task.ID] = TaskStatus{Status: task.Status}
			}
		}
	}
	return statuses
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
