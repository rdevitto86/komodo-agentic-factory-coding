package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckPRTitleFollowsTheTemplate(t *testing.T) {
	good := []string{
		"docs: rename specs to hld and lld, split decisions, carry the backlog",
		"fix(guard): refuse gh pr create",
		"feat!: drop the legacy backlog",
	}
	for _, title := range good {
		if err := checkPRTitle(title); err != nil {
			t.Fatalf("%q: %v", title, err)
		}
	}
	bad := []string{
		"",
		"rename specs",
		"docs: rename specs.",
		"feature: add a thing",
		"docs: " + strings.Repeat("x", 70),
	}
	for _, title := range bad {
		if err := checkPRTitle(title); err == nil {
			t.Fatalf("%q: want refused", title)
		}
	}
}

func TestPRBodyTakesExactlyOneSource(t *testing.T) {
	file := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(file, []byte("## Summary\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := prBody("", file); err != nil || got != "## Summary\n" {
		t.Fatalf("file: %q, %v", got, err)
	}
	if got, err := prBody("inline", ""); err != nil || got != "inline" {
		t.Fatalf("inline: %q, %v", got, err)
	}
	if _, err := prBody("", ""); err == nil {
		t.Fatal("want neither refused")
	}
	if _, err := prBody("inline", file); err == nil {
		t.Fatal("want both refused")
	}
}

func TestPRCreateRefusesABadTitleBeforeTouchingTheForge(t *testing.T) {
	root := emptyRepo(t)
	got := runCLI(t, root, "", "pr", "create", "--title", "Rename specs.", "--body", "b")
	if got.code == 0 || !strings.Contains(got.stderr, "is not <type>: <summary>") {
		t.Fatalf("want a title refusal, got %d: %s%s", got.code, got.stdout, got.stderr)
	}
}
