package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/install"
)

// TestRepoIgnoresCoversASeedChangeAndAProjectChange proves every host's overlay and every rendered
// copy get their own gitignore line, with no host named outside internal/mount.
func TestRepoIgnoresCoversASeedChangeAndAProjectChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolates check-ignore from the developer's own global excludes
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	plan := install.Plan{Host: "claude", Root: root}
	plan.AddSeed(filepath.Join(root, ".claude", "settings.local.json"), []byte("{}"), "the personal overlay")
	plan.AddSeed(filepath.Join(root, "CLAUDE.local.md"), []byte("# Personal overlay\n"), "the personal overlay")
	plan.AddProject(filepath.Join(root, ".claude", "komodo", "AGENTS.md"), []byte("rules"), "the rendered rules")
	ignore := repoIgnores(root, []install.Plan{plan})
	var lines []string
	for _, change := range ignore.Changes {
		lines = append(lines, string(change.Body))
	}
	body := strings.Join(lines, "")
	for _, want := range []string{"/.claude/settings.local.json", "/CLAUDE.local.md", "/.claude/komodo/AGENTS.md"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the gitignore plan lacks %q:\n%s", want, body)
		}
	}
}

// TestRepoIgnoresSkipsAPathAlreadyCovered proves an existing rule needs no line of its own.
func TestRepoIgnoresSkipsAPathAlreadyCovered(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/.claude/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := install.Plan{Host: "claude", Root: root}
	plan.AddSeed(filepath.Join(root, ".claude", "settings.local.json"), []byte("{}"), "the personal overlay")
	ignore := repoIgnores(root, []install.Plan{plan})
	if len(ignore.Changes) != 1 {
		t.Fatalf("changes = %v, want only the line StateDir entry", ignore.Changes)
	}
}
