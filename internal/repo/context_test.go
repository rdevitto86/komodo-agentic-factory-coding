package repo

import (
	"os"
	"path/filepath"
	"testing"
)

// contextRepo builds a repo root with the given context files under .komodo/context.
func contextRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, ContextDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadContextReadsPathsFromFrontmatter(t *testing.T) {
	root := contextRepo(t, map[string]string{
		"go.md": "---\npaths: [\"internal/repo/**\"]\n---\n\nRead the neighbours first.\n",
	})
	contexts, skipped := LoadContext(root)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v", skipped)
	}
	if len(contexts) != 1 || contexts[0].Name != "go.md" {
		t.Fatalf("contexts = %v", contexts)
	}
	if len(contexts[0].Paths) != 1 || contexts[0].Paths[0] != "internal/repo/**" {
		t.Fatalf("paths = %v", contexts[0].Paths)
	}
}

func TestLoadContextWithNoDirReturnsNothing(t *testing.T) {
	root := t.TempDir()
	contexts, skipped := LoadContext(root)
	if contexts != nil || skipped != nil {
		t.Fatalf("contexts = %v, skipped = %v", contexts, skipped)
	}
}

func TestLoadContextSkipsAMalformedFileWithOneLine(t *testing.T) {
	root := contextRepo(t, map[string]string{
		"bad.md":  "No frontmatter here.\n",
		"good.md": "---\npaths: []\n---\n\nBody.\n",
	})
	contexts, skipped := LoadContext(root)
	if len(contexts) != 1 || contexts[0].Name != "good.md" {
		t.Fatalf("contexts = %v", contexts)
	}
	if len(skipped) != 1 || skipped[0] != "bad.md: no frontmatter block" {
		t.Fatalf("skipped = %v", skipped)
	}
}

func TestContextMatchesWithNoPathsAppliesToEveryTask(t *testing.T) {
	context := Context{Name: "all.md", Paths: nil}
	if !context.Matches([]string{"anything/at/all.txt"}) {
		t.Fatal("a context with no paths must match every task")
	}
}

func TestContextMatchesByGlob(t *testing.T) {
	context := Context{Name: "go.md", Paths: []string{"**/*.go"}}
	if !context.Matches([]string{"internal/repo/context.go"}) {
		t.Fatal("a matching glob must match")
	}
	if context.Matches([]string{"README.md"}) {
		t.Fatal("a non-matching glob must not match")
	}
}
