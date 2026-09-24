package line

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/backlog"
)

// TaskStatus is one task's live status in the run, and the note that explains it.
type TaskStatus struct {
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
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
func RecordStatus(root, taskID, status, note string) error {
	statuses := LoadStatus(root)
	statuses[taskID] = TaskStatus{Status: status, Note: note}
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

// ClearStatus drops the run's live status once ship has written it into BACKLOG.md.
func ClearStatus(root string) error {
	if err := os.Remove(statusPath(root)); err != nil && !os.IsNotExist(err) {
		return err
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

// shippedStatus is every DONE or BLOCKED status a group worktree's BACKLOG.md holds for a task
// root still has open, which is what a ship commit carries until its pull request lands.
func shippedStatus(root string, parsed backlog.Backlog) map[string]TaskStatus {
	statuses := map[string]TaskStatus{}
	entries, err := os.ReadDir(filepath.Join(root, StateDir, "wt"))
	if err != nil {
		return statuses
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "TG-") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path, err := backlog.Find(filepath.Join(root, StateDir, "wt", name))
		if err != nil {
			continue
		}
		committed, err := backlog.Load(path)
		if err != nil {
			continue
		}
		for _, task := range committed.Tasks() {
			if task.Open() {
				continue
			}
			if current, ok := parsed.Task(task.ID); ok && current.Open() {
				statuses[task.ID] = TaskStatus{Status: task.Status}
			}
		}
	}
	return statuses
}

// statusNote is the attempt count and the first line of the failure a status carries.
func statusNote(attempt Attempt) string {
	return fmt.Sprintf("attempt %d: %s", attempt.Count, firstLine(attempt.Failure))
}

// sortedKeys is a status map's task ids in order, so a ship writes them the same way every time.
func sortedKeys(statuses map[string]TaskStatus) []string {
	keys := make([]string, 0, len(statuses))
	for id := range statuses {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	return keys
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
