package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// TestLoadLabelsPanicsOnAMalformedFileNamingItsPath proves a present but broken labels.json
// fails loudly instead of silently falling back to the toolkit's own rules.
func TestLoadLabelsPanicsOnAMalformedFileNamingItsPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, LabelsFile)
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("a malformed labels file did not panic")
		}
		if !strings.Contains(fmt.Sprint(r), path) {
			t.Fatalf("panic = %v, want it to name %s", r, path)
		}
	}()
	LoadLabels(root)
}

// TestLoadLabelsRejectsAnUnknownField proves the decode is strict, not merely tolerant of bad JSON.
func TestLoadLabelsRejectsAnUnknownField(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"scopes":[],"default":"scope/app","not_a_real_field":true}`
	if err := os.WriteFile(filepath.Join(root, LabelsFile), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("an unknown labels field did not panic")
		}
	}()
	LoadLabels(root)
}
