package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
)

const migrateSample = "## [EPIC-01] Sample epic\nShips as `1.0.0`.\n\n" +
	"### [TG-01.1] First group\n```yaml\ntype: feat\nversion: 1.0.0\ndepends_on: []\n```\n\n" +
	"#### [TSK-01.1.1] Do a thing [P: H] [DONE]\n```yaml\nfiles: [a.go]\ndone_when: [go test ./a/...]\ncontext: [\"docs/prd.md#12-numbered-heading\"]\n```\n\n" +
	"#### [TSK-01.1.2] Do another thing [P: M] [READY]\n```yaml\nfiles: [b.go]\ndone_when: [go test ./b/...]\n```\n\n" +
	"### [TG-01.2] Second group, a later version\n```yaml\ntype: fix\nversion: 1.1.0\ndepends_on: []\n```\n\n" +
	"#### [TSK-01.2.1] Fix a thing [P: C] [READY]\n```yaml\nfiles: [c.go]\ndone_when: [go test ./c/...]\n```\n"

// writeSampleBacklog writes migrateSample as BACKLOG.md at root, and the docs/prd.md it references.
func writeSampleBacklog(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(migrateSample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	prd := "# PRD\n\n## 1.2 Numbered heading\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "prd.md"), []byte(prd), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunMigrateWritesOneGroupFolderPerGroup proves migrate converts every BACKLOG.md group into its
// own folder under its epic's, the version landing in the epic index file and each task in its own file.
func TestRunMigrateWritesOneGroupFolderPerGroup(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	runMigrate(root, nil)

	group, found, err := backlog.Locate(root, "TG-01.1")
	if err != nil || !found {
		t.Fatalf("first group folder: found %v, %v", found, err)
	}
	if len(group.Problems) != 0 || len(group.File.Problems) != 0 {
		t.Fatalf("problems = %v %v", group.Problems, group.File.Problems)
	}
	if group.File.Version != "1.0.0" || group.File.EpicID != "EPIC-01" || group.Epic.Version != "1.0.0" {
		t.Fatalf("version/epic = %q/%q, epic %+v", group.File.Version, group.File.EpicID, group.Epic)
	}
	if len(group.File.Tasks) != 2 || !group.File.Tasks[0].Done || group.File.Tasks[1].Done {
		t.Fatalf("tasks = %+v", group.File.Tasks)
	}
	if len(group.TaskPaths) != 2 || filepath.Base(group.TaskPaths["TSK-01.1.2"]) != "tsk-01.1.2.md" {
		t.Fatalf("task files = %v, want one per task", group.TaskPaths)
	}
	header, err := os.ReadFile(group.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(header), "version:") || strings.Contains(string(header), "epic:") {
		t.Fatalf("TG.md repeats what EPIC.md holds: %s", header)
	}

	// BACKLOG.md stays until a human removes it.
	if _, err := os.Stat(filepath.Join(root, "BACKLOG.md")); err != nil {
		t.Fatalf("BACKLOG.md removed: %v", err)
	}
}

// TestRunMigrateSkipsAGroupWhoseEveryTaskIsDone proves a fully done group stays history in
// CHANGELOG.md and git, migrate never opening a file for it.
func TestRunMigrateSkipsAGroupWhoseEveryTaskIsDone(t *testing.T) {
	root := t.TempDir()
	sample := "## [EPIC-03] Sample epic\nShips as `1.0.0`.\n\n" +
		"### [TG-03.1] A closed group\n```yaml\ntype: feat\nversion: 1.0.0\ndepends_on: []\n```\n\n" +
		"#### [TSK-03.1.1] Done already [P: H] [DONE]\n```yaml\nfiles: [a.go]\ndone_when: [go test ./a/...]\n```\n"
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	runMigrate(root, nil)
	if _, err := os.Stat(backlog.GroupDirPath(root, "TG-03.1")); err == nil {
		t.Fatal("migrate opened a folder for a group whose every task is already done")
	}
}

// TestRunMigrateCarriesModeBaseTierAndFacets proves migrate keeps a group's mode and base, and a
// task's tier and facets, in the group file it writes.
func TestRunMigrateCarriesModeBaseTierAndFacets(t *testing.T) {
	root := t.TempDir()
	sample := "## [EPIC-02] Sample epic\nShips as `1.0.0`.\n\n" +
		"### [TG-02.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\nmode: single\nbase: feat/1.0.0\ndepends_on: []\n```\n\n" +
		"#### [TSK-02.1.1] Do a thing [P: H] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [go test ./a/...]\n" +
		"tier: heavy\nfacets: [go, docs]\n```\n"
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	runMigrate(root, nil)
	group, found, err := backlog.Locate(root, "TG-02.1")
	if err != nil || !found {
		t.Fatalf("group folder: found %v, %v", found, err)
	}
	file := group.File
	if file.Mode != "single" || file.Base != "feat/1.0.0" {
		t.Fatalf("mode/base = %q/%q", file.Mode, file.Base)
	}
	if len(file.Tasks) != 1 || file.Tasks[0].Tier != "heavy" {
		t.Fatalf("tasks = %+v", file.Tasks)
	}
	if got := file.Tasks[0].Facets; len(got) != 2 || got[0] != "go" || got[1] != "docs" {
		t.Fatalf("facets = %v", got)
	}
}

// TestRunMigrateSkipsAGroupWhoseVersionDiffersFromItsEpics proves a group at another version than its
// epic folder is named and left in the source.
func TestRunMigrateSkipsAGroupWhoseVersionDiffersFromItsEpics(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	out := captureStdout(t, func() { runMigrate(root, nil) })
	if !strings.Contains(out, "skip TG-01.2: version 1.1.0 differs from EPIC-01's 1.0.0") {
		t.Fatalf("migrate never named the group it could not place: %s", out)
	}
	if _, err := os.Stat(backlog.GroupDirPath(root, "TG-01.2")); err == nil {
		t.Fatal("migrate wrote TG-01.2 under an epic at another version")
	}
	if !strings.Contains(out, "1 group folder(s) written") {
		t.Fatalf("count = %s", out)
	}
	problems, _, _, err := groupFileLintProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("lint problems = %v", problems)
	}
}

// TestRunMigrateKeepsAGithubStyleAnchorOnANumberedHeading proves a context anchor migrates intact
// and still resolves against its target file's numbered heading.
func TestRunMigrateKeepsAGithubStyleAnchorOnANumberedHeading(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	runMigrate(root, nil)

	data := groupText(t, root, "TG-01.1")
	if !strings.Contains(data, "docs/prd.md#12-numbered-heading") {
		t.Fatalf("context anchor missing: %s", data)
	}
	problems, _, _, err := groupFileLintProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		if strings.Contains(problem, "names no heading") {
			t.Fatalf("lint rejects the migrated anchor: %v", problems)
		}
	}
}

// TestRunMigrateDryRunWritesNothing proves --dry-run only prints the folders it would write.
func TestRunMigrateDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	out := captureStdout(t, func() { runMigrate(root, []string{"--dry-run"}) })
	for _, want := range []string{"write docs/backlog/epic-01/EPIC.md\n", "write docs/backlog/epic-01/tg-01.1\n", "1 group folder(s) would be written"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q: %s", want, out)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, groupFilesDir))
	if err == nil && len(entries) > 0 {
		t.Fatalf("dry-run wrote files: %v", entries)
	}
}

const foreignTODO = "# Refunds\n\n- [ ] Add a partial refund endpoint\n- [x] Log every refund\n" +
	"A line neither a heading nor a bullet.\n"

// TestRunMigrateImportsATODOFile proves a repo with no BACKLOG.md but a TODO.md imports it into a
// REFINEMENT group file, printing the line Import could not place.
func TestRunMigrateImportsATODOFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "TODO.md"), []byte(foreignTODO), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() { runMigrate(root, nil) })
	if !strings.Contains(out, "could not place") {
		t.Fatalf("migrate printed no skipped line: %s", out)
	}
	tree, err := backlog.LoadTree(root)
	if err != nil || len(tree.Epics) != 1 || len(tree.Groups) != 1 {
		t.Fatalf("tree = %d epic(s), %d group(s), err = %v, want one of each", len(tree.Epics), len(tree.Groups), err)
	}
	file := tree.Groups[0].File
	if len(file.Problems) != 0 || len(tree.Problems) != 0 {
		t.Fatalf("problems = %v %v", file.Problems, tree.Problems)
	}
	if file.Status != "REFINEMENT" || len(file.Tasks) != 2 || file.Tasks[0].Done || !file.Tasks[1].Done {
		t.Fatalf("file = %+v", file)
	}
	if _, err := os.Stat(filepath.Join(root, "TODO.md")); err != nil {
		t.Fatal("TODO.md must stay until a human removes it")
	}
	problems, _, _, err := groupFileLintProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("lint problems = %v", problems)
	}
}

// TestRunMigrateImportsAForeignBacklogWithNoTGHeadings proves a BACKLOG.md that names no
// komodo-grammar group imports through Import instead of writing nothing.
func TestRunMigrateImportsAForeignBacklogWithNoTGHeadings(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(foreignTODO), 0o644); err != nil {
		t.Fatal(err)
	}
	runMigrate(root, nil)
	tree, err := backlog.LoadTree(root)
	if err != nil || len(tree.Groups) != 1 {
		t.Fatalf("tree = %d group(s), err = %v, want one group folder", len(tree.Groups), err)
	}
}

// TestRunMigrateRefusesWithNeitherBacklogNorTODO proves migrate fails clearly instead of writing nothing silently.
func TestRunMigrateRefusesWithNeitherBacklogNorTODO(t *testing.T) {
	if _, _, _, err := migrateSource(t.TempDir()); err == nil {
		t.Fatal("want an error with no BACKLOG.md or TODO.md")
	}
}

// TestRunMigrateSkipsAGroupFolderThatAlreadyExists proves migrate never overwrites a hand-edited the group index file
// or the epic index file.
func TestRunMigrateSkipsAGroupFolderThatAlreadyExists(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	dest := filepath.Join(backlog.GroupDirPath(root, "TG-01.1"), backlog.GroupFileName)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("hand-edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	epicPath := filepath.Join(backlog.EpicDir(root, "EPIC-01"), backlog.EpicFileName)
	if err := os.WriteFile(epicPath, []byte("## [EPIC-01] Mine [READY]\n\n```yaml\nversion: 1.0.0\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() { runMigrate(root, nil) })
	if !strings.Contains(out, "skip docs/backlog/epic-01/tg-01.1: already exists") {
		t.Fatalf("migrate never said it skipped the folder: %s", out)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hand-edited" {
		t.Fatalf("migrate overwrote an existing TG.md: %s", data)
	}
	if epic, _ := os.ReadFile(epicPath); !strings.Contains(string(epic), "Mine") {
		t.Fatalf("migrate overwrote an existing EPIC.md: %s", epic)
	}
}

// flatGroupText is one group file in the layout before the tree: version and epic in its own yaml.
const flatGroupText = "## [TG-07.1] A flat group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-07\ndepends_on: []\n```\n\n" +
	"- [ ] **TSK-07.1.1** A task\n  - files: `a.go`\n  - done_when: `go test ./a/...`\n\n" +
	"- [x] **TSK-07.1.2** A done task\n  - files: `b.go`\n  - done_when: `go test ./b/...`\n"

// TestRunMigrateDryRunOnFlatFilesPrintsTheFolders proves a flat docs/backlog is the source migrate
// prefers, and a dry run names each folder without writing one.
func TestRunMigrateDryRunOnFlatFilesPrintsTheFolders(t *testing.T) {
	root := t.TempDir()
	writeFlatGroupFile(t, root, "TG-07.1-a-flat-group.md", flatGroupText)
	writeFlatGroupFile(t, root, "TG-07.2-another.md",
		"## [TG-07.2] Another [P: M] [REFINEMENT]\n\n```yaml\ntype: fix\nversion: 1.0.0\nepic: EPIC-07\ndepends_on: []\n```\n")
	out := captureStdout(t, func() { runMigrate(root, []string{"--dry-run"}) })
	for _, want := range []string{
		"write docs/backlog/epic-07/EPIC.md\n", "write docs/backlog/epic-07/tg-07.1\n", "write docs/backlog/epic-07/tg-07.2\n",
		"2 group folder(s) would be written",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q: %s", want, out)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, groupFilesDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("dry-run wrote a folder: %s", entry.Name())
		}
	}
}

// TestRunMigrateMovesFlatFilesIntoTheTreeAndLeavesThem proves a real run writes the tree, lints clean,
// and leaves the flat files for a person to remove.
func TestRunMigrateMovesFlatFilesIntoTheTreeAndLeavesThem(t *testing.T) {
	root := t.TempDir()
	writeFlatGroupFile(t, root, "TG-07.1-a-flat-group.md", flatGroupText)
	out := captureStdout(t, func() { runMigrate(root, nil) })
	if !strings.Contains(out, "1 group folder(s) written") || !strings.Contains(out, "stays until it is removed by hand") {
		t.Fatalf("migrate output = %s", out)
	}
	group, found, err := backlog.Locate(root, "TG-07.1")
	if err != nil || !found {
		t.Fatalf("group folder: found %v, %v", found, err)
	}
	if group.Epic.ID != "EPIC-07" || group.File.Version != "1.0.0" || group.File.Status != "READY" {
		t.Fatalf("group = %+v under %+v", group.File, group.Epic)
	}
	if len(group.File.Tasks) != 2 || group.File.Tasks[0].Done || !group.File.Tasks[1].Done {
		t.Fatalf("tasks = %+v", group.File.Tasks)
	}
	for _, name := range []string{"tsk-07.1.1.md", "tsk-07.1.2.md"} {
		if _, err := os.Stat(filepath.Join(group.Dir, name)); err != nil {
			t.Fatalf("task file %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, groupFilesDir, "TG-07.1-a-flat-group.md")); err != nil {
		t.Fatalf("migrate removed the flat file: %v", err)
	}
	problems, _, _, err := groupFileLintProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("lint problems = %v", problems)
	}
	notes := groupFileNotes(root)
	found = false
	for _, note := range notes {
		if strings.Contains(note, "TG-07.1-a-flat-group.md") {
			found = true
		}
	}
	if !found {
		t.Fatalf("notes = %v, want the flat file noted for removal", notes)
	}
	again := captureStdout(t, func() { runMigrate(root, nil) })
	if !strings.Contains(again, "skip docs/backlog/epic-07/tg-07.1: already exists") {
		t.Fatalf("a second migrate rewrote the folder: %s", again)
	}
}
