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

func TestLoadRootReadsGroupFilesWhenPresent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "backlog", "TG-01.1-example.md"),
		"## [TG-01.1] Example group [P: H] [READY]\n\n"+
			"```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-01\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-01.1.1** Do the thing\n  - files: `a.go`\n  - checks: `go test ./...`\n")
	parsed, err := LoadRoot(root)
	if err != nil {
		t.Fatalf("LoadRoot: %v", err)
	}
	if len(parsed.Groups) != 1 || parsed.Groups[0].ID != "TG-01.1" {
		t.Fatalf("groups = %+v", parsed.Groups)
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

func TestLoadRootFallsBackToBacklogMdWithNoGroupFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "BACKLOG.md"),
		"## [EPIC-01] Phase 0\n\n### [TG-01.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"+
			"#### [TSK-01.1.1] A task [P: H] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n")
	parsed, err := LoadRoot(root)
	if err != nil {
		t.Fatalf("LoadRoot: %v", err)
	}
	if len(parsed.Groups) != 1 || parsed.Groups[0].ID != "TG-01.1" {
		t.Fatalf("groups = %+v", parsed.Groups)
	}
}

func TestLoadRootPrefersGroupFilesOverBacklogMd(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "BACKLOG.md"),
		"## [EPIC-01] Phase 0\n\n### [TG-01.1] Legacy group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n")
	writeFile(t, filepath.Join(root, "docs", "backlog", "TG-02.1-group.md"),
		"## [TG-02.1] Group file group [P: H] [READY]\n\n"+
			"```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-02\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-02.1.1** A task\n  - files: `a.go`\n")
	parsed, err := LoadRoot(root)
	if err != nil {
		t.Fatalf("LoadRoot: %v", err)
	}
	if len(parsed.Groups) != 1 || parsed.Groups[0].ID != "TG-02.1" {
		t.Fatalf("groups = %+v, want only the group file's group", parsed.Groups)
	}
}

func TestLoadRootReturnsAnErrorWithNeitherSource(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadRoot(root); err == nil {
		t.Fatal("want an error with no BACKLOG.md and no docs/backlog/")
	}
}

func TestExistsReportsEitherSource(t *testing.T) {
	root := t.TempDir()
	if Exists(root) {
		t.Fatal("want false with neither source")
	}
	writeFile(t, filepath.Join(root, "docs", "backlog", "TG-03.1-group.md"),
		"## [TG-03.1] Group [P: H] [REFINEMENT]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-03\ndepends_on: []\n```\n")
	if !Exists(root) {
		t.Fatal("want true with a docs/backlog/ group file")
	}
}
