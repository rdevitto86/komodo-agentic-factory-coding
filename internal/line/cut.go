package line

import (
	"fmt"
	"os"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
	"komodo/internal/mount"
	"komodo/internal/plan"
)

// RefuseOpenRun refuses to cut a group while another open, unshipped group claims a file it
// claims, since their task branches would conflict; a group it cannot read counts as overlapping.
func RefuseOpenRun(root, group string) error {
	for _, state := range OpenRuns(root) {
		if state.Group == group || !groupsOverlap(root, state.Group, group) {
			continue
		}
		return fmt.Errorf("%s is open and not shipped and shares files with %s; finish it with komodo step %s, or cut %s anyway with --force",
			state.Group, group, state.Group, group)
	}
	return nil
}

// groupsOverlap reports whether any task of one group claims a file a task of the other claims.
func groupsOverlap(root, left, right string) bool {
	path, err := backlog.Find(root)
	if err != nil {
		return true
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return true
	}
	first, okFirst := exactGroup(parsed, left)
	second, okSecond := exactGroup(parsed, right)
	if !okFirst || !okSecond {
		return true
	}
	// A task claiming no files may touch any, so its group overlaps every other.
	if claimsNothing(first) || claimsNothing(second) {
		return true
	}
	for _, a := range first.Tasks {
		for _, b := range second.Tasks {
			if plan.Overlap(a, b) {
				return true
			}
		}
	}
	return false
}

// claimsNothing reports whether any task of the group declares no files.
func claimsNothing(group backlog.Group) bool {
	for _, task := range group.Tasks {
		declared := false
		for _, file := range task.Files() {
			if strings.TrimSpace(file) != "" {
				declared = true
			}
		}
		if !declared {
			return true
		}
	}
	return false
}

// exactGroup is the group whose id is exactly id, never a title match.
func exactGroup(parsed backlog.Backlog, id string) (backlog.Group, bool) {
	for _, group := range parsed.Groups {
		if group.ID == id {
			return group, true
		}
	}
	return backlog.Group{}, false
}

// Start cuts the group branch in its own worktree from the base and records the choice, holding the
// cut lock from the lock and overlap checks through the saved run; force skips the overlap check.
func Start(root string, plan *Plan, base string, force bool) (RunState, error) {
	release, err := acquireCutLock(root)
	if err != nil {
		return RunState{}, err
	}
	defer release()
	if err := CheckLock(root, plan.Group); err != nil {
		return RunState{}, err
	}
	if !force {
		if err := RefuseOpenRun(root, plan.Group); err != nil {
			return RunState{}, err
		}
	}
	if base != "" {
		plan.Base = base
	}
	if err := Fetch(root, plan.Base); err != nil {
		return RunState{}, err
	}
	path := WorktreePath(root, plan.Worktree)
	if _, err := os.Stat(path); err != nil {
		if err := AddWorktree(root, plan.Branch, plan.Base, path); err != nil {
			return RunState{}, err
		}
	}
	if err := RenderProject(root, path); err != nil {
		return RunState{}, err
	}
	state := RunState{
		Run:     fmt.Sprintf("%s-%d", plan.Group, time.Now().Unix()),
		Group:   plan.Group,
		Base:    plan.Base,
		Branch:  plan.Branch,
		Started: time.Now().UTC(),
	}
	state.Worktree = path
	state.Waves = plan.Waves
	others := map[string]bool{}
	for _, open := range OpenRuns(root) {
		if open.Group != plan.Group {
			others[open.Group] = true
		}
	}
	for _, old := range LoadRuns(root) {
		// A shipped group's record goes, so only open groups keep a run directory.
		if old.Group != plan.Group && !others[old.Group] {
			if err := os.RemoveAll(RunDir(root, old.Group)); err != nil {
				return state, err
			}
		}
	}
	if err := keepGroupStatus(root, plan.Group); err != nil {
		return state, err
	}
	// Another open group's stations still read the run ledger, so it is archived only when this run is alone.
	if len(others) == 0 {
		if err := Book(root).TruncateRun(); err != nil {
			return state, err
		}
	}
	if err := SaveRun(root, state); err != nil {
		return state, err
	}
	Stamp(root, ledger.Entry{Run: state.Run, Group: state.Group, Station: "intake", Outcome: "started"})
	return state, nil
}

// keepGroupStatus drops the live status of every task outside groupID, so another group's never
// ships here, sparing each other open group's own status file.
func keepGroupStatus(root, groupID string) error {
	path, err := backlog.Find(root)
	if err != nil {
		return err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return err
	}
	var taskIDs []string
	if group, ok := parsed.Group(groupID); ok {
		for _, task := range group.Tasks {
			taskIDs = append(taskIDs, task.ID)
		}
	}
	spare := map[string]bool{}
	for _, open := range OpenRuns(root) {
		if open.Group != groupID {
			spare[open.Group] = true
		}
	}
	return pruneStatus(root, spare, func(taskID string) bool { return contains(taskIDs, taskID) })
}

// RenderProject rebuilds the worktree's gitignored project config for every host installed on
// root, from the profile and the repo layer, so a run always has the right tools.
func RenderProject(root, worktree string) error {
	return render(root, worktree, true)
}

// RenderRoot rewrites every installed host's whole config at root, agents included, as the install does.
func RenderRoot(root string) error {
	return render(root, root, false)
}

// render applies each installed host's plan at worktree, narrowed to the project copies when projectOnly is set.
func render(root, worktree string, projectOnly bool) error {
	binary := mount.BinaryPath()
	for _, host := range mount.Active() {
		if host.Installed == nil || host.Render == nil || !host.Installed(root) {
			continue
		}
		plan, err := host.Render(worktree, binary)
		if err != nil {
			return err
		}
		if projectOnly {
			plan = plan.Project()
		}
		if _, err := plan.Apply(); err != nil {
			return err
		}
	}
	return nil
}
