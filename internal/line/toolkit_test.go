package line

import (
	"os"
	"path/filepath"
	"testing"
)

const bareBacklog = "### [TG-90.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-90.1.1] Build the thing [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - go test ./a/...\n```\n"

// bareRepo builds a repo holding only a BACKLOG.md, no komodo/ directory of its own.
func bareRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(bareBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
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
