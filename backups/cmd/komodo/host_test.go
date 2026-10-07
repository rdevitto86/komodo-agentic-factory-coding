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
		t.Fatalf("changes = %v, want only the harness StateDir entry", ignore.Changes)
	}
}

// TestPlanMountedSeesOnlyARenderedFile proves the gate re-renders a host only where one already landed.
func TestPlanMountedSeesOnlyARenderedFile(t *testing.T) {
	dir := t.TempDir()
	rendered := filepath.Join(dir, "settings.json")
	plan := install.Plan{Host: "fake", Root: dir}
	plan.Add(rendered, []byte("{}"), "a rendered file")
	if planMounted(plan) {
		t.Fatal("a host with nothing on disk counted as mounted")
	}
	if err := os.WriteFile(rendered, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !planMounted(plan) {
		t.Fatal("a host with its rendered file on disk did not count as mounted")
	}
}

// TestInstallGlobalDryRunPrintsTheOrchestratorPlanAndWritesNothing proves install --global --dry-run
// names the user-level settings it would write and touches neither the repo nor the user's home.
func TestInstallGlobalDryRunPrintsTheOrchestratorPlanAndWritesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "install", "--global", "--dry-run")
	if got.code != 0 {
		t.Fatalf("install --global --dry-run exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, filepath.Join(".claude", "settings.json")) {
		t.Fatalf("stdout = %q, want it to name the user's settings.json", got.stdout)
	}
	settings := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		t.Fatalf("install --global --dry-run wrote %s", settings)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("install --global --dry-run touched the repo")
	}
}

// TestInstallGlobalWritesTheResolvedHookNotTheRawBinary proves install --global renders the user's
// settings with the published hook's path, not the unresolved binary flag, and leaves the repo alone.
func TestInstallGlobalWritesTheResolvedHookNotTheRawBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := fixtureRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "komodo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "install", "--global")
	if got.code != 0 {
		t.Fatalf("install --global exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("install --global wrote no settings: %v", err)
	}
	published := filepath.Join(home, ".komodo", "bin", "komodo")
	if !strings.Contains(string(data), published+" guard") {
		t.Fatalf("settings = %s, want the guard command naming the published hook %s", data, published)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("install --global rendered the repo; it must touch only the machine")
	}
}

// TestStringOrNoneNamesAnEmptyValue proves stringOrNone prints none for an empty string and the
// value itself otherwise.
func TestStringOrNoneNamesAnEmptyValue(t *testing.T) {
	if got := stringOrNone(""); got != "none" {
		t.Fatalf("stringOrNone(\"\") = %q, want none", got)
	}
	if got := stringOrNone("main"); got != "main" {
		t.Fatalf("stringOrNone(main) = %q, want main", got)
	}
}
