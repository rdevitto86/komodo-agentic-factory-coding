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

func TestContextMatchesAMultiSegmentDirGlob(t *testing.T) {
	context := Context{Name: "repo.md", Paths: []string{"internal/repo/**"}}
	if !context.Matches([]string{"internal/repo/context.go"}) {
		t.Fatal("internal/repo/** must match a file under internal/repo")
	}
	if context.Matches([]string{"internal/line/clip.go"}) {
		t.Fatal("internal/repo/** must not match a file under a different dir")
	}
}

func TestContextMatchesASingleStarGlob(t *testing.T) {
	context := Context{Name: "web.md", Paths: []string{"src/*.ts", "docs/*.md"}}
	if !context.Matches([]string{"src/a.ts"}) {
		t.Fatal("src/*.ts must match a file directly under src")
	}
	if !context.Matches([]string{"docs/readme.md"}) {
		t.Fatal("docs/*.md must match a file directly under docs")
	}
	if context.Matches([]string{"src/nested/a.ts"}) {
		t.Fatal("src/*.ts must not match a file nested deeper than one segment")
	}
}

func TestLoadContextReadsPathsFromAYAMLBlockList(t *testing.T) {
	root := contextRepo(t, map[string]string{
		"go.md": "---\npaths:\n  - internal/repo/**\n  - src/*.ts\n---\n\nRead the neighbours first.\n",
	})
	contexts, skipped := LoadContext(root)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v", skipped)
	}
	if len(contexts) != 1 || len(contexts[0].Paths) != 2 {
		t.Fatalf("paths = %v", contexts[0].Paths)
	}
	if contexts[0].Paths[0] != "internal/repo/**" || contexts[0].Paths[1] != "src/*.ts" {
		t.Fatalf("paths = %v", contexts[0].Paths)
	}
	if contexts[0].Matches([]string{"README.md"}) {
		t.Fatal("a block-list context must not match every task")
	}
}

func TestLoadContextAcceptsACRLFFile(t *testing.T) {
	root := contextRepo(t, map[string]string{
		"go.md": "---\r\npaths: [\"internal/repo/**\"]\r\n---\r\n\r\nRead the neighbours first.\r\n",
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
