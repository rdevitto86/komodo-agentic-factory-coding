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

// statusPath is where the run keeps live task status until ship writes it into BACKLOG.md.
func statusPath(root string) string {
	return filepath.Join(root, StateDir, "status.json")
}

// LoadStatus reads the run's live task status, empty when the run has recorded none.
func LoadStatus(root string) map[string]TaskStatus {
	statuses := map[string]TaskStatus{}
	data, err := os.ReadFile(statusPath(root))
	if err != nil {
		return statuses
	}
	_ = json.Unmarshal(data, &statuses)
	return statuses
}

// RecordStatus sets one task's live status in .komodo/status.json, written atomically.
func RecordStatus(root, taskID, status string) error {
	statuses := LoadStatus(root)
	statuses[taskID] = TaskStatus{Status: status}
	return saveStatus(root, statuses)
}

// saveStatus writes the run's live status atomically, removing the file when none remains.
func saveStatus(root string, statuses map[string]TaskStatus) error {
	if len(statuses) == 0 {
		if err := os.Remove(statusPath(root)); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return err
	}
	path := statusPath(root)
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
	return pruneStatus(root, func(taskID string) bool { return !contains(taskIDs, taskID) })
}

// KeepStatus drops every task's live status except the named ones, so a new run starts clean.
func KeepStatus(root string, taskIDs []string) error {
	return pruneStatus(root, func(taskID string) bool { return contains(taskIDs, taskID) })
}

// pruneStatus keeps only the live statuses keep accepts, writing nothing when none change.
func pruneStatus(root string, keep func(taskID string) bool) error {
	statuses := LoadStatus(root)
	changed := false
	for taskID := range statuses {
		if !keep(taskID) {
			delete(statuses, taskID)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return saveStatus(root, statuses)
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

// shippedStatus is every DONE or BLOCKED status the recorded run's worktree BACKLOG.md holds for
// one of its group's tasks root still has open, which is what its ship commit carries until it lands.
func shippedStatus(root string, parsed backlog.Backlog) map[string]TaskStatus {
	statuses := map[string]TaskStatus{}
	state, err := LoadRun(root)
	if err != nil || state.Group == "" {
		return statuses
	}
	group, ok := parsed.Group(state.Group)
	if !ok {
		return statuses
	}
	worktree := state.Worktree
	if worktree == "" {
		worktree = filepath.Join(StateDir, "wt", state.Group)
	}
	path, err := backlog.Find(WorktreePath(root, worktree))
	if err != nil {
		return statuses
	}
	committed, err := backlog.Load(path)
	if err != nil {
		return statuses
	}
	for _, current := range group.Tasks {
		task, ok := committed.Task(current.ID)
		if ok && !task.Open() && current.Open() {
			statuses[task.ID] = TaskStatus{Status: task.Status}
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
