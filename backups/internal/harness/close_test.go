package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/git"
)

const closeBacklog = "### [TG-08.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-08.1.1] Do it [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - test -f a/one.go\n```\n"

// closeRepo builds a repo with a backlog, a builder schema, and the task's file.
func closeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	backlogtest.SeedText(t, root, closeBacklog)
	write("a/one.go", "package a\n\n// One returns one.\nfunc One() int {\n\tx := 1\n\treturn x\n}\n")
	schema := `{"type":"object","properties":{"result":{"type":"string","enum":["DONE","BLOCKED"]},` +
		`"summary":{"type":"string"},"changed":{"type":"array","items":{"type":"object",` +
		`"properties":{"path":{"type":"string"},"what":{"type":"string"}},"required":["path","what"]}},` +
		`"verified":{"type":"array","items":{"type":"object","properties":{"command":{"type":"string"},` +
		`"exit_code":{"type":"integer"}},"required":["command","exit_code"]}}},` +
		`"required":["result","summary","changed","verified"]}`
	write(filepath.Join(RolesDir, "builder.schema.json"), schema)
	return root
}

// writeResult puts a result JSON where the output device reads it.
func writeResult(t *testing.T, root, taskID string, body map[string]any) {
	t.Helper()
	path := ResultPath(root, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// goodResult is a result that satisfies the builder schema.
func goodResult() map[string]any {
	return map[string]any{
		"result": "DONE", "summary": "did it",
		"changed":  []any{map[string]any{"path": "a/one.go", "what": "added One"}},
		"verified": []any{map[string]any{"command": "test -f a/one.go", "exit_code": float64(0)}},
	}
}

func TestRepairTextCarriesTheFailure(t *testing.T) {
	root := closeRepo(t)
	if _, err := bumpAttemptForTest(root, "TSK-08.1.1", "missing required key \"summary\"", ""); err != nil {
		t.Fatal(err)
	}
	text := RepairText(root, "TSK-08.1.1")
	if !strings.Contains(text, "missing required key") {
		t.Fatalf("repair text = %q", text)
	}
}

// bumpAttemptForTest writes an attempt record directly for a test.
func bumpAttemptForTest(root, taskID, failure, diff string) (Attempt, error) {
	attempt := Attempt{Count: 1, Failure: failure, Diff: diff}
	data, err := json.MarshalIndent(attempt, "", "  ")
	if err != nil {
		return attempt, err
	}
	path := attemptPath(root, taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return attempt, err
	}
	return attempt, os.WriteFile(path, append(data, '\n'), 0o644)
}

func TestValidateNamesEveryDeparture(t *testing.T) {
	schema := Schema{
		Type:     "object",
		Required: []string{"result", "changed"},
		Properties: map[string]Schema{
			"result":  {Type: "string", Enum: []any{"DONE", "BLOCKED"}},
			"changed": {Type: "array", Items: &Schema{Type: "object", Required: []string{"path"}, Properties: map[string]Schema{"path": {Type: "string"}}}},
		},
	}
	value := map[string]any{"result": "MAYBE", "changed": []any{map[string]any{"what": "x"}}}
	problems := Validate(schema, value)
	joined := strings.Join(problems, "\n")
	for _, want := range []string{`"MAYBE" is not one of`, `missing required key "path"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems do not mention %q: %v", want, problems)
		}
	}
}

func TestValidateAcceptsAGoodResult(t *testing.T) {
	root := closeRepo(t)
	schema, err := LoadSchema(root, "builder")
	if err != nil {
		t.Fatal(err)
	}
	if problems := Validate(schema, any(goodResult())); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestAStopDuringCommitBuildsPreCommitHookNeverCommits(t *testing.T) {
	const bound = 10 * time.Second
	root := gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-1", Title: "A group", Type: "feat", Branch: "main", Worktree: root,
		Tasks: []PlanTask{{ID: "TSK-1", Files: []string{"a.txt"}}},
	}
	before, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	began := time.Now()
	if err := CommitBuildContext(ctx, root, plan); err == nil {
		t.Fatal("commit build = nil, want the stopped hook's failure")
	}
	if took := time.Since(began); took > bound {
		t.Fatalf("commit build took %s after the stop, want under %s", took, bound)
	}
	after, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil || after != before {
		t.Fatalf("HEAD = %s (%v), want %s; a stop mid-hook must never commit", after, err, before)
	}
}

// TestIsToolkitNeedsTheKomodoModule proves a target repo whose own binary is cmd/komodo is not the toolkit.
func TestIsToolkitNeedsTheKomodoModule(t *testing.T) {
	cases := map[string]bool{"komodo": true, "github.com/example/runner": false, "": false}
	for module, want := range cases {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if module != "" {
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+module+"\n\ngo 1.27\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got := isToolkit(root); got != want {
			t.Errorf("module %q: isToolkit = %v, want %v", module, got, want)
		}
	}
}
