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

// runMigrate moves a backlog into the tree: flat docs/backlog group files, this repo's BACKLOG.md, or a
// foreign BACKLOG.md or TODO.md; --dry-run prints what it would write without writing it.
func runMigrate(root string, args []string) {
	set := flag.NewFlagSet("migrate", flag.ExitOnError)
	dryRun := set.Bool("dry-run", false, "print the files migrate would write, without writing them")
	_ = set.Parse(args)

	files, source, skipped, err := migrateSource(root)
	if err != nil {
		fail(err)
	}
	for _, skip := range skipped {
		fmt.Printf("%s:%d: could not place: %s\n", relPath(root, source), skip.Line, skip.Text)
	}
	epics := migrateEpics(files)
	written := 0
	for _, epic := range epics {
		path := filepath.Join(backlog.EpicDir(root, epic.ID), backlog.EpicFileName)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if *dryRun {
			fmt.Printf("write %s\n", relPath(root, path))
			continue
		}
		if _, err := backlog.WriteEpic(root, epic); err != nil {
			fail(err)
		}
		fmt.Printf("write %s\n", relPath(root, path))
	}
	for _, file := range files {
		if reason := migratePlaceProblem(root, epics, file); reason != "" {
			fmt.Printf("skip %s: %s\n", file.ID, reason)
			continue
		}
		dir := backlog.GroupDirPath(root, file.ID)
		if _, err := os.Stat(filepath.Join(dir, backlog.GroupFileName)); err == nil {
			fmt.Printf("skip %s: already exists\n", relPath(root, dir))
			continue
		}
		written++
		if *dryRun {
			fmt.Printf("write %s\n", relPath(root, dir))
			continue
		}
		if _, err := backlog.WriteGroup(root, file); err != nil {
			fail(err)
		}
		fmt.Printf("write %s\n", relPath(root, dir))
	}
	if *dryRun {
		fmt.Printf("%d group folder(s) would be written; %s stays until it is removed by hand\n", written, relPath(root, source))
		return
	}
	fmt.Printf("%d group folder(s) written; %s stays until it is removed by hand\n", written, relPath(root, source))
	line.Stamp(root, ledger.Entry{Station: "migrate", Task: relPath(root, source), Outcome: "migrated"})
}

// migrateEpics is one EpicFile per epic folder, by group number, at the first group's version and type.
func migrateEpics(files []backlog.GroupFile) []backlog.EpicFile {
	seen := map[string]bool{}
	var out []backlog.EpicFile
	for _, file := range files {
		id := backlog.EpicIDOfGroup(file.ID)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, backlog.EpicFile{ID: id, Title: "Epic " + id, Status: "READY", Version: file.Version,
			Type: file.Type, GroupsMax: backlog.DefaultGroupsMax})
	}
	return out
}

// migratePlaceProblem is why a group cannot land in the tree, or empty: its version differs from its epic folder's.
func migratePlaceProblem(root string, epics []backlog.EpicFile, file backlog.GroupFile) string {
	id := backlog.EpicIDOfGroup(file.ID)
	version := ""
	if data, err := os.ReadFile(filepath.Join(backlog.EpicDir(root, id), backlog.EpicFileName)); err == nil {
		version = backlog.ParseEpicFile(string(data)).Version
	} else {
		for _, epic := range epics {
			if epic.ID == id {
				version = epic.Version
			}
		}
	}
	if file.Version != version {
		return fmt.Sprintf("version %s differs from %s's %s; the tree holds one version per epic, so renumber it under its own epic",
			file.Version, id, version)
	}
	return ""
}

// migrateSource picks what migrate converts: flat group files under docs/backlog, else this repo's own
// BACKLOG.md, else a foreign BACKLOG.md or TODO.md through Import, reporting the lines Import could not place.
func migrateSource(root string) (files []backlog.GroupFile, source string, skipped []backlog.ImportSkip, err error) {
	flat, err := backlog.FlatGroupFiles(root)
	if err != nil {
		return nil, "", nil, err
	}
	if len(flat) > 0 {
		files, err := backlog.LoadFlatGroupFiles(flat)
		return files, filepath.Join(root, backlog.GroupFilesDir, "*.md"), nil, err
	}
	if path, findErr := backlog.Find(root); findErr == nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, "", nil, readErr
		}
		parsed := backlog.Parse(string(data))
		if len(parsed.Groups) > 0 {
			return migrateGroupFiles(parsed), path, nil, nil
		}
		result := backlog.Import(string(data), relPath(root, path))
		return result.Groups, path, result.Skipped, nil
	}
	todo := filepath.Join(root, "TODO.md")
	data, readErr := os.ReadFile(todo)
	if readErr != nil {
		return nil, "", nil, fmt.Errorf("no flat docs/backlog files, BACKLOG.md or TODO.md at %s to migrate", root)
	}
	result := backlog.Import(string(data), "TODO.md")
	return result.Groups, todo, result.Skipped, nil
}

// migrateGroupFiles converts every open group of a parsed BACKLOG.md into a GroupFile.
func migrateGroupFiles(parsed backlog.Backlog) []backlog.GroupFile {
	var out []backlog.GroupFile
	for _, group := range parsed.Groups {
		// A group whose every task is DONE stays history in CHANGELOG.md and git.
		if migrateGroupStatus(group) == "DONE" {
			continue
		}
		out = append(out, backlog.GroupFile{
			ID:        group.ID,
			Title:     group.Title,
			Priority:  migrateGroupPriority(group),
			Status:    migrateGroupStatus(group),
			Type:      group.Type(),
			Version:   group.Version(),
			EpicID:    group.EpicID,
			Mode:      group.Fields.String("mode"),
			Base:      group.Base(),
			DependsOn: group.DependsOn(),
			Tasks:     migrateGroupTasks(group),
		})
	}
	return out
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
			Tier:      task.Tier(),
			Facets:    task.Facets(),
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
