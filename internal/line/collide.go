package line

import (
	"fmt"
	"strings"

	"komodo/internal/backlog"
)

// RefuseCollision refuses to brief a task whose claimed files overlap a closed, unmerged task
// branch of the open run, since QC would stop on a conflict a person has to resolve.
func RefuseCollision(root, taskID string) error {
	state, err := LoadRun(root)
	if err != nil || state.Branch == "" {
		return nil
	}
	path, err := backlog.Find(root)
	if err != nil {
		return nil
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil
	}
	task, ok := parsed.Task(taskID)
	if !ok {
		return nil
	}
	merged := map[string]bool{}
	for _, name := range strings.Split(gitOr(root, "branch", "--merged", state.Branch, "--format=%(refname:short)"), "\n") {
		merged[strings.TrimSpace(name)] = true
	}
	for _, wave := range state.Waves {
		for _, other := range wave {
			if other == taskID {
				continue
			}
			branch := TaskBranch(other)
			if _, err := git(root, "rev-parse", "--verify", "refs/heads/"+branch); err != nil || merged[branch] {
				continue
			}
			candidate, ok := parsed.Task(other)
			if ok && claimsOverlap(task, candidate) {
				return fmt.Errorf("%s shares a file with %s, whose branch %s is closed but not merged into %s; merge that wave first, or pick disjoint files",
					taskID, other, branch, state.Branch)
			}
		}
	}
	return nil
}

// gitOr runs one git command and returns its output, or nothing when it fails.
func gitOr(dir string, args ...string) string {
	out, err := git(dir, args...)
	if err != nil {
		return ""
	}
	return out
}
