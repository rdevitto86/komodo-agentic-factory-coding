package backlog

import (
	"os"
	"path/filepath"
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
