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

// TestGateStillRunsToolkitChecksWhenNoCommandsJsonExists verifies the gate runs go vet and go test for the toolkit.
func TestGateStillRunsToolkitChecksWhenNoCommandsJsonExists(t *testing.T) {
	root := emptyRepo(t)
	// Create cmd/komodo/main.go to make it look like the toolkit checkout.
	if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create go.mod so go commands work.
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
