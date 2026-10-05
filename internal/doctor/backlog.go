package doctor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/git"
)

// stallTasks is how many REFINEMENT tasks a backlog may hold before it is a pile, not a plan.
const stallTasks = 50

// stallAge is how long a REFINEMENT group may go without a commit touching it.
const stallAge = 14 * 24 * time.Hour

// checkStalledBacklog reports a backlog the line cannot drain: more than stallTasks open REFINEMENT
// tasks, or a REFINEMENT group folder no commit has touched in stallAge.
func checkStalledBacklog(root string, at time.Time) []Problem {
	tree, err := backlog.LoadTree(root)
	if err != nil {
		return nil
	}
	var problems []Problem
	open := 0
	for _, group := range tree.Groups {
		if group.File.Status != "REFINEMENT" {
			continue
		}
		for _, task := range group.File.Tasks {
			if !task.Done {
				open++
			}
		}
		rel, _ := filepath.Rel(root, group.Dir)
		touched, err := git.Run(root, "log", "-1", "--format=%ct", "--", filepath.ToSlash(rel))
		seconds, parseErr := strconv.ParseInt(strings.TrimSpace(touched), 10, 64)
		if err != nil || parseErr != nil {
			continue
		}
		if age := at.Sub(time.Unix(seconds, 0)); age > stallAge {
			problems = append(problems, Problem{"backlog", filepath.ToSlash(rel),
				fmt.Sprintf("a REFINEMENT group untouched for %d days; promote it to READY with real proofs, or delete it", int(age.Hours()/24))})
		}
	}
	if open > stallTasks {
		problems = append(problems, Problem{"backlog", filepath.ToSlash(backlog.GroupFilesDir),
			fmt.Sprintf("%d open REFINEMENT tasks; the line runs none of them, so triage them down to %d", open, stallTasks)})
	}
	return problems
}
