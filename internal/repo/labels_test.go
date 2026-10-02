package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLabelsScopeTakesTheFirstMatchingRuleElseTheDefault(t *testing.T) {
	labels := Labels{
		Scopes: []ScopeRule{
			{Label: "scope/pipeline", Paths: []string{"lib/stacks/**"}},
			{Label: "scope/docs", Paths: []string{"docs/**"}},
		},
		Default: "scope/app",
	}
	cases := []struct {
		files []string
		want  string
	}{
		{[]string{"docs/hld.md", "lib/stacks/ci.ts"}, "scope/pipeline"},
		{[]string{"docs/hld.md"}, "scope/docs"},
		{[]string{"src/main.ts"}, "scope/app"},
		{nil, "scope/app"},
	}
	for _, c := range cases {
		if got := labels.Scope(c.files); got != c.want {
			t.Fatalf("Scope(%v) = %q, want %q", c.files, got, c.want)
		}
	}
}

func TestLoadLabelsReadsTheFileAndReportsItsAbsence(t *testing.T) {
	root := t.TempDir()
	if _, found := LoadLabels(root); found {
		t.Fatal("want found false with no labels file")
	}
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"scopes":[{"label":"scope/docs","paths":["docs/**"]}],"default":"scope/app"}`
	if err := os.WriteFile(filepath.Join(root, LabelsFile), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	labels, found := LoadLabels(root)
	if !found || labels.Scope([]string{"docs/a.md"}) != "scope/docs" || labels.Default != "scope/app" {
		t.Fatalf("got %+v, found %v", labels, found)
	}
}
