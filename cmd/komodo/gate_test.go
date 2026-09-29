package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitOutput runs one git command in dir and returns its trimmed stdout, failing the test on error.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

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

// TestGateCommentsCheckIsScopedToTheStagedDiff proves a pre-existing undocumented function the commit
// never touches does not fail the gate, only a newly staged one would.
func TestGateCommentsCheckIsScopedToTheStagedDiff(t *testing.T) {
	root := fixtureRepo(t)
	bad := filepath.Join(root, "a", "one.go")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("package a\n\nfunc One() int {\n\tx := 1\n\treturn x\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "a pre-existing undocumented function")

	good := filepath.Join(root, "b", "two.go")
	if err := os.MkdirAll(filepath.Dir(good), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "// Package b is a fixture.\npackage b\n\n// Two returns two.\nfunc Two() int { return 2 }\n"
	if err := os.WriteFile(good, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "b/two.go")

	got := runCLI(t, root, "", "gate")
	if got.code != 0 {
		t.Fatalf("gate exited %d, want 0: %s%s", got.code, got.stdout, got.stderr)
	}
	if strings.Contains(got.stdout, "UNDOCUMENTED") {
		t.Fatalf("stdout %q, want the pre-existing violation left alone", got.stdout)
	}
}

// TestGateSkipsBuildChecksOnADocsOnlyPushRange proves --from/--to scope build checks to what the range touched.
func TestGateSkipsBuildChecksOnADocsOnlyPushRange(t *testing.T) {
	root := fixtureRepo(t)
	from := gitOutput(t, root, "rev-parse", "HEAD")

	notes := filepath.Join(root, "docs", "notes.md")
	if err := os.MkdirAll(filepath.Dir(notes), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notes, []byte("notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "docs only")
	to := gitOutput(t, root, "rev-parse", "HEAD")

	got := runCLI(t, root, "", "gate", "--from", from, "--to", to)
	if got.code != 0 {
		t.Fatalf("gate exited %d, want 0: %s%s", got.code, got.stdout, got.stderr)
	}
	if strings.Contains(got.stdout, "go build") || strings.Contains(got.stdout, "go vet") {
		t.Fatalf("stdout %q, want no build check for a docs-only range", got.stdout)
	}
	for _, step := range []string{"gate: komodo lint", "gate: komodo doctor", "gate: komodo guard check", "gate: komodo comments check"} {
		if !strings.Contains(got.stdout, step) {
			t.Fatalf("stdout %q, want it to still reach %q", got.stdout, step)
		}
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
