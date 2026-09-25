package main

import (
	"encoding/json"
	"komodo/internal/recall"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestACommandOutsideAGitRepoFails(t *testing.T) {
	got := runCLI(t, t.TempDir(), "", "lint")
	if got.code != 1 || !strings.Contains(got.stderr, "no git repository") {
		t.Fatalf("exit %d, stderr %q; a command outside a repo names why it stopped", got.code, got.stderr)
	}
}

func TestLintFailsOnATaskWithNoDoneWhen(t *testing.T) {
	root := fixtureRepo(t)
	broken := "# Backlog\n\n### [TG-91.1] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-91.1.1] No proof [P: C] [READY]\n```yaml\nfiles: [a.go]\n```\n"
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "lint")
	if got.code != 1 || !strings.Contains(got.stdout+got.stderr, "no done_when") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", got.code, got.stdout, got.stderr)
	}
}

func TestTheRunCommandsRefuseAGroupWithNoOpenRun(t *testing.T) {
	root := fixtureRepo(t)
	cases := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"brief", "TG-90.2", "--review"}, 1, "not an open run's group"},
		{[]string{"close", "--fix"}, 1, "usage: komodo close --fix"},
		{[]string{"close", "--fix", "TG-90.2"}, 1, "not an open run's group"},
		{[]string{"close", "--group", "TG-99.9"}, 1, "no run is in progress"},
	}
	for _, each := range cases {
		got := runCLI(t, root, "", each.args...)
		if got.code != each.code || !strings.Contains(got.stdout+got.stderr, each.want) {
			t.Fatalf("komodo %v exited %d, want %d with %q\nstdout: %s\nstderr: %s",
				each.args, got.code, each.code, each.want, got.stdout, got.stderr)
		}
	}
}

func TestBriefOffTheLineCutsFromTheCheckedOutBranch(t *testing.T) {
	root := fixtureRepo(t)
	got := runCLI(t, root, "", "brief", "TSK-90.2.1")
	if got.code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s; an ad hoc brief needs no open run", got.code, got.stdout, got.stderr)
	}
	head, err := exec.Command("git", "-C", root, "rev-parse", "main").Output()
	if err != nil {
		t.Fatal(err)
	}
	base, err := exec.Command("git", "-C", filepath.Join(root, ".komodo", "wt", "TSK-90.2.1"), "rev-parse", "HEAD").Output()
	if err != nil || string(base) != string(head) {
		t.Fatalf("worktree HEAD = %q, main = %q, err = %v", base, head, err)
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

func TestMachineWritesTheLocalReviewersResultAndStampsTheLedger(t *testing.T) {
	root := fixtureRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OLLAMA_MODEL", "coder:7b")
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := `{"local":true,"local_reviewer":true}`
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := recall.Save(filepath.Join(home, ".komodo", "recall.json"), "coder:7b", recall.Score{Cases: 20, Recall: 0.95}); err != nil {
		t.Fatal(err)
	}
	chat, err := json.Marshal(map[string]any{
		"message": map[string]string{"role": "assistant",
			"content": `{"summary":"ok","blast_radius":"low","blast_radius_why":"none","findings":[]}`},
		"prompt_eval_count": 10, "eval_count": 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/chat" {
			_, _ = w.Write(chat)
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"coder:7b","model":"coder:7b"}]}`))
	}))
	defer server.Close()
	t.Setenv("OLLAMA_BASE_URL", server.URL)
	binary := filepath.Join(root, "bin", "komodo-"+runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, root, "", "install", "--host", "claude"); got.code != 0 {
		t.Fatalf("install exited %d: %s", got.code, got.stderr)
	}
	brief := filepath.Join(root, ".komodo", "briefs", "TSK-90.2.1.md")
	if err := os.MkdirAll(filepath.Dir(brief), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brief, []byte("Review this diff."), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "machine", "TSK-90.2.1", "--role", "reviewer")
	if got.code != 0 || !strings.Contains(got.stdout, "wrote") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", got.code, got.stdout, got.stderr)
	}
	result, err := os.ReadFile(filepath.Join(root, ".komodo", "results", "TSK-90.2.1.json"))
	if err != nil || !strings.Contains(string(result), `"blast_radius"`) {
		t.Fatalf("result = %q, err = %v", result, err)
	}
	if metrics := runCLI(t, root, "", "metrics"); !strings.Contains(metrics.stdout, "machine") {
		t.Fatalf("the ledger never recorded the machine station:\n%s", metrics.stdout)
	}
}

// fakeGh puts a gh on PATH that answers a pull request view and the review-thread calls.
func fakeGh(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
case "$*" in
  "pr view"*) echo '{"number":7,"url":"https://example.invalid/pr/7","state":"OPEN","title":"t","isDraft":false}' ;;
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
