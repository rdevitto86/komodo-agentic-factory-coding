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
// epic's folder, and a worktree under .komodo/wt or a run's branch or refs/komodo tip no open group owns.
func Leftovers(root string) []string {
	tree := groupFiles(root)
	open := openGroups(tree)
	notes := endedEpicFiles(root, tree)
	notes = append(notes, oldHarnessLeftovers(root)...)
	worktrees, named := orphanWorktrees(root, open, openEpicBranches(tree))
	notes = append(notes, worktrees...)
	return append(notes, orphanBranches(root, open, named)...)
}

// FlatBacklogFiles names each group file still lying flat under docs/backlog, outside the tree.
func FlatBacklogFiles(root string) []string {
	tree := groupFiles(root)
	var notes []string
	for _, path := range tree.Flat {
		notes = append(notes, rel(root, path)+": a group file outside the tree; run komodo migrate, then remove it")
	}
	return notes
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

// groupFiles reads the docs/backlog tree, empty when root holds none or it cannot be read.
func groupFiles(root string) backlog.Tree {
	tree, err := backlog.LoadTree(root)
	if err != nil {
		return backlog.Tree{}
	}
	return tree
}

// openGroups is every group with a task left open, keyed by the group's id and each open task's id,
// the names a line worktree takes.
func openGroups(tree backlog.Tree) map[string]bool {
	open := map[string]bool{}
	for _, group := range tree.Groups {
		for _, task := range group.File.Tasks {
			if !task.Done {
				open[group.File.ID], open[task.ID] = true, true
			}
		}
	}
	return open
}

// endedEpicFiles names each epic folder whose every group has every task ticked.
func endedEpicFiles(root string, tree backlog.Tree) []string {
	running := map[string]bool{}
	for _, epic := range tree.Epics {
		// An epic folder that fails to parse may hide open work, so it stays open.
		running[epic.ID] = len(epic.Problems) > 0
	}
	for _, group := range tree.Groups {
		for _, task := range group.File.Tasks {
			running[group.Epic.ID] = running[group.Epic.ID] || !task.Done
		}
		// A group with no parsed task or a parse problem may hide open work, so its epic stays open.
		if len(group.Problems) > 0 || len(group.File.Problems) > 0 || len(group.File.Tasks) == 0 {
			running[group.Epic.ID] = true
		}
	}
	grouped := map[string]bool{}
	for _, group := range tree.Groups {
		grouped[group.Epic.ID] = true
	}
	var notes []string
	for _, epic := range tree.Epics {
		if epic.ID == "" || !grouped[epic.ID] || running[epic.ID] {
			continue
		}
		notes = append(notes, rel(root, backlog.EpicDir(root, epic.ID))+": "+epic.ID+" has ended; once it reaches the default branch, komodo sync opens its cleanup PR")
	}
	return notes
}

// cleanupPrefix names sync's own transient worktree, cleanup-<epic>, which never outlives its PR.
const cleanupPrefix = "cleanup-"

// openEpicBranches is the branch each epic with an open task ships on.
func openEpicBranches(tree backlog.Tree) map[string]bool {
	epics := map[string]bool{}
	for _, group := range tree.Groups {
		if group.Epic.ID == "" || group.File.Version == "" {
			continue
		}
		for _, task := range group.File.Tasks {
			if !task.Done {
				epics["feat/"+group.File.Version] = true
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
