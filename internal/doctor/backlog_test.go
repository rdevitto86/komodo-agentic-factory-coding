package doctor

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
)

// refinementGroup is one REFINEMENT group of an epic id with tasks open tasks.
func refinementGroup(id string, tasks int) backlog.GroupFile {
	group := backlog.GroupFile{ID: id, Title: "A group", Priority: "M", Status: "REFINEMENT", Type: "fix", Version: "1.0.0", EpicID: "EPIC-70"}
	for index := 1; index <= tasks; index++ {
		group.Tasks = append(group.Tasks, backlog.GroupTask{
			ID: fmt.Sprintf("TSK-%s.%d", strings.TrimPrefix(id, "TG-"), index), Title: fmt.Sprintf("Task %d", index),
		})
	}
	return group
}

func TestCheckStalledBacklogFlagsAPileAndAnUntouchedGroup(t *testing.T) {
	root := gitRepo(t)
	backlogtest.Seed(t, root, refinementGroup("TG-70.1", 12), refinementGroup("TG-70.2", 12))
	commitAll(t, root, "plan")
	if got := checkStalledBacklog(root, time.Now()); len(got) != 0 {
		t.Fatalf("problems = %+v; 24 fresh REFINEMENT tasks are a plan, not a pile", got)
	}
	for index := 3; index <= 6; index++ {
		id := fmt.Sprintf("TG-70.%d", index)
		backlogtest.Seed(t, root, refinementGroup(id, 12))
	}
	commitAll(t, root, "more")
	got := checkStalledBacklog(root, time.Now().Add(15*24*time.Hour))
	var pile, stale int
	for _, problem := range got {
		switch {
		case strings.Contains(problem.Detail, "open REFINEMENT tasks"):
			pile++
		case strings.Contains(problem.Detail, "untouched for 15 days"):
			stale++
		}
	}
	if pile != 1 || stale != 6 {
		t.Fatalf("problems = %+v; want one pile of 72 tasks and six untouched groups", got)
	}
}
