package facet

import (
	"os"
	"path/filepath"
	"testing"

	"komodo/internal/detect"
)

// repoRoot walks up from the test's working directory to the repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "..", "..")
}

func TestLoadAllReadsEveryShippedFacet(t *testing.T) {
	facets, err := LoadAll(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aws", "azure", "github-actions", "gcp", "postgres"}
	if len(facets) != len(want) {
		t.Fatalf("facets = %v, want %d entries", facets, len(want))
	}
}

func TestLoadRejectsMcpJSON(t *testing.T) {
	facets, err := LoadAll(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, facet := range facets {
		if _, err := os.Stat(filepath.Join(repoRoot(t), FacetsDir, facet.Name, "mcp.json")); err == nil {
			t.Fatalf("%s ships mcp.json, reserved for a later pass", facet.Name)
		}
	}
}

func TestFacetAppendicesComeFromTheirOwnHeadings(t *testing.T) {
	facet, err := Load(repoRoot(t), "aws")
	if err != nil {
		t.Fatal(err)
	}
	if facet.BuilderAppendix() == "" {
		t.Fatal("builder appendix is empty")
	}
	if facet.ReviewerAppendix() == "" {
		t.Fatal("reviewer appendix is empty")
	}
	if facet.BuilderAppendix() == facet.ReviewerAppendix() {
		t.Fatal("builder and reviewer appendices must differ")
	}
}

func TestSelectMatchesDetectionMarkers(t *testing.T) {
	profile := detect.Profile{Cloud: []string{"aws"}}
	selected, err := Select(repoRoot(t), profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range selected {
		if name == "aws" {
			found = true
		}
	}
	if !found {
		t.Fatalf("selected = %v, want aws", selected)
	}
}

func TestSelectAddsTaskFacetsNeverDropsDetection(t *testing.T) {
	profile := detect.Profile{Cloud: []string{"aws"}}
	selected, err := Select(repoRoot(t), profile, []string{"postgres"})
	if err != nil {
		t.Fatal(err)
	}
	has := map[string]bool{}
	for _, name := range selected {
		has[name] = true
	}
	if !has["aws"] || !has["postgres"] {
		t.Fatalf("selected = %v, want aws and postgres", selected)
	}
}

func TestOverridesFileAddsAFacetDetectionMissed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, OverrideFile), []byte("gcp\n# a comment\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Reuse the shipped facets so LoadAll finds real facet.md and SKILL.md files.
	if err := os.Symlink(filepath.Join(repoRoot(t), FacetsDir), filepath.Join(root, FacetsDir)); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	selected, err := Select(root, detect.Profile{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0] != "gcp" {
		t.Fatalf("selected = %v, want [gcp]", selected)
	}
}
