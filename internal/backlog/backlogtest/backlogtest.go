// Package backlogtest seeds a test repo's docs/backlog group files through the package's own
// renderer, so a fixture never drifts from the grammar it exercises.
package backlogtest

import (
	"os"
	"path/filepath"
	"testing"

	"komodo/internal/backlog"
)

// Seed writes each group as docs/backlog/<group-id>-<slug>.md under root, through the renderer.
func Seed(t *testing.T, root string, groups ...backlog.GroupFile) {
	t.Helper()
	dir := filepath.Join(root, backlog.GroupFilesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		path := filepath.Join(dir, group.ID+"-"+backlog.Slug(group.Title)+".md")
		text := backlog.RenderGroupFileDocument(group)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// SeedText parses a legacy BACKLOG.md fixture and writes its groups as group files, the same way
// `komodo migrate` would, so an old-grammar fixture never has to be hand-translated.
func SeedText(t *testing.T, root, text string) {
	t.Helper()
	Seed(t, root, legacyGroupFiles(backlog.Parse(text))...)
}

// legacyGroupFiles converts every legacy-grammar group into the shape a group file holds, the
// group's own status and priority collapsed from its tasks' worst case, as migrate does.
func legacyGroupFiles(parsed backlog.Backlog) []backlog.GroupFile {
	var out []backlog.GroupFile
	for _, group := range parsed.Groups {
		out = append(out, backlog.GroupFile{
			ID: group.ID, Title: group.Title, Priority: legacyGroupPriority(group),
			Status: legacyGroupStatus(group), Type: group.Type(), Version: group.Version(),
			EpicID: legacyEpicID(parsed, group), DependsOn: group.DependsOn(),
			Tasks: legacyGroupTasks(group),
		})
	}
	return out
}

// legacyEpicID keeps the group's own epic id when its version matches the epic's, else a
// per-version id, mirroring migrate's split for an epic spanning several versions.
func legacyEpicID(parsed backlog.Backlog, group backlog.Group) string {
	epic, ok := parsed.Epic(group.EpicID)
	if !ok || epic.Version() == "" || epic.Version() == group.Version() {
		return group.EpicID
	}
	return group.EpicID + "-" + group.Version()
}

// priorityRank orders a priority letter from most to least urgent, for finding a group's own.
var priorityRank = []string{"C", "H", "M", "L"}

// legacyGroupPriority is the most urgent priority among a group's tasks, M with none.
func legacyGroupPriority(group backlog.Group) string {
	best := ""
	for _, task := range group.Tasks {
		if best == "" || rankIndex(task.Priority) < rankIndex(best) {
			best = task.Priority
		}
	}
	if best == "" {
		return "M"
	}
	return best
}

// rankIndex is a priority letter's position in priorityRank, last with an unknown letter.
func rankIndex(priority string) int {
	for index, letter := range priorityRank {
		if letter == priority {
			return index
		}
	}
	return len(priorityRank)
}

// legacyGroupStatus is a group file's own status: DONE once every task is, BLOCKED with one
// stopped, READY with one runnable, else REFINEMENT.
func legacyGroupStatus(group backlog.Group) string {
	if len(group.Tasks) == 0 {
		return "REFINEMENT"
	}
	allDone, anyBlocked, anyReady := true, false, false
	for _, task := range group.Tasks {
		if task.Status != "DONE" {
			allDone = false
		}
		if task.Status == "BLOCKED" {
			anyBlocked = true
		}
		if task.Ready() {
			anyReady = true
		}
	}
	switch {
	case allDone:
		return "DONE"
	case anyBlocked:
		return "BLOCKED"
	case anyReady:
		return "READY"
	default:
		return "REFINEMENT"
	}
}

// legacyGroupTasks converts every legacy-grammar task of a group into a group-file checkbox task.
func legacyGroupTasks(group backlog.Group) []backlog.GroupTask {
	var out []backlog.GroupTask
	for _, task := range group.Tasks {
		out = append(out, backlog.GroupTask{
			ID: task.ID, Title: task.Title, Done: task.Status == "DONE",
			Files: task.Files(), Checks: task.DoneWhen(), Owner: task.Fields.String("owner"),
			Context: task.Context(), DependsOn: task.DependsOn(),
			Priority: task.Priority, Status: task.Status,
		})
	}
	return out
}
