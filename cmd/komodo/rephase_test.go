package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/git"
	"komodo/internal/pr"
)

// rephaseTestBacklog holds one epic and group sharing a version, its epic branch already pushed.
const rephaseTestBacklog = "## [EPIC-05] Phase 1: the conductor drives\n" +
	"*Goal: one group runs through the conductor within 60 minutes. Ships as `2.0.0`.*\n\n" +
	"### [TG-05.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-05.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

// rephaseCommandRepo builds a remoted repo with the epic branch already on origin.
func rephaseCommandRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	backlogtest.SeedText(t, root, rephaseTestBacklog)
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "seed"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	bare := filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	if _, err := git.Run(root, "remote", "add", "origin", bare); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "push", "origin", "HEAD:refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Run(root, "push", "origin", "HEAD:refs/heads/feat/2.0.0"); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRephaseDeletesTheOldBranchOnYes(t *testing.T) {
	root := rephaseCommandRepo(t)
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		if strings.HasPrefix(strings.Join(args, " "), "pr list") {
			return "[]", nil
		}
		return "", nil
	}}
	var out bytes.Buffer
	if err := rephase(root, "EPIC-05", "3.0.0", client, strings.NewReader("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "deleted feat/2.0.0") {
		t.Fatalf("out = %s, want it to report the delete", out.String())
	}
	if _, err := git.Run(root, "ls-remote", "--exit-code", "--heads", "origin", "refs/heads/feat/2.0.0"); err == nil {
		t.Fatal("feat/2.0.0 should be gone from origin")
	}
}

func TestRephaseKeepsTheOldBranchByDefault(t *testing.T) {
	root := rephaseCommandRepo(t)
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		if strings.HasPrefix(strings.Join(args, " "), "pr list") {
			return "[]", nil
		}
		return "", nil
	}}
	var out bytes.Buffer
	if err := rephase(root, "EPIC-05", "3.0.0", client, strings.NewReader("\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "kept feat/2.0.0") {
		t.Fatalf("out = %s, want it to report the branch kept", out.String())
	}
	if _, err := git.Run(root, "ls-remote", "--exit-code", "--heads", "origin", "refs/heads/feat/2.0.0"); err != nil {
		t.Fatal("feat/2.0.0 should still be on origin")
	}
}

func TestRunRephaseFailsWithTooFewArgs(t *testing.T) {
	root := rephaseCommandRepo(t)
	got := runCLI(t, root, "", "rephase", "EPIC-05")
	if got.code == 0 {
		t.Fatal("want a non-zero exit with a missing new version")
	}
}
