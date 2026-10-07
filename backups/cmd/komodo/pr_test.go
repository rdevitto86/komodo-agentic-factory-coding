package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
)

func TestPRBaseDefaultsToTheNewestOpenEpicBranch(t *testing.T) {
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	runGit(t, root, "push", "origin", "main")
	if got := prBase(root, ""); got != "main" {
		t.Fatalf("base = %q, want the default branch with no open epic", got)
	}
	for _, epic := range []backlog.EpicFile{
		{ID: "EPIC-02", Title: "Two", Status: "READY", Version: "2.0.0", GroupsMax: 6},
		{ID: "EPIC-03", Title: "Three", Status: "READY", Version: "3.0.0", GroupsMax: 6},
	} {
		if _, err := backlog.WriteEpic(root, epic); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "push", "origin", "main:refs/heads/feat/2.0.0")
	if got := prBase(root, ""); got != "feat/2.0.0" {
		t.Fatalf("base = %q, want the only epic branch origin holds", got)
	}
	runGit(t, root, "push", "origin", "main:refs/heads/feat/3.0.0")
	if got := prBase(root, ""); got != "feat/3.0.0" {
		t.Fatalf("base = %q, want the newest open epic's branch", got)
	}
	if got := prBase(root, "main"); got != "main" {
		t.Fatalf("base = %q, want an explicit --base kept", got)
	}
}

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

func TestPRCreateRefusesATrailerInTheBodyBeforeTouchingTheForge(t *testing.T) {
	root := emptyRepo(t)
	got := runCLI(t, root, "", "pr", "create", "--title", "feat: a thing", "--body", "b\n\nCo-authored-by: A <a@b.c>")
	if got.code == 0 || !strings.Contains(got.stderr, "trailer or a session link") {
		t.Fatalf("want a trailer refusal, got %d: %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestPRCreateRefusesADetachedHeadThatTracksNoBranch(t *testing.T) {
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	runGit(t, root, "checkout", "--detach", "HEAD")
	got := runCLI(t, root, "", "pr", "create", "--title", "feat: a thing", "--body", "b")
	if got.code == 0 || !strings.Contains(got.stderr, "tracks no branch") {
		t.Fatalf("want a detached-untracked refusal, got %d: %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestPRCreateOnADetachedWorktreeReadsItsTrackedBranch(t *testing.T) {
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	runGit(t, root, "push", "origin", "main")
	runGit(t, root, "checkout", "--detach", "HEAD")
	runGit(t, root, "config", "extensions.worktreeConfig", "true")
	runGit(t, root, "config", "--worktree", "komodo.branch", "task/x")
	got := runCLI(t, root, "", "pr", "create", "--title", "feat: a thing", "--body", "b")
	if got.code == 0 || !strings.Contains(got.stderr, "git push origin HEAD:refs/heads/task/x") {
		t.Fatalf("want a refusal naming the push, got %d: %s%s", got.code, got.stdout, got.stderr)
	}
}

// TestLabelFlagsSetCollectsEveryRepeatedValue proves --label may repeat, each value kept in order.
func TestLabelFlagsSetCollectsEveryRepeatedValue(t *testing.T) {
	var flags labelFlags
	if err := flags.Set("a"); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("b"); err != nil {
		t.Fatal(err)
	}
	if flags.String() != "a,b" {
		t.Fatalf("flags = %q, want a,b", flags.String())
	}
}

// TestChangedFilesReadsTheDiffSinceBase proves changedFiles names a file the branch added since base.
func TestChangedFilesReadsTheDiffSinceBase(t *testing.T) {
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	runGit(t, root, "checkout", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "add new.go")
	got := changedFiles(root, "main")
	if len(got) != 1 || got[0] != "new.go" {
		t.Fatalf("changedFiles = %v, want [new.go]", got)
	}
}

// TestRunPRLabelAppliesTheBranchsLabels proves pr label looks up the open pull request and applies labels.
func TestRunPRLabelAppliesTheBranchsLabels(t *testing.T) {
	fakeGh(t)
	root, _ := tagRepo(t, "main", "## 1.0.0\n")
	runGit(t, root, "push", "origin", "main")
	runGit(t, root, "checkout", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "add new.go")
	got := runCLI(t, root, "", "pr", "label")
	if got.code != 0 || !strings.Contains(got.stdout, "labels:") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", got.code, got.stdout, got.stderr)
	}
}
