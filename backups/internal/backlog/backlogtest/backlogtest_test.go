package backlogtest

import (
	"testing"

	"komodo/internal/backlog"
)

// legacyGroups is a minimal two-group legacy-grammar fixture spanning two epics.
const legacyGroups = "### [TG-90.1] A shipped group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-90.1.1] Do it [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n" +
	"\n### [TG-91.1] A pending group\n```yaml\ntype: feat\nversion: 3.0.0\n```\n\n" +
	"#### [TSK-91.1.1] Not done [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"true\"]\n```\n"

func TestSeedTextWritesATreeTheBacklogPackageCanLoad(t *testing.T) {
	root := t.TempDir()
	SeedText(t, root, legacyGroups)
	tree, err := backlog.LoadTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(tree.Groups))
	}
	var ids []string
	for _, group := range tree.Groups {
		ids = append(ids, group.File.ID)
	}
	if ids[0] != "TG-90.1" && ids[1] != "TG-90.1" {
		t.Fatalf("groups = %v, want TG-90.1 among them", ids)
	}
}

func TestSeedWritesAGroupUnderAnExplicitEpicID(t *testing.T) {
	root := t.TempDir()
	Seed(t, root, backlog.GroupFile{
		ID: "TG-1.1", EpicID: "EPIC-1", Title: "A group", Status: "READY", Type: "feat", Version: "1.0.0",
	})
	tree, err := backlog.LoadTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Groups) != 1 || tree.Groups[0].EpicDir == "" {
		t.Fatalf("tree = %+v, want one group filed under its epic", tree)
	}
}
