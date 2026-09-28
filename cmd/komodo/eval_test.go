package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// evalRoot is an empty repo root: its own .git stops repoRoot's walk before any real checkout.
func evalRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// evalSuite writes a suite of groupsPerRepo groups in a Go and a TypeScript repo, cloned from url.
func evalSuite(t *testing.T, url string, groupsPerRepo int) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"group.md": "### [TG-01.1] Greet\n", "hidden/greet_test.go": "package greet\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	type group struct {
		ID          string   `json:"id"`
		Repo        string   `json:"repo"`
		Commit      string   `json:"commit"`
		File        string   `json:"file"`
		Hidden      string   `json:"hidden"`
		HiddenTests []string `json:"hidden_tests"`
		Test        string   `json:"test"`
	}
	suite := map[string]any{"repos": []map[string]string{
		{"name": "api", "url": url, "language": "Go"},
		{"name": "web", "url": url, "language": "TypeScript"},
	}}
	var groups []group
	for _, repo := range []string{"api", "web"} {
		for index := range groupsPerRepo {
			groups = append(groups, group{
				ID: fmt.Sprintf("TG-01.%d", index+1), Repo: repo, Commit: strings.Repeat("a", 40), File: "group.md",
				Hidden: "hidden", HiddenTests: []string{"greet_test.go"}, Test: "go test ./...",
			})
		}
	}
	suite["groups"] = groups
	data, err := json.Marshal(suite)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "suite.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestEvalListsTheSuiteAndRefusesOneTooSmall proves --list prints each pinned group and fails a short suite.
func TestEvalListsTheSuiteAndRefusesOneTooSmall(t *testing.T) {
	root := evalRoot(t)
	golden := evalSuite(t, "https://example.invalid/r.git", 10)
	short := evalSuite(t, "https://example.invalid/r.git", 1)
	cases := []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{"a golden-sized suite", []string{"--list", "--suite", golden}, 0, "TG-01.10  " + strings.Repeat("a", 40), ""},
		{"a short suite", []string{"--list", "--suite", short}, 1, "web (TypeScript)", "smaller than the golden suite"},
		{"no suite", []string{"--list"}, 1, "", "suite.json"},
		{"neither --list, --cases nor --runs", []string{"--suite", golden}, 2, "", "-cases"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			got := runCLI(t, root, "", append([]string{"eval"}, each.args...)...)
			if got.code != each.code || !strings.Contains(got.stdout, each.stdout) || !strings.Contains(got.stderr, each.stderr) {
				t.Fatalf("exit %d, stdout %q, stderr %q; want %d, %q, %q",
					got.code, got.stdout, got.stderr, each.code, each.stdout, each.stderr)
			}
		})
	}
}

// TestEvalRunsStopWhenAGroupCannotBeCloned proves --runs reaches the eval and fails on a repo it cannot clone.
func TestEvalRunsStopWhenAGroupCannotBeCloned(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-repo")
	suite := evalSuite(t, missing, 1)
	got := runCLI(t, evalRoot(t), "", "eval", "--runs", "1", "--suite", suite, "--work", t.TempDir())
	if got.code != 1 || !strings.Contains(got.stderr, "clone") {
		t.Fatalf("exit %d, stderr %q; want the clone's failure", got.code, got.stderr)
	}
}

// TestEvalCasesStopWhenACaseCannotBeCloned proves --cases reaches the live cases and fails, before any session,
// on a repo it cannot clone.
func TestEvalCasesStopWhenACaseCannotBeCloned(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-repo")
	suite := evalSuite(t, missing, 1)
	got := runCLI(t, evalRoot(t), "", "eval", "--cases", "--suite", suite, "--work", t.TempDir())
	if got.code != 1 || !strings.Contains(got.stderr, "case preflight: host login: clone") ||
		strings.Contains(got.stdout, "passed") {
		t.Fatalf("exit %d, stdout %q, stderr %q; want the first case's clone failure", got.code, got.stdout, got.stderr)
	}
}
