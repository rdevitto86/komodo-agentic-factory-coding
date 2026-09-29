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

// TestRunMigrateWritesOneGroupFilePerGroup proves migrate converts every BACKLOG.md group into its own file.
func TestRunMigrateWritesOneGroupFilePerGroup(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	runMigrate(root, nil)

	firstPath := filepath.Join(root, groupFilesDir, "TG-01.1-first-group.md")
	data, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("first group file: %v", err)
	}
	file := backlog.ParseGroupFile(string(data))
	if len(file.Problems) != 0 {
		t.Fatalf("problems = %v", file.Problems)
	}
	if file.Version != "1.0.0" || file.EpicID != "EPIC-01" {
		t.Fatalf("version/epic = %q/%q", file.Version, file.EpicID)
	}
	if len(file.Tasks) != 2 || !file.Tasks[0].Done || file.Tasks[1].Done {
		t.Fatalf("tasks = %+v", file.Tasks)
	}

	// BACKLOG.md stays until a human removes it.
	if _, err := os.Stat(filepath.Join(root, "BACKLOG.md")); err != nil {
		t.Fatalf("BACKLOG.md removed: %v", err)
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
	data, err := os.ReadFile(filepath.Join(root, groupFilesDir, "TG-02.1-a-group.md"))
	if err != nil {
		t.Fatalf("group file: %v", err)
	}
	file := backlog.ParseGroupFile(string(data))
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

// TestRunMigrateSplitsAnEpicSpanningSeveralVersions proves a group whose version differs from its
// epic's gets its own synthetic epic id, so the migrated group files never disagree on version.
func TestRunMigrateSplitsAnEpicSpanningSeveralVersions(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	runMigrate(root, nil)

	secondPath := filepath.Join(root, groupFilesDir, "TG-01.2-second-group-a-later-version.md")
	data, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("second group file: %v", err)
	}
	file := backlog.ParseGroupFile(string(data))
	if file.EpicID == "EPIC-01" {
		t.Fatalf("epic = %q, want a split id distinct from EPIC-01", file.EpicID)
	}

	problems, _, _, err := groupFileLintProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		if strings.Contains(problem, "split the epic per version") {
			t.Fatalf("lint still finds a version split: %v", problems)
		}
	}
}

// TestRunMigrateKeepsAGithubStyleAnchorOnANumberedHeading proves a context anchor migrates intact
// and still resolves against its target file's numbered heading.
func TestRunMigrateKeepsAGithubStyleAnchorOnANumberedHeading(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	runMigrate(root, nil)

	data, err := os.ReadFile(filepath.Join(root, groupFilesDir, "TG-01.1-first-group.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "docs/prd.md#12-numbered-heading") {
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

// TestRunMigrateDryRunWritesNothing proves --dry-run only prints the files it would write.
func TestRunMigrateDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	out := captureStdout(t, func() { runMigrate(root, []string{"--dry-run"}) })
	if !strings.Contains(out, "TG-01.1") {
		t.Fatalf("dry-run output missing TG-01.1: %s", out)
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
	entries, err := os.ReadDir(filepath.Join(root, groupFilesDir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %v, err = %v, want one group file", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(root, groupFilesDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	file := backlog.ParseGroupFile(string(data))
	if len(file.Problems) != 0 {
		t.Fatalf("problems = %v", file.Problems)
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
	entries, err := os.ReadDir(filepath.Join(root, groupFilesDir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %v, err = %v, want one group file", entries, err)
	}
}

// TestRunMigrateRefusesWithNeitherBacklogNorTODO proves migrate fails clearly instead of writing nothing silently.
func TestRunMigrateRefusesWithNeitherBacklogNorTODO(t *testing.T) {
	if _, _, _, err := migrateSource(t.TempDir()); err == nil {
		t.Fatal("want an error with no BACKLOG.md or TODO.md")
	}
}

// TestRunMigrateSkipsAGroupFileThatAlreadyExists proves migrate never overwrites a hand-edited group file.
func TestRunMigrateSkipsAGroupFileThatAlreadyExists(t *testing.T) {
	root := t.TempDir()
	writeSampleBacklog(t, root)
	dest := filepath.Join(root, groupFilesDir, "TG-01.1-first-group.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("hand-edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	runMigrate(root, nil)
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hand-edited" {
		t.Fatalf("migrate overwrote an existing group file: %s", data)
	}
}
