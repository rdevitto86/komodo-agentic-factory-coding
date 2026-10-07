// Package backlogtest seeds a test repo's docs/backlog tree through the package's own renderer, so a
// fixture never drifts from the grammar it exercises.
package backlogtest

import (
	"os"
	"path/filepath"
	"testing"

	"komodo/internal/backlog"
)

// Seed writes each group as an epic folder, a group folder and one task file per task under root.
func Seed(t *testing.T, root string, groups ...backlog.GroupFile) {
	t.Helper()
	for _, group := range groups {
		epicID := group.EpicID
		if epicID == "" {
			epicID = backlog.EpicIDOfGroup(group.ID)
		}
		epicPath := filepath.Join(backlog.EpicDir(root, epicID), backlog.EpicFileName)
		if _, err := os.Stat(epicPath); err != nil {
			epic := backlog.EpicFile{ID: epicID, Title: "Epic " + epicID, Status: "READY", Version: group.Version,
				Type: group.Type, GroupsMax: 99}
			if _, err := backlog.WriteEpic(root, epic); err != nil {
				t.Fatal(err)
			}
		}
		if backlog.EpicIDOfGroup(group.ID) != epicID {
			// A group filed under another epic's folder still needs that folder; the loader reports the mismatch.
			dir := filepath.Join(backlog.EpicDir(root, epicID), backlog.GroupDirName(group.ID))
			writeGroupDir(t, dir, group)
			continue
		}
		if _, err := backlog.WriteGroup(root, group); err != nil {
			t.Fatal(err)
		}
	}
}

// writeGroupDir writes a group folder at dir directly, for a fixture that places a group off its number.
func writeGroupDir(t *testing.T, dir string, group backlog.GroupFile) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, backlog.GroupFileName), []byte(backlog.RenderGroupHeader(group)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, task := range group.Tasks {
		if err := os.WriteFile(filepath.Join(dir, backlog.TaskFileName(task.ID)), []byte(backlog.RenderGroupFileTask(task)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// SeedText parses an old-grammar backlog fixture and writes its groups into the tree, the same
// way `komodo migrate` would, so a legacy fixture never has to be hand-translated.
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
			EpicID: legacyEpicID(parsed, group), Mode: group.Fields.String("mode"), Base: group.Base(),
			DependsOn: group.DependsOn(),
			Tasks:     legacyGroupTasks(group),
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
			Tier: task.Tier(), Facets: task.Facets(),
		})
	}
	return out
}
