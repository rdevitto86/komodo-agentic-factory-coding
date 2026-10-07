package harness

import (
	"os"
	"path/filepath"
	"testing"

	repopkg "komodo/internal/repo"
)

func TestStageLabelFollowsTheVersionPhase(t *testing.T) {
	cases := map[string]string{
		"1.0.0-alpha.5": "stage/alpha",
		"1.0.0-beta.4":  "stage/beta",
		"1.0.0-rc.1":    "stage/rc",
		"1.0.0":         "stage/ga",
		"1.0.0-dev.1":   "",
		"":              "",
	}
	for version, want := range cases {
		if got := StageLabel(version); got != want {
			t.Fatalf("StageLabel(%q) = %q, want %q", version, got, want)
		}
	}
}

func TestScopeLabelPrefersTheRepoFileOverTheToolkitRules(t *testing.T) {
	root := t.TempDir()
	files := []string{"internal/guard/git.go"}
	if got := ScopeLabel(root, files); got != "scope/guard" {
		t.Fatalf("with no labels file got %q, want scope/guard", got)
	}
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, repopkg.LabelsFile), []byte(`{"default":"scope/app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ScopeLabel(root, files); got != "scope/app" {
		t.Fatalf("with a labels file got %q, want scope/app", got)
	}
}

func TestOptionalLabelsAddsTheStageAndBranchFeature(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\n## 1.0.0-beta.3 — 2026-10-01\n\n- x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := OptionalLabels(root, "feat/1.0.0-beta.4")
	if len(got) != 2 || got[0] != "stage/beta" || got[1] != "branch/feature" {
		t.Fatalf("got %v, want [stage/beta branch/feature]", got)
	}
	if got := OptionalLabels(root, "main"); len(got) != 1 || got[0] != "stage/beta" {
		t.Fatalf("against main got %v, want [stage/beta]", got)
	}
}
