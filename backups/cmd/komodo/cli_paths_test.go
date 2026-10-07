package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestACommandOutsideAGitRepoFails(t *testing.T) {
	got := runCLI(t, t.TempDir(), "", "lint")
	if got.code != 1 || !strings.Contains(got.stderr, "no git repository") {
		t.Fatalf("exit %d, stderr %q; a command outside a repo names why it stopped", got.code, got.stderr)
	}
}

// TestGlobalGuardAndHookExitZeroOutsideAGitRepo proves the global guard and status hooks, run in
// every session by the user-level settings, do nothing instead of failing one started outside a repo.
func TestGlobalGuardAndHookExitZeroOutsideAGitRepo(t *testing.T) {
	outside := t.TempDir()
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"` + filepath.ToSlash(outside) + `","tool_input":{"command":"echo hi"}}`
	guard := runCLI(t, outside, payload, "guard")
	if guard.code != 0 || guard.stdout != "" || guard.stderr != "" {
		t.Fatalf("guard outside a repo = %+v, want a silent exit 0", guard)
	}
	status := runCLI(t, outside, `{"cwd":"`+filepath.ToSlash(outside)+`"}`, "hook", "status", "--host", "claude")
	if status.code != 0 || status.stdout != "" || status.stderr != "" {
		t.Fatalf("hook status outside a repo = %+v, want a silent exit 0", status)
	}
}

// TestLintFailsOnATaskWithNoDoneWhen proves the group-file grammar's own lint catches a missing done_when.
func TestLintFailsOnATaskWithNoDoneWhen(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	seedGroup(t, root,
		"## [TG-91.1] G [P: C] [READY]\n\n```yaml\ntype: feat\nversion: 1.0.0\nepic: EPIC-91\ndepends_on: []\n```\n\n"+
			"- [ ] **TSK-91.1.1** No proof\n  - files: `a.go`\n")
	got := runCLI(t, root, "", "lint")
	if got.code != 1 || !strings.Contains(got.stdout+got.stderr, "no done_when") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", got.code, got.stdout, got.stderr)
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	root := fixtureRepo(t)
	before, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, root, "", "sync", "--dry-run"); got.code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", got.code, got.stdout, got.stderr)
	}
	after, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil || string(after) != string(before) {
		t.Fatalf("status before %q, after %q, err = %v; a dry run writes nothing", before, after, err)
	}
}

func TestGateRunsTheReposOwnCompileAndVerifyCommands(t *testing.T) {
	root := fixtureRepo(t)
	commands := filepath.Join(root, ".komodo", "commands.json")
	if err := os.MkdirAll(filepath.Dir(commands), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(commands, []byte(`{"compile":"echo compiled-ok","verify":"echo verified-ok"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "gate")
	if strings.Contains(got.stdout, "gate: go vet") || !strings.Contains(got.stdout, "compiled-ok") || !strings.Contains(got.stdout, "verified-ok") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s; a repo outside the toolkit gates on its own commands", got.code, got.stdout, got.stderr)
	}
	if err := os.WriteFile(commands, []byte(`{"verify":"exit 3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	failed := runCLI(t, root, "", "gate")
	if failed.code != 1 || !strings.Contains(failed.stderr, "exit 3") {
		t.Fatalf("exit %d\nstderr: %s; a failing verify fails the gate and names the command", failed.code, failed.stderr)
	}
}

// fakeGh puts a gh on PATH that answers a pull request view or create and the review-thread calls.
func fakeGh(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
case "$*" in
  "pr view"*) echo '{"number":7,"url":"https://example.invalid/pr/7","state":"OPEN","title":"t","isDraft":false}' ;;
  "pr create"*) echo 'https://example.invalid/pr/8' ;;
  *resolveReviewThread*) echo '{"data":{}}' ;;
  "api graphql"*) echo '{"data":{"resource":{"reviewThreads":{"nodes":[` +
		`{"id":"T1","isResolved":false,"path":"a.go","line":3,"comments":{"nodes":[{"body":"fix it","author":{"login":"r"}}]}},` +
		`{"id":"T2","isResolved":true,"path":"b.go","line":1,"comments":{"nodes":[{"body":"done","author":{"login":"r"}}]}}]}}}}' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestThreadsListsOnlyUnresolvedThreadsAndResolvesOne(t *testing.T) {
	fakeGh(t)
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "threads", "7")
	if got.code != 0 || !strings.Contains(got.stdout, `"T1"`) || strings.Contains(got.stdout, `"T2"`) {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", got.code, got.stdout, got.stderr)
	}
	resolved := runCLI(t, root, "", "threads", "--resolve", "T1")
	if resolved.code != 0 || !strings.Contains(resolved.stdout, "resolved T1") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", resolved.code, resolved.stdout, resolved.stderr)
	}
}

func TestRecallScoresTheLocalModelAndRecordsIt(t *testing.T) {
	root := fixtureRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	reply, err := json.Marshal(map[string]any{
		"message": map[string]string{"role": "assistant",
			"content": `{"summary":"ok","blast_radius":"low","blast_radius_why":"none","findings":[]}`},
		"prompt_eval_count": 10, "eval_count": 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(reply)
	}))
	defer server.Close()
	t.Setenv("OLLAMA_BASE_URL", server.URL)
	got := runCLI(t, root, "", "recall", "--model", "coder:7b")
	if got.code != 0 || !strings.Contains(got.stdout, "coder:7b: caught 0/") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", got.code, got.stdout, got.stderr)
	}
	data, err := os.ReadFile(filepath.Join(home, ".komodo", "recall.json"))
	if err != nil || !strings.Contains(string(data), "coder:7b") {
		t.Fatalf("recall.json = %q, err = %v; the score is recorded by model", data, err)
	}
}
