package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGateFailsWhenNoCompileOrVerifyCommandFound verifies the gate rejects a repo with no build checks.
func TestGateFailsWhenNoCompileOrVerifyCommandFound(t *testing.T) {
	root := emptyRepo(t)
	got := runCLI(t, root, "", "gate")
	if got.code != 1 {
		t.Fatalf("exit %d, want 1 when no build checks are detected", got.code)
	}
	if !strings.Contains(got.stderr, ".komodo/commands.json") {
		t.Fatalf("stderr %q, want it to name .komodo/commands.json", got.stderr)
	}
	if !strings.Contains(got.stderr, "no build checks found") {
		t.Fatalf("stderr %q, want it to say no build checks found", got.stderr)
	}
}

// TestGateInALineWorktreeReadsTheMainCheckoutsCommands verifies a worktree finds the root's gitignored build check.
func TestGateInALineWorktreeReadsTheMainCheckoutsCommands(t *testing.T) {
	root := emptyRepo(t)
	runGit(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := []byte(`{"compile": "echo root-compile-ran"}`)
	if err := os.WriteFile(filepath.Join(root, ".komodo", "commands.json"), commands, 0o644); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(t.TempDir(), "TG-01.1")
	runGit(t, root, "worktree", "add", "-q", "-b", "feat/a-group", worktree)
	got := runCLI(t, worktree, "", "gate")
	if strings.Contains(got.stderr, "no build checks found") {
		t.Fatalf("the worktree's gate missed the main checkout's commands.json: %s", got.stderr)
	}
	if !strings.Contains(got.stdout, "root-compile-ran") {
		t.Fatalf("stdout %q, want the root's compile command to run", got.stdout)
	}
}

// TestGateStillRunsToolkitChecksWhenNoCommandsJsonExists verifies the gate runs go vet and go test for the toolkit.
func TestGateStillRunsToolkitChecksWhenNoCommandsJsonExists(t *testing.T) {
	root := emptyRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module komodo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "gate")
	if !strings.Contains(got.stdout, "go vet") {
		t.Fatalf("stdout %q, want it to run go vet", got.stdout)
	}
	if !strings.Contains(got.stdout, "go test") {
		t.Fatalf("stdout %q, want it to run go test", got.stdout)
	}
}
