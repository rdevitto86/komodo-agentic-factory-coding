package backlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoadTreeOrdersNumericallyAndInheritsFromTheEpic proves tg-15.2 precedes tg-15.10, task files
// sort by number, and every group carries its epic's version and id.
func TestLoadTreeOrdersNumericallyAndInheritsFromTheEpic(t *testing.T) {
	root := t.TempDir()
	seedEpic(t, root, "EPIC-15", "1.0.0-beta.6")
	for _, id := range []string{"TG-15.10", "TG-15.2", "TG-15.1"} {
		seedGroup(t, root, id, "## ["+id+"] Group "+id+" [P: H] [READY]\n\n```yaml\ntype: fix\ndepends_on: []\n```\n",
			map[string]string{
				"TSK-" + strings.TrimPrefix(id, "TG-") + ".10": "- [ ] **TSK-" + strings.TrimPrefix(id, "TG-") + ".10** Ten\n  - files: `j.go`\n",
				"TSK-" + strings.TrimPrefix(id, "TG-") + ".2":  "- [ ] **TSK-" + strings.TrimPrefix(id, "TG-") + ".2** Two\n  - files: `b.go`\n",
			})
	}
	tree, err := LoadTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Problems) != 0 {
		t.Fatalf("problems = %v", tree.Problems)
	}
	var ids []string
	for _, group := range tree.Groups {
		ids = append(ids, group.File.ID)
	}
	if strings.Join(ids, " ") != "TG-15.1 TG-15.2 TG-15.10" {
		t.Fatalf("order = %v, want numeric", ids)
	}
	first := tree.Groups[0].File
	if first.Version != "1.0.0-beta.6" || first.EpicID != "EPIC-15" {
		t.Fatalf("group = %+v, want the epic's version and id", first)
	}
	if first.Tasks[0].ID != "TSK-15.1.2" || first.Tasks[1].ID != "TSK-15.1.10" {
		t.Fatalf("tasks = %+v, want numeric order", first.Tasks)
	}
}

// TestLoadTreeNamesEveryStructuralProblem proves a misnamed folder, a version in the group index file, a task under
// the wrong group and a file holding two tasks each read as one problem.
func TestLoadTreeNamesEveryStructuralProblem(t *testing.T) {
	root := t.TempDir()
	seedEpic(t, root, "EPIC-02", "1.0.0")
	dir := filepath.Join(EpicDir(root, "EPIC-02"), "tg-02.1")
	writeFile(t, filepath.Join(dir, GroupFileName), "## [TG-02.9] Misnamed [P: H] [READY]\n\n```yaml\ntype: fix\nversion: 1.0.0\nepic: EPIC-03\ndepends_on: []\n```\n")
	writeFile(t, filepath.Join(dir, "tsk-02.9.1.md"), "- [ ] **TSK-02.9.1** One\n  - files: `a.go`\n- [ ] **TSK-02.9.2** Two\n  - files: `b.go`\n")
	writeFile(t, filepath.Join(dir, "tsk-02.9.3.md"), "- [ ] **TSK-07.1.1** Stray\n  - files: `c.go`\n")
	writeFile(t, filepath.Join(EpicDir(root, "EPIC-04"), EpicFileName), "## [EPIC-05] Wrong folder [READY]\n\n```yaml\nversion: 1.0.0\n```\n")
	tree, err := LoadTree(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(tree.Problems, "\n")
	for _, want := range []string{
		"folder tg-02.1 holds TG-02.9; name the folder tg-02.9",
		"version belongs in the epic's EPIC.md, not TG.md",
		"epic EPIC-03 disagrees with the folder's epic EPIC-02",
		"tsk-02.9.1.md: holds more than one task",
		"tsk-02.9.3.md holds TSK-07.1.1; name the file tsk-07.1.1.md",
		"TSK-07.1.1 sits under TG-02.9; its number says TG-07.1",
		"folder epic-04 holds EPIC-05; name the folder epic-05",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems lack %q:\n%s", want, joined)
		}
	}
}

// TestLintTreeCapsFilesPerGroupAndGroupsPerEpic proves the two caps and that a ticked task's files
// never count toward the group's.
func TestLintTreeCapsFilesPerGroupAndGroupsPerEpic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(EpicDir(root, "EPIC-01"), EpicFileName), "## [EPIC-01] Small [READY]\n\n```yaml\nversion: 1.0.0\ngroups_max: 1\n```\n")
	var open, done strings.Builder
	open.WriteString("- [ ] **TSK-01.1.1** Wide\n  - files: ")
	done.WriteString("- [x] **TSK-01.2.1** Done wide\n  - files: ")
	for index := 0; index <= MaxGroupFiles; index++ {
		if index > 0 {
			open.WriteString(", ")
			done.WriteString(", ")
		}
		open.WriteString("`f" + strings.Repeat("x", index) + ".go`")
		done.WriteString("`f" + strings.Repeat("x", index) + ".go`")
	}
	seedGroup(t, root, "TG-01.1", "## [TG-01.1] Wide [P: H] [READY]\n\n```yaml\ntype: fix\ndepends_on: []\n```\n",
		map[string]string{"TSK-01.1.1": open.String() + "\n"})
	seedGroup(t, root, "TG-01.2", "## [TG-01.2] Done [P: H] [READY]\n\n```yaml\ntype: fix\ndepends_on: []\n```\n",
		map[string]string{"TSK-01.2.1": done.String() + "\n"})
	tree, err := LoadTree(root)
	if err != nil {
		t.Fatal(err)
	}
	problems := strings.Join(LintTree(tree), "\n")
	if !strings.Contains(problems, "TG-01.1: open tasks declare 21 files, over the cap of 20") {
		t.Errorf("problems lack the file cap:\n%s", problems)
	}
	if strings.Contains(problems, "TG-01.2:") {
		t.Errorf("a ticked task's files must not count:\n%s", problems)
	}
	if !strings.Contains(problems, "EPIC-01: 2 groups, over its groups_max of 1") {
		t.Errorf("problems lack the group cap:\n%s", problems)
	}
}

// TestWritersTargetTheRightFile proves a tick lands in the task's file, a note in the group index file, and an
// appended task in a new file with the next id.
func TestWritersTargetTheRightFile(t *testing.T) {
	root := t.TempDir()
	if _, err := WriteEpic(root, EpicFile{ID: "EPIC-07", Title: "Seven", Version: "1.2.0", Type: "feat", GroupsMax: 3, Goal: "Goal."}); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteEpic(root, EpicFile{ID: "EPIC-07", Title: "Again", Version: "1.2.0"}); err == nil {
		t.Fatal("a second WriteEpic must refuse")
	}
	file := GroupFile{ID: "TG-07.1", Title: "First", Priority: "H", Status: "READY", Type: "feat",
		Tasks: []GroupTask{{ID: "TSK-07.1.1", Title: "One", Files: []string{"a.go"}, Checks: []string{"go test ./..."}}}}
	if _, err := WriteGroup(root, file); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteGroup(root, GroupFile{ID: "TG-08.1", Title: "No epic", Priority: "H", Status: "READY", Type: "feat"}); err == nil {
		t.Fatal("a group with no epic folder must refuse")
	}
	group, found, err := Locate(root, "TG-07.1")
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if group.Epic.Title != "Seven" || group.File.Version != "1.2.0" || group.Epic.GroupsMax != 3 {
		t.Fatalf("group = %+v", group.Epic)
	}
	if err := group.WriteTaskStatus("TSK-07.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(group.TaskPaths["TSK-07.1.1"])
	if !strings.HasPrefix(string(data), "- [x] **TSK-07.1.1**") {
		t.Fatalf("task file = %q, want ticked", data)
	}
	header, _ := os.ReadFile(group.Path)
	if strings.Contains(string(header), "TSK-07.1.1") {
		t.Fatalf("TG.md must hold no task:\n%s", header)
	}
	note := BlockerNote{At: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC), Run: "r1", State: "Building", Needs: "a decision"}
	if err := group.WriteNote(note); err != nil {
		t.Fatal(err)
	}
	header, _ = os.ReadFile(group.Path)
	if !strings.Contains(string(header), "> **Blocked** 2026-10-04 12:00") || !strings.Contains(string(header), "[BLOCKED]") {
		t.Fatalf("TG.md = %s, want the note and BLOCKED", header)
	}
	had, err := group.RemoveNote()
	if err != nil || !had {
		t.Fatalf("had = %v, err = %v", had, err)
	}
	if err := group.SetStatus("READY"); err != nil {
		t.Fatal(err)
	}
	id, path, err := group.AppendTask(GroupTask{Title: "Two", Files: []string{"b.go"}})
	if err != nil || id != "TSK-07.1.2" || filepath.Base(path) != "tsk-07.1.2.md" {
		t.Fatalf("id = %q, path = %q, err = %v", id, path, err)
	}
	group, _, _ = Locate(root, "TG-07.1")
	if len(group.File.Tasks) != 2 || group.File.Status != "READY" || len(group.Paths()) != 3 {
		t.Fatalf("group = %+v, paths = %v", group.File, group.Paths())
	}
	_, taskPath, found, err := LocateTask(root, "TSK-07.1.2")
	if err != nil || !found || taskPath != path {
		t.Fatalf("LocateTask = %q, %v, %v", taskPath, found, err)
	}
}

// TestEpicFileRoundTrips proves RenderEpicFile and ParseEpicFile agree, groups_max included.
func TestEpicFileRoundTrips(t *testing.T) {
	epic := EpicFile{ID: "EPIC-15", Title: "The harness runs beta.6", Status: "READY", Version: "1.0.0-beta.6", Type: "fix", GroupsMax: 14, Goal: "One paragraph.\n\nA second."}
	parsed := ParseEpicFile(RenderEpicFile(epic))
	if len(parsed.Problems) != 0 {
		t.Fatalf("problems = %v", parsed.Problems)
	}
	if parsed.ID != epic.ID || parsed.Title != epic.Title || parsed.Status != epic.Status || parsed.Version != epic.Version ||
		parsed.Type != epic.Type || parsed.GroupsMax != 14 || parsed.Goal != epic.Goal {
		t.Fatalf("parsed = %+v, want %+v", parsed, epic)
	}
	plain := ParseEpicFile("## [EPIC-01] Plain [READY]\n\n```yaml\nversion: 1.0.0\n```\n")
	if plain.GroupsMax != DefaultGroupsMax {
		t.Fatalf("groups_max = %d, want the default %d", plain.GroupsMax, DefaultGroupsMax)
	}
	bad := ParseEpicFile("## [EPIC-01] Bad [READY]\n\n```yaml\nversion: 1.0.0\ngroups_max: zero\n```\n")
	if len(bad.Problems) != 1 || !strings.Contains(bad.Problems[0], "groups_max") {
		t.Fatalf("problems = %v", bad.Problems)
	}
}

// TestIDsMapToFolderNames proves the id to folder and file name mapping both ways.
func TestIDsMapToFolderNames(t *testing.T) {
	if EpicDirName("EPIC-15") != "epic-15" || GroupDirName("TG-15.3") != "tg-15.3" || TaskFileName("TSK-15.3.1") != "tsk-15.3.1.md" {
		t.Fatal("folder names must be the lower-cased ids")
	}
	if EpicIDOfGroup("TG-15.3") != "EPIC-15" || GroupIDOfTask("TSK-15.3.12") != "TG-15.3" {
		t.Fatal("ids must map by number")
	}
	if !numericLess("tg-15.2", "tg-15.10") || numericLess("tg-15.10", "tg-15.2") {
		t.Fatal("numeric order must hold")
	}
}

// TestEmptiedFoldersAreNotGroupsOrEpics proves a folder a ship emptied reads back as nothing.
func TestEmptiedFoldersAreNotGroupsOrEpics(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(EpicDir(root, "EPIC-03"), "tg-03.1"), 0o755); err != nil {
		t.Fatal(err)
	}
	tree, err := LoadTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Epics) != 0 || len(tree.Groups) != 0 || len(tree.Problems) != 0 || Exists(root) {
		t.Fatalf("tree = %+v, want nothing from empty folders", tree)
	}
}
