package backlog

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedEpic writes an epic index file under root for the given epic and version.
func seedEpic(t *testing.T, root, epicID, version string) {
	t.Helper()
	writeFile(t, filepath.Join(EpicDir(root, epicID), EpicFileName),
		"## ["+epicID+"] Epic "+epicID+" [READY]\n\n```yaml\nversion: "+version+"\ntype: feat\n```\n\nThe goal.\n")
}

// seedGroup writes a group index file and one file per task text under the group's folder.
func seedGroup(t *testing.T, root, groupID, header string, tasks map[string]string) {
	t.Helper()
	dir := GroupDirPath(root, groupID)
	writeFile(t, filepath.Join(dir, GroupFileName), header)
	for id, text := range tasks {
		writeFile(t, filepath.Join(dir, TaskFileName(id)), text)
	}
}

func TestLoadRootReadsTheTreeWhenPresent(t *testing.T) {
	root := t.TempDir()
	seedEpic(t, root, "EPIC-01", "1.0.0")
	seedGroup(t, root, "TG-01.1", "## [TG-01.1] Example group [P: H] [READY]\n\n```yaml\ntype: feat\ndepends_on: []\n```\n",
		map[string]string{"TSK-01.1.1": "- [ ] **TSK-01.1.1** Do the thing\n  - files: `a.go`\n  - checks: `go test ./...`\n"})
	parsed, err := LoadRoot(root)
	if err != nil {
		t.Fatalf("LoadRoot: %v", err)
	}
	if len(parsed.Groups) != 1 || parsed.Groups[0].ID != "TG-01.1" {
		t.Fatalf("groups = %+v", parsed.Groups)
	}
	if got := parsed.Groups[0].Version(); got != "1.0.0" {
		t.Fatalf("version = %q, want the epic's 1.0.0 inherited", got)
	}
	if parsed.Groups[0].EpicID != "EPIC-01" {
		t.Fatalf("epic = %q, want EPIC-01 from the folder", parsed.Groups[0].EpicID)
	}
	epic, ok := parsed.Epic("EPIC-01")
	if !ok || epic.Version() != "1.0.0" || epic.Title != "Epic EPIC-01" || epic.Goal != "The goal." {
		t.Fatalf("epic = %+v", epic)
	}
	task, ok := parsed.Task("TSK-01.1.1")
	if !ok {
		t.Fatal("want TSK-01.1.1")
	}
	if task.Status != "READY" || task.Priority != "H" {
		t.Fatalf("task = %+v", task)
	}
	if got := task.Files(); len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("files = %v", got)
	}
	if got := task.DoneWhen(); len(got) != 1 || got[0] != "go test ./..." {
		t.Fatalf("done_when = %v", got)
	}
}

// TestATasksOwnStatusAndPriorityOverrideItsGroups proves a task's status and priority lines win over
// the group heading's, so a BLOCKED task in a READY group never reaches the harness's queue.
func TestATasksOwnStatusAndPriorityOverrideItsGroups(t *testing.T) {
	root := t.TempDir()
	seedEpic(t, root, "EPIC-01", "1.0.0")
	seedGroup(t, root, "TG-01.1", "## [TG-01.1] Example group [P: M] [READY]\n\n```yaml\ntype: feat\ndepends_on: []\n```\n",
		map[string]string{
			"TSK-01.1.1": "- [ ] **TSK-01.1.1** Wait for a person\n  - files: `a.go`\n  - priority: H\n  - status: BLOCKED\n",
			"TSK-01.1.2": "- [ ] **TSK-01.1.2** Do the thing\n  - files: `b.go`\n",
		})
	parsed, err := LoadRoot(root)
	if err != nil {
		t.Fatalf("LoadRoot: %v", err)
	}
	blocked, _ := parsed.Task("TSK-01.1.1")
	if blocked.Status != "BLOCKED" || blocked.Priority != "H" || blocked.Ready() {
		t.Fatalf("blocked = %+v; its own status and priority must win", blocked)
	}
	plain, _ := parsed.Task("TSK-01.1.2")
	if plain.Status != "READY" || plain.Priority != "M" {
		t.Fatalf("plain = %+v; a task with neither line inherits its group's", plain)
	}
}

func TestLoadRootCarriesModeBaseTierAndFacets(t *testing.T) {
	root := t.TempDir()
	seedEpic(t, root, "EPIC-01", "1.0.0")
	seedGroup(t, root, "TG-01.1", "## [TG-01.1] Example group [P: H] [READY]\n\n```yaml\ntype: feat\nmode: single\nbase: feat/1.0.0\ndepends_on: []\n```\n",
		map[string]string{"TSK-01.1.1": "- [ ] **TSK-01.1.1** Do the thing\n  - files: `a.go`\n  - checks: `go test ./...`\n  - tier: heavy\n  - facets: go, docs\n"})
	parsed, err := LoadRoot(root)
	if err != nil {
		t.Fatalf("LoadRoot: %v", err)
	}
	group := parsed.Groups[0]
	if group.Mode() != "single" {
		t.Fatalf("mode = %q, want single", group.Mode())
	}
	if group.Base() != "feat/1.0.0" {
		t.Fatalf("base = %q, want feat/1.0.0", group.Base())
	}
	task, ok := parsed.Task("TSK-01.1.1")
	if !ok {
		t.Fatal("want TSK-01.1.1")
	}
	if task.Tier() != "heavy" {
		t.Fatalf("tier = %q, want heavy", task.Tier())
	}
	if got := task.Facets(); len(got) != 2 || got[0] != "go" || got[1] != "docs" {
		t.Fatalf("facets = %v", got)
	}
}

// TestLoadRootIgnoresFlatAndLegacyFiles proves LoadRoot reads only the tree: a flat group file or a
// legacy backlog file is listed for migrate, never loaded as a group.
func TestLoadRootIgnoresFlatAndLegacyFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, LegacyName),
		"## [EPIC-01] Phase 0\n\n### [TG-01.1] Legacy group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n")
	writeFile(t, filepath.Join(root, GroupFilesDir, "TG-03.1-flat.md"),
		"## [TG-03.1] Flat group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-03\ndepends_on: []\n```\n")
	seedEpic(t, root, "EPIC-02", "1.0.0")
	seedGroup(t, root, "TG-02.1", "## [TG-02.1] Tree group [P: H] [READY]\n\n```yaml\ntype: feat\ndepends_on: []\n```\n",
		map[string]string{"TSK-02.1.1": "- [ ] **TSK-02.1.1** A task\n  - files: `a.go`\n"})
	parsed, err := LoadRoot(root)
	if err != nil {
		t.Fatalf("LoadRoot: %v", err)
	}
	if len(parsed.Groups) != 1 || parsed.Groups[0].ID != "TG-02.1" {
		t.Fatalf("groups = %+v, want only the tree's group", parsed.Groups)
	}
	tree, err := LoadTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Flat) != 1 || filepath.Base(tree.Flat[0]) != "TG-03.1-flat.md" {
		t.Fatalf("flat = %v, want the one flat file listed for migrate", tree.Flat)
	}
}

// TestLoadRootIsEmptyWithNoGroupFiles proves an empty repo is an empty backlog, not an error.
func TestLoadRootIsEmptyWithNoGroupFiles(t *testing.T) {
	root := t.TempDir()
	parsed, err := LoadRoot(root)
	if err != nil {
		t.Fatalf("LoadRoot: %v", err)
	}
	if len(parsed.Groups) != 0 {
		t.Fatalf("groups = %+v, want none", parsed.Groups)
	}
}

func TestExistsReportsGroupFolders(t *testing.T) {
	root := t.TempDir()
	if Exists(root) {
		t.Fatal("want false with no groups")
	}
	seedEpic(t, root, "EPIC-03", "1.0.0")
	if Exists(root) {
		t.Fatal("want false with an epic but no group")
	}
	seedGroup(t, root, "TG-03.1", "## [TG-03.1] Group [P: H] [REFINEMENT]\n\n```yaml\ntype: feat\ndepends_on: []\n```\n", nil)
	if !Exists(root) {
		t.Fatal("want true with a group folder")
	}
}
