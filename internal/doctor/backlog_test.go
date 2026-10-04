package doctor

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// refinementGroup is one REFINEMENT group file with tasks open tasks.
func refinementGroup(id string, tasks int) string {
	var body strings.Builder
	fmt.Fprintf(&body, "## [%s] A group [P: M] [REFINEMENT]\n\n```yaml\ntype: fix\nversion: 1.0.0\nepic: EPIC-70\ndepends_on: []\n```\n\n", id)
	for index := 1; index <= tasks; index++ {
		fmt.Fprintf(&body, "- [ ] **TSK-%s.%d** Task %d\n", strings.TrimPrefix(id, "TG-"), index, index)
	}
	return body.String()
}

func TestCheckStalledBacklogFlagsAPileAndAnUntouchedGroup(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "docs/backlog/TG-70.1-a.md", refinementGroup("TG-70.1", 12))
	write(t, root, "docs/backlog/TG-70.2-b.md", refinementGroup("TG-70.2", 12))
	commitAll(t, root, "plan")
	if got := checkStalledBacklog(root, time.Now()); len(got) != 0 {
		t.Fatalf("problems = %+v; 24 fresh REFINEMENT tasks are a plan, not a pile", got)
	}
	for index := 3; index <= 6; index++ {
		id := fmt.Sprintf("TG-70.%d", index)
		write(t, root, "docs/backlog/"+id+"-x.md", refinementGroup(id, 12))
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
