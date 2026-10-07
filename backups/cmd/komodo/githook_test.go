package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestGitHookUsageFailsWithNoName refuses a call naming no hook.
func TestGitHookUsageFailsWithNoName(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	got := runCLI(t, root, "", "git-hook")
	if got.code != 1 || !strings.Contains(got.stderr, "usage: komodo git-hook") {
		t.Fatalf("exit %d, stderr %q", got.code, got.stderr)
	}
}

// TestGitHookRunsAnUnnamedHookThroughTheGate proves an unrecognized hook name falls back to the
// plain gate step and runs the installed binary.
func TestGitHookRunsAnUnnamedHookThroughTheGate(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	fakeToolkitBinary(t)
	var out, errOut bytes.Buffer
	code, err := gitHook(root, "some-other-hook", nil, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
}

// TestGitHookCommitMsgRefusesWithNoMessageFile proves commit-msg needs git's own message path.
func TestGitHookCommitMsgRefusesWithNoMessageFile(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	var out, errOut bytes.Buffer
	_, err := gitHook(root, "commit-msg", nil, strings.NewReader(""), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "message file") {
		t.Fatalf("err = %v, want it to name the missing message file", err)
	}
}
