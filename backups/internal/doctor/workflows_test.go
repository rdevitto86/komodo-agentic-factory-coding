package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckWorkflowsIsSilentWithoutTheDirectory(t *testing.T) {
	root := t.TempDir()
	if got := checkWorkflows(root); len(got) != 0 {
		t.Fatalf("problems = %+v, want none", got)
	}
}

func TestCheckWorkflowsNamesEachFileAndTheFix(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ci.yml"), []byte("on: push\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	problems := checkWorkflows(root)
	if len(problems) != 1 {
		t.Fatalf("problems = %+v, want one", problems)
	}
	if problems[0].Where != filepath.Join(".github", "workflows", "ci.yml") {
		t.Fatalf("where = %q, want the file's path", problems[0].Where)
	}
	if !strings.Contains(problems[0].Detail, "gate is local") || !strings.Contains(problems[0].Detail, "komodo gate --install") {
		t.Fatalf("detail = %q, want it to name the fix", problems[0].Detail)
	}
}

func TestCheckWorkflowsSkipsASubdirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".github", "workflows", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := checkWorkflows(root); len(got) != 0 {
		t.Fatalf("problems = %+v, want none for a bare subdirectory", got)
	}
}
