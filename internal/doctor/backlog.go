package doctor

import (
	"fmt"
	"os"
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
// tasks, or a REFINEMENT group no commit has touched in stallAge.
func checkStalledBacklog(root string, at time.Time) []Problem {
	paths, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(backlog.GroupFilesDir), "*.md"))
	var problems []Problem
	open := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		file := backlog.ParseGroupFile(string(data))
		if file.Status != "REFINEMENT" {
			continue
		}
		for _, task := range file.Tasks {
			if !task.Done {
				open++
			}
		}
		rel, _ := filepath.Rel(root, path)
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
