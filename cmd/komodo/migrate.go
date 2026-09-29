package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
	"komodo/internal/line"
)

// runMigrate converts the repo's BACKLOG.md into one docs/backlog group file per group, in the
// current grammar; --dry-run prints the files it would write without writing them.
func runMigrate(root string, args []string) {
	set := flag.NewFlagSet("migrate", flag.ExitOnError)
	dryRun := set.Bool("dry-run", false, "print the files migrate would write, without writing them")
	_ = set.Parse(args)

	path, err := backlog.Find(root)
	if err != nil {
		fail(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	parsed := backlog.Parse(string(data))
	files := migrateGroupFiles(parsed)

	written := 0
	for _, file := range files {
		dest := filepath.Join(root, groupFilesDir, file.ID+"-"+backlog.Slug(file.Title)+".md")
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("skip %s: already exists\n", relPath(root, dest))
			continue
		}
		if *dryRun {
			fmt.Printf("write %s\n", relPath(root, dest))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(dest, []byte(backlog.RenderGroupFileDocument(file)), 0o644); err != nil {
			fail(err)
		}
		fmt.Printf("write %s\n", relPath(root, dest))
		written++
	}
	if *dryRun {
		fmt.Printf("%d group file(s) would be written; %s stays until it is removed by hand\n", len(files), path)
		return
	}
	fmt.Printf("%d group file(s) written; %s stays until it is removed by hand\n", written, path)
	line.Stamp(root, ledger.Entry{Station: "migrate", Task: "BACKLOG.md", Outcome: "migrated"})
}

// migrateGroupFiles converts every group of a parsed BACKLOG.md into a GroupFile, splitting an
// epic across a synthetic id per version so a group's version never disagrees with its epic's.
func migrateGroupFiles(parsed backlog.Backlog) []backlog.GroupFile {
	var out []backlog.GroupFile
	for _, group := range parsed.Groups {
		out = append(out, backlog.GroupFile{
			ID:        group.ID,
			Title:     group.Title,
			Priority:  migrateGroupPriority(group),
			Status:    migrateGroupStatus(group),
			Type:      group.Type(),
			Version:   group.Version(),
			EpicID:    migrateEpicID(parsed, group),
			DependsOn: group.DependsOn(),
			Tasks:     migrateGroupTasks(group),
		})
	}
	return out
}

// migrateEpicID keeps the group's own epic id when its version matches the epic's, else a
// per-version id, so an epic spanning several versions splits instead of failing lint.
func migrateEpicID(parsed backlog.Backlog, group backlog.Group) string {
	epic, ok := parsed.Epic(group.EpicID)
	if !ok || epic.Version() == "" || epic.Version() == group.Version() {
		return group.EpicID
	}
	return group.EpicID + "-" + group.Version()
}

// priorityRank orders a priority letter from most to least urgent, for finding a group's own.
var priorityRank = []string{"C", "H", "M", "L"}

// migrateGroupPriority is the most urgent priority among a group's tasks, M with none.
func migrateGroupPriority(group backlog.Group) string {
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

// migrateGroupStatus is a group file's own status: DONE once every task is, BLOCKED with one
// stopped, READY with one runnable, else REFINEMENT.
func migrateGroupStatus(group backlog.Group) string {
	if len(group.Tasks) == 0 {
		return "REFINEMENT"
	}
	allDone := true
	anyBlocked := false
	anyReady := false
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

// migrateGroupTasks converts every BACKLOG.md task of a group into a group-file checkbox task.
func migrateGroupTasks(group backlog.Group) []backlog.GroupTask {
	var out []backlog.GroupTask
	for _, task := range group.Tasks {
		out = append(out, backlog.GroupTask{
			ID:        task.ID,
			Title:     task.Title,
			Done:      task.Status == "DONE",
			Files:     task.Files(),
			Checks:    task.DoneWhen(),
			Owner:     task.Fields.String("owner"),
			Context:   task.Context(),
			DependsOn: task.DependsOn(),
			Priority:  task.Priority,
			Status:    task.Status,
		})
	}
	return out
}

// relPath is path relative to root, or path itself when it does not sit under root.
func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}
