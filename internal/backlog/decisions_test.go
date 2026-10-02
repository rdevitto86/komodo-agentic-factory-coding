package backlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDecisions(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, DecisionsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLintDecisionsAcceptsDistinctNumbersAndAReadme(t *testing.T) {
	root := writeDecisions(t, "README.md", "0001-one.md", "0002-two.md")
	if problems := LintDecisions(root); len(problems) != 0 {
		t.Fatalf("want no problems, got %v", problems)
	}
}

func TestLintDecisionsRefusesASharedNumber(t *testing.T) {
	root := writeDecisions(t, "0001-one.md", "0002-two.md", "0002-other.md")
	problems := LintDecisions(root)
	if len(problems) != 1 || !strings.Contains(problems[0], "decision 0002 is taken by 0002-other.md, 0002-two.md") {
		t.Fatalf("want one shared-number problem, got %v", problems)
	}
}

func TestLintDecisionsRefusesAMisnamedFile(t *testing.T) {
	root := writeDecisions(t, "0001-one.md", "notes.md", "12-short.md")
	problems := LintDecisions(root)
	if len(problems) != 2 {
		t.Fatalf("want two naming problems, got %v", problems)
	}
}

func TestLintDecisionsIgnoresARepoWithoutTheDirectory(t *testing.T) {
	if problems := LintDecisions(t.TempDir()); problems != nil {
		t.Fatalf("want nil, got %v", problems)
	}
}
