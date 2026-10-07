package backlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindLooksAtTheRootThenDocs proves Find locates a legacy backlog file at root, then under docs/,
// the root's own file winning when both exist.
func TestFindLooksAtTheRootThenDocs(t *testing.T) {
	root := t.TempDir()
	if _, err := Find(root); err == nil {
		t.Fatal("Find named a legacy backlog file that does not exist")
	}
	docs := filepath.Join(root, "docs", LegacyName)
	if err := os.MkdirAll(filepath.Dir(docs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docs, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if found, err := Find(root); err != nil || found != docs {
		t.Fatalf("found = %q, err = %v; docs/%s is the fallback", found, err, LegacyName)
	}
	top := filepath.Join(root, LegacyName)
	if err := os.WriteFile(top, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if found, err := Find(root); err != nil || found != top {
		t.Fatalf("found = %q, err = %v; the root's own file wins", found, err)
	}
	loaded, err := Load(top)
	if err != nil || len(loaded.Tasks()) != 2 {
		t.Fatalf("loaded %d task(s), err = %v", len(loaded.Tasks()), err)
	}
	if _, err := Load(filepath.Join(root, "missing.md")); err == nil {
		t.Fatal("Load read a file that does not exist")
	}
}

// TestParseKeepsAGoalLineWithNoShipsAsClause proves a goal line with no inline version still
// reaches the epic's title, instead of being dropped by the lookahead.
func TestParseKeepsAGoalLineWithNoShipsAsClause(t *testing.T) {
	text := "## [EPIC-09] Nine\n*Goal: ship the thing.*\n\n### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n"
	epic, ok := Parse(text).Epic("EPIC-09")
	if !ok {
		t.Fatal("EPIC-09 not parsed")
	}
	if !strings.Contains(epic.Title, "ship the thing") {
		t.Fatalf("title = %q, want it to carry the goal line", epic.Title)
	}
}
