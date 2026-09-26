package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"komodo/internal/backlog"
)

// writeTree creates each named file, with parent directories, under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// group parses text and returns its one group, failing the test when it doesn't hold exactly one.
func group(t *testing.T, text string) (backlog.Backlog, backlog.Group) {
	t.Helper()
	parsed := backlog.Parse(text)
	if len(parsed.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(parsed.Groups))
	}
	return parsed, parsed.Groups[0]
}

const oneTaskGroup = "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\n" +
	"files: [a/one.go, a/]\ndone_when:\n  - go test ./a/...\ncontext:\n  - docs/spec.md#a\n```\n"

func TestBuildExpandsADirectoryIntoItsFiles(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a/one.go": "package a\n", "a/two.go": "package a\n"})
	parsed, g := group(t, oneTaskGroup)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(card.Files) != 2 || card.Files[0] != "a/one.go" || card.Files[1] != "a/two.go" {
		t.Fatalf("files = %v", card.Files)
	}
}

func TestBuildExpandsAGlob(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a/one.go": "package a\n", "a/one_test.go": "package a\n"})
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\nfiles: [\"a/*_test.go\"]\n```\n"
	parsed, g := group(t, text)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(card.Files) != 1 || card.Files[0] != "a/one_test.go" {
		t.Fatalf("files = %v", card.Files)
	}
}

func TestBuildAllowsANewFileWhoseParentExists(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a/one.go": "package a\n"})
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\nfiles: [a/two.go]\n```\n"
	parsed, g := group(t, text)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(card.Files) != 1 || card.Files[0] != "a/two.go" {
		t.Fatalf("files = %v, want the new file kept", card.Files)
	}
}

func TestBuildDropsANewFileWhoseParentIsMissing(t *testing.T) {
	root := t.TempDir()
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\nfiles: [nope/two.go]\n```\n"
	parsed, g := group(t, text)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(card.Files) != 0 {
		t.Fatalf("files = %v, want none: the parent directory doesn't exist", card.Files)
	}
}

func TestBuildCollectsHandWrittenChecksAndContextDeduped(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a/one.go": "package a\n"})
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\n" +
		"files: [a/one.go]\ndone_when:\n  - go test ./a/...\ncontext:\n  - docs/spec.md#a\n```\n\n" +
		"#### [TSK-01.1.2] Second task [P: C] [READY]\n```yaml\n" +
		"files: [a/one.go]\ndone_when:\n  - go test ./a/...\n  - go vet ./a/...\ncontext:\n  - docs/spec.md#a\n```\n"
	parsed, g := group(t, text)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if got := card.Checks; len(got) != 2 || got[0] != "go test ./a/..." || got[1] != "go vet ./a/..." {
		t.Fatalf("checks = %v", got)
	}
	if got := card.Context; len(got) != 1 || got[0] != "docs/spec.md#a" {
		t.Fatalf("context = %v", got)
	}
}

func TestBuildTaskListCarriesEveryCheckboxInOrder(t *testing.T) {
	root := t.TempDir()
	parsed, g := group(t, oneTaskGroup)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(card.Tasks) != 1 || card.Tasks[0].ID != "TSK-01.1.1" || card.Tasks[0].Done {
		t.Fatalf("tasks = %+v", card.Tasks)
	}
}

func TestBuildBaseIsMainWithNoOverride(t *testing.T) {
	root := t.TempDir()
	parsed, g := group(t, oneTaskGroup)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if card.Base != "main" {
		t.Fatalf("base = %q, want main", card.Base)
	}
}

func TestBuildBaseIsTheExplicitField(t *testing.T) {
	root := t.TempDir()
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\nbase: feat/parent\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\n```\n"
	parsed, g := group(t, text)
	card, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if card.Base != "feat/parent" {
		t.Fatalf("base = %q, want feat/parent", card.Base)
	}
}

func TestBuildBaseFollowsDependsOnToItsGroupsBranch(t *testing.T) {
	root := t.TempDir()
	text := "### [TG-01.1] Parent group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\n```\n\n" +
		"### [TG-01.2] Child group\n```yaml\ntype: feat\nversion: 1.0.0\ndepends_on: [TG-01.1]\n```\n\n" +
		"#### [TSK-01.2.1] Second task [P: C] [READY]\n```yaml\n```\n"
	parsed := backlog.Parse(text)
	if len(parsed.Groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(parsed.Groups))
	}
	card, err := Build(root, parsed, parsed.Groups[1])
	if err != nil {
		t.Fatal(err)
	}
	if card.Base != "feat/parent-group" {
		t.Fatalf("base = %q, want the parent group's branch", card.Base)
	}
}

func TestBuildTierIsHeavyOnlyWhenATaskAsksForIt(t *testing.T) {
	root := t.TempDir()
	parsed, g := group(t, oneTaskGroup)
	standard, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if standard.Tier != "standard" {
		t.Fatalf("tier = %q, want the builder role's own tier", standard.Tier)
	}
	text := "### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\ntier: heavy\n```\n"
	parsed, g = group(t, text)
	heavy, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if heavy.Tier != "heavy" {
		t.Fatalf("tier = %q, want heavy", heavy.Tier)
	}
}

func TestBuildHashIsStableForTheSameContentAndChangesWithIt(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a/one.go": "package a\n"})
	parsed, g := group(t, oneTaskGroup)
	first, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash == "" || first.Hash != second.Hash {
		t.Fatalf("hash = %q vs %q, want the same non-empty hash", first.Hash, second.Hash)
	}
	changedParsed, changedGroup := group(t, oneTaskGroup+"\n#### [TSK-01.1.2] Another task [P: C] [READY]\n```yaml\n```\n")
	changed, err := Build(root, changedParsed, changedGroup)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Hash == first.Hash {
		t.Fatal("hash did not change when the group's content changed")
	}
}

func TestWriteSavesTheCardUnderQueueDir(t *testing.T) {
	root := t.TempDir()
	parsed, g := group(t, oneTaskGroup)
	card, err := Write(root, parsed, g)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, Path("TG-01.1")))
	if err != nil {
		t.Fatalf("card file not written: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("card file is empty")
	}
	if card.Group != "TG-01.1" {
		t.Fatalf("card.Group = %q", card.Group)
	}
}

func TestReadyGroupsReturnsOnlyGroupsWithAReadyTask(t *testing.T) {
	text := "### [TG-01.1] Ready group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] Task [P: C] [READY]\n```yaml\n```\n\n" +
		"### [TG-01.2] Done group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.2.1] Task [P: C] [DONE]\n```yaml\n```\n"
	parsed := backlog.Parse(text)
	groups := ReadyGroups(parsed)
	if len(groups) != 1 || groups[0].ID != "TG-01.1" {
		t.Fatalf("ready groups = %v", groups)
	}
}
