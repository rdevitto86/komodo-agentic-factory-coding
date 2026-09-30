package line

import (
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
)

var bareGroup = backlog.GroupFile{
	ID: "TG-90.1", Title: "A group", Priority: "C", Status: "READY", Type: "feat", Version: "2.0.0",
	Tasks: []backlog.GroupTask{
		{ID: "TSK-90.1.1", Title: "Build the thing", Files: []string{"a/one.go"}, Checks: []string{"go test ./a/..."}},
	},
}

// bareRepo builds a repo holding only a group file, no komodo/ directory of its own.
func bareRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogtest.Seed(t, root, bareGroup)
	return root
}

// TestNextBriefAndStepRunOnARepoWithNoKomodoDir proves the line runs from the embedded toolkit.
func TestNextBriefAndStepRunOnARepoWithNoKomodoDir(t *testing.T) {
	root := bareRepo(t)

	plan, err := PlanForStation(root, "")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if plan == nil || plan.Group != "TG-90.1" {
		t.Fatalf("plan = %+v", plan)
	}
	found := false
	for _, role := range plan.Roles {
		found = found || role.Name == "builder"
	}
	if !found {
		t.Fatalf("plan.Roles = %+v, want the embedded builder role", plan.Roles)
	}

	brief, err := BuildBrief(root, root, "TSK-90.1.1", "builder", "")
	if err != nil {
		t.Fatalf("brief: %v", err)
	}
	if brief.Text == "" {
		t.Fatal("brief text is empty")
	}

	action, err := Step(root, "")
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if action == nil || action.Action == "" {
		t.Fatalf("action = %+v", action)
	}
}
