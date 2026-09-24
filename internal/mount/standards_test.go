package mount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes one file under root, making its directories.
func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSelectStandardsKeepsOnlyWhatTheRepoCanUse(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "cmd/a.go", "package main\n")
	writeFile(t, root, "node_modules/x/b.ts", "x\n")
	writeFile(t, root, ".komodo/standards/python.md", "Prefer pathlib.\n")
	skills := []Skill{
		{Name: "run", Body: "# Run\n"},
		{Name: "standards-go", Body: "---\nname: standards-go\nglobs: [\"**/*.go\"]\n---\n# Go\n"},
		{Name: "standards-typescript", Body: "---\nname: standards-typescript\nglobs: ['**/*.ts', '**/*.tsx']\n---\n# TS\n"},
		{Name: "standards-python", Body: "---\nname: standards-python\nglobs: [\"**/*.py\"]\n---\n# Py\n"},
		{Name: "standards-sdlc", Body: "---\nname: standards-sdlc\n---\n# Any repo\n"},
	}
	var names []string
	for _, skill := range SelectStandards(root, skills) {
		names = append(names, skill.Name)
	}
	got := strings.Join(names, ",")
	if got != "run,standards-go,standards-python,standards-sdlc" {
		t.Fatalf("selected = %s; want the non-standard, the matched, the forced, and the glob-free", got)
	}
}

func TestToolkitTreesAreSkippedOnlyInTheToolkitItself(t *testing.T) {
	plain := t.TempDir()
	if toolkitTrees(plain) != nil {
		t.Fatal("a plain repo has no toolkit trees")
	}
	toolkitRoot := t.TempDir()
	writeFile(t, toolkitRoot, "cmd/komodo/main.go", "package main\n")
	writeFile(t, toolkitRoot, "komodo/facets/gcp/x.tf", "provider \"google\" {}\n")
	trees := toolkitTrees(toolkitRoot)
	if !trees[filepath.Join(toolkitRoot, "komodo")] || !trees[filepath.Join(toolkitRoot, "templates")] {
		t.Fatalf("trees = %v", trees)
	}
	for _, file := range repoFiles(toolkitRoot) {
		if strings.HasPrefix(file, "komodo/") {
			t.Fatalf("the toolkit's own facet source was walked: %s", file)
		}
	}
}

func TestMergeOverrideAppendsNewAndFoldsACollision(t *testing.T) {
	skills := []Skill{{Name: "review", Body: "# Review\n"}}
	byName := map[string]int{"review": 0}
	MergeOverride(&skills, byName, "review", false, "---\nname: review\n---\nAlso check SQL.\n")
	MergeOverride(&skills, byName, "deploy", true, "---\nname: deploy\n---\n# Deploy\n")
	MergeOverride(&skills, byName, "ghost", false, "an override of nothing")
	if len(skills) != 2 || skills[1].Name != "deploy" {
		t.Fatalf("skills = %+v", skills)
	}
	body := skills[0].Body
	if !strings.Contains(body, "## Repo overrides\n\nAlso check SQL.") || strings.Count(body, "---") != 0 {
		t.Fatalf("folded body = %q", body)
	}
	if stripFrontmatter("no frontmatter") != "no frontmatter" {
		t.Fatal("a body with no frontmatter must pass through")
	}
}
