package repo

import (
	"os"
	"path/filepath"
	"testing"
)

// standardsRepo builds a repo root with one file per named standard under .komodo/standards.
func standardsRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, StandardsDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadStandardsWithFrontmatterIsNew(t *testing.T) {
	root := standardsRepo(t, map[string]string{
		"terraform.md": "---\nglobs: [\"**/*.tf\"]\n---\n\n# Terraform\n\nOne module per resource group.\n",
	})
	standards, skipped := LoadStandards(root)
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v", skipped)
	}
	if len(standards) != 1 || !standards[0].New || standards[0].Name != "terraform" {
		t.Fatalf("standards = %+v", standards)
	}
}

func TestLoadStandardsWithoutFrontmatterAppends(t *testing.T) {
	root := standardsRepo(t, map[string]string{
		"go.md": "Extract this repo's own error-wrapping convention.\n",
	})
	standards, _ := LoadStandards(root)
	if len(standards) != 1 || standards[0].New || standards[0].Name != "go" {
		t.Fatalf("standards = %+v", standards)
	}
}

func TestLoadStandardsWithNoDirReturnsNothing(t *testing.T) {
	root := t.TempDir()
	standards, skipped := LoadStandards(root)
	if standards != nil || skipped != nil {
		t.Fatalf("standards = %v, skipped = %v", standards, skipped)
	}
}
