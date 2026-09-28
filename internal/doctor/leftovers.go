package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/line"
)

// Leftovers names what an ended epic or a finished run left behind: an ended epic's group files, and a worktree
// under .komodo/wt or a run's local branch no open group owns; it never fails a check.
func Leftovers(root string) []string {
	paths, files := groupFiles(root)
	open := openGroups(root, files)
	notes := endedEpicFiles(root, paths, files)
	worktrees, named := orphanWorktrees(root, open)
	notes = append(notes, worktrees...)
	return append(notes, orphanBranches(root, open, named)...)
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

// openGroups is every group with a task left open, in BACKLOG.md or a group file, keyed by the group's
// id and each open task's id, the names a line worktree takes.
func openGroups(root string, files map[string]backlog.GroupFile) map[string]bool {
	open := map[string]bool{}
	if path, err := backlog.Find(root); err == nil {
		if parsed, err := backlog.Load(path); err == nil {
			for _, group := range parsed.Groups {
				for _, task := range group.Tasks {
					if task.Open() {
						open[group.ID], open[task.ID] = true, true
					}
				}
			}
		}
	}
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
	}
	var notes []string
	for _, path := range paths {
		file, ok := files[path]
		if !ok || file.EpicID == "" || running[file.EpicID] {
			continue
		}
		notes = append(notes, rel(root, path)+": "+file.EpicID+" has ended; komodo sync opens its cleanup PR")
	}
	return notes
}

// orphanWorktrees names each worktree under the main checkout's .komodo/wt that no open group owns,
// returning the notes and the branches they name.
func orphanWorktrees(root string, open map[string]bool) ([]string, map[string]bool) {
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
		if !strings.HasPrefix(path, parked) || open[filepath.Base(path)] {
			continue
		}
		named[current.Branch] = true
		notes = append(notes, current.Path+" on branch "+current.Branch+": no open group owns this worktree")
	}
	return notes, named
}

// orphanBranches names each local branch a recorded run cut for a group with no open task, skipping
// the branches a worktree note already named.
func orphanBranches(root string, open, named map[string]bool) []string {
	var notes []string
	for _, state := range line.LoadRuns(root) {
		if state.Branch == "" || open[state.Group] || named[state.Branch] {
			continue
		}
		if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+state.Branch); err != nil {
			continue
		}
		named[state.Branch] = true
		notes = append(notes, "branch "+state.Branch+": its group "+state.Group+" is no longer open")
	}
	return notes
}
