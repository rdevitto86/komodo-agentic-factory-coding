package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/line"
)

// Leftovers names what an ended epic, a stale local.json, or a finished run left behind: an ended
// epic's group files, and a worktree under .komodo/wt or a run's branch or refs/komodo tip no open group owns.
func Leftovers(root string) []string {
	paths, files := groupFiles(root)
	open := openGroups(files)
	notes := endedEpicFiles(root, paths, files)
	notes = append(notes, oldHarnessLeftovers(root)...)
	worktrees, named := orphanWorktrees(root, open, openEpicBranches(files))
	notes = append(notes, worktrees...)
	return append(notes, orphanBranches(root, open, named)...)
}

// oldHarnessLeftovers names a .komodo/local.json a prior harness wrote, still holding its base key.
func oldHarnessLeftovers(root string) []string {
	path := filepath.Join(root, ".komodo", "local.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var parsed map[string]any
	if json.Unmarshal(data, &parsed) != nil {
		return nil
	}
	if _, ok := parsed["base"]; !ok {
		return nil
	}
	return []string{rel(root, path) + ": a base key from an earlier harness; komodo no longer reads it"}
}

// groupFiles parses every docs/backlog group file, returning their paths in order and each one's parse.
func groupFiles(root string) ([]string, map[string]backlog.GroupFile) {
	paths, err := filepath.Glob(filepath.Join(root, "docs", "backlog", "*.md"))
	if err != nil {
		return nil, nil
	}
	files := make(map[string]backlog.GroupFile, len(paths))
	for _, path := range paths {
		if data, err := os.ReadFile(path); err == nil {
			files[path] = backlog.ParseGroupFile(string(data))
		}
	}
	return paths, files
}

// openGroups is every group with a task left open, keyed by the group's id and each open task's id,
// the names a line worktree takes.
func openGroups(files map[string]backlog.GroupFile) map[string]bool {
	open := map[string]bool{}
	for _, file := range files {
		for _, task := range file.Tasks {
			if !task.Done {
				open[file.ID], open[task.ID] = true, true
			}
		}
	}
	return open
}

// endedEpicFiles names each group file of an epic whose every task is ticked.
func endedEpicFiles(root string, paths []string, files map[string]backlog.GroupFile) []string {
	running := map[string]bool{}
	for _, file := range files {
		for _, task := range file.Tasks {
			running[file.EpicID] = running[file.EpicID] || !task.Done
		}
		// A file with no parsed task or a parse problem may hide open work, so its epic stays open.
		if len(file.Problems) > 0 || len(file.Tasks) == 0 {
			running[file.EpicID] = true
		}
	}
	var notes []string
	for _, path := range paths {
		file, ok := files[path]
		if !ok || file.EpicID == "" || running[file.EpicID] {
			continue
		}
		notes = append(notes, rel(root, path)+": "+file.EpicID+" has ended; once it reaches the default branch, komodo sync opens its cleanup PR")
	}
	return notes
}

// cleanupPrefix names sync's own transient worktree, cleanup-<epic>, which never outlives its PR.
const cleanupPrefix = "cleanup-"

// openEpicBranches is the branch each epic with an open task ships on.
func openEpicBranches(files map[string]backlog.GroupFile) map[string]bool {
	epics := map[string]bool{}
	for _, file := range files {
		if file.EpicID == "" || file.Version == "" {
			continue
		}
		for _, task := range file.Tasks {
			if !task.Done {
				epics["feat/"+file.Version] = true
				break
			}
		}
	}
	return epics
}

// orphanWorktrees names each worktree under the main checkout's .komodo/wt that no open group or epic
// branch owns, skipping sync's own cleanup-* ones, returning the notes and the branches they name.
func orphanWorktrees(root string, open, epics map[string]bool) ([]string, map[string]bool) {
	named := map[string]bool{}
	worktrees, err := git.Worktrees(root)
	if err != nil {
		return nil, named
	}
	var notes []string
	var parked string
	for index, current := range worktrees {
		if index == 0 {
			parked = filepath.Join(current.Path, line.StateDir, "wt") + string(filepath.Separator)
			continue
		}
		path := filepath.Clean(current.Path)
		if !strings.HasPrefix(path, parked) || open[filepath.Base(path)] || strings.HasPrefix(filepath.Base(path), cleanupPrefix) {
			continue
		}
		branch := TrackedOf(current)
		if branch == "" {
			notes = append(notes, current.Path+" detached: no open group owns this worktree")
			continue
		}
		if epics[branch] {
			continue
		}
		named[branch] = true
		notes = append(notes, current.Path+" on branch "+branch+": no open group owns this worktree")
	}
	return notes, named
}

// orphanBranches names each local branch or refs/komodo tip a recorded run cut for a group with no open task, skipping
// the branches a worktree note already named.
func orphanBranches(root string, open, named map[string]bool) []string {
	var notes []string
	for _, state := range line.LoadRuns(root) {
		if state.Branch == "" || open[state.Group] || named[state.Branch] {
			continue
		}
		_, headErr := git.Run(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+state.Branch)
		_, tipErr := git.Run(root, "rev-parse", "--verify", "--quiet", line.TipRef(state.Branch))
		if headErr != nil && tipErr != nil {
			continue
		}
		named[state.Branch] = true
		notes = append(notes, "branch "+state.Branch+": its group "+state.Group+" is no longer open")
	}
	return notes
}
