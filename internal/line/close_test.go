package line

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
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
	write("BACKLOG.md", closeBacklog)
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

func TestCloseFlipsTheStatusWhenEverythingPasses(t *testing.T) {
	root := closeRepo(t)
	writeResult(t, root, "TSK-08.1.1", goodResult())
	outcome, err := CloseTask(root, "TSK-08.1.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "DONE" {
		t.Fatalf("outcome = %+v", outcome)
	}
	data, _ := os.ReadFile(filepath.Join(root, "BACKLOG.md"))
	if !strings.Contains(string(data), "[TSK-08.1.1] Do it [P: C] [DONE]") {
		t.Fatal("the status token was not flipped")
	}
}

func TestCloseRejectsAResultThatBreaksTheSchema(t *testing.T) {
	root := closeRepo(t)
	bad := goodResult()
	bad["result"] = "FINISHED"
	writeResult(t, root, "TSK-08.1.1", bad)
	outcome, err := CloseTask(root, "TSK-08.1.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "IN_PROGRESS" || len(outcome.Problems) == 0 {
		t.Fatalf("outcome = %+v", outcome)
	}
	if !strings.Contains(outcome.Problems[0], "not one of") {
		t.Fatalf("problem = %q", outcome.Problems[0])
	}
}

func TestCloseRerunsDoneWhen(t *testing.T) {
	root := closeRepo(t)
	writeResult(t, root, "TSK-08.1.1", goodResult())
	if err := os.Remove(filepath.Join(root, "a", "one.go")); err != nil {
		t.Fatal(err)
	}
	outcome, err := CloseTask(root, "TSK-08.1.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status == "DONE" {
		t.Fatal("a failing done_when still closed the task")
	}
	if !strings.Contains(strings.Join(outcome.Problems, "\n"), "done_when") {
		t.Fatalf("problems = %v", outcome.Problems)
	}
}

func TestCloseRunsTheCommentLint(t *testing.T) {
	root := closeRepo(t)
	body := "package a\n\nfunc One() int {\n\tx := 1\n\treturn x\n}\n"
	if err := os.WriteFile(filepath.Join(root, "a", "one.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	writeResult(t, root, "TSK-08.1.1", goodResult())
	outcome, err := CloseTask(root, "TSK-08.1.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(outcome.Problems, "\n"), "UNDOCUMENTED") {
		t.Fatalf("problems = %v", outcome.Problems)
	}
}

func TestSecondFailureBlocksTheTask(t *testing.T) {
	root := closeRepo(t)
	bad := goodResult()
	delete(bad, "summary")
	writeResult(t, root, "TSK-08.1.1", bad)
	first, err := CloseTask(root, "TSK-08.1.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "IN_PROGRESS" || first.Attempt != 1 {
		t.Fatalf("first = %+v", first)
	}
	second, err := CloseTask(root, "TSK-08.1.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "BLOCKED" || second.Attempt != 2 {
		t.Fatalf("second = %+v", second)
	}
	data, _ := os.ReadFile(filepath.Join(root, "BACKLOG.md"))
	if !strings.Contains(string(data), "[BLOCKED]") {
		t.Fatal("the blocked status was not written")
	}
}

func TestRepairTextCarriesTheFailure(t *testing.T) {
	root := closeRepo(t)
	bad := goodResult()
	delete(bad, "summary")
	writeResult(t, root, "TSK-08.1.1", bad)
	if _, err := CloseTask(root, "TSK-08.1.1", false); err != nil {
		t.Fatal(err)
	}
	text := RepairText(root, "TSK-08.1.1")
	if !strings.Contains(text, "missing required key") {
		t.Fatalf("repair text = %q", text)
	}
}

func TestClosingClearsTheFailureRecord(t *testing.T) {
	root := closeRepo(t)
	bad := goodResult()
	delete(bad, "summary")
	writeResult(t, root, "TSK-08.1.1", bad)
	if _, err := CloseTask(root, "TSK-08.1.1", false); err != nil {
		t.Fatal(err)
	}
	writeResult(t, root, "TSK-08.1.1", goodResult())
	if _, err := CloseTask(root, "TSK-08.1.1", false); err != nil {
		t.Fatal(err)
	}
	if RepairText(root, "TSK-08.1.1") != "" {
		t.Fatal("the failure record survived a pass")
	}
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

func TestTheLedgerTimesTheBuildAndNotJustTheClose(t *testing.T) {
	root := closeRepo(t)
	if err := SaveRun(root, RunState{Run: "TG-08.1-1", Group: "TG-08.1", Base: "main", Branch: "feat/a"}); err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(root, StateDir, "briefs", "TSK-08.1.1.md")
	if err := os.MkdirAll(filepath.Dir(brief), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brief, []byte("brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	ago := time.Now().Add(-90 * time.Second)
	if err := os.Chtimes(brief, ago, ago); err != nil {
		t.Fatal(err)
	}
	if _, err := CloseTask(root, "TSK-08.1.1", false); err != nil {
		t.Fatal(err)
	}
	entries, err := Book(root).Read("line.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var build, closed *ledger.Entry
	for index, entry := range entries {
		switch entry.Station {
		case "build":
			build = &entries[index]
		case "close":
			closed = &entries[index]
		}
	}
	if build == nil {
		t.Fatal("a close must stamp what the build itself cost")
	}
	if build.Seconds < 80 {
		t.Fatalf("build seconds = %v; the build spans its brief to its close, not the close alone", build.Seconds)
	}
	if closed == nil || closed.Seconds > 30 {
		t.Fatalf("close = %+v; the close still measures only itself", closed)
	}
}

// taskWith parses one task out of a backlog fragment.
func taskWith(t *testing.T, files string) backlog.Task {
	t.Helper()
	text := "### [TG-30.1] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-30.1.1] T [P: C] [READY]\n```yaml\nfiles: [" + files + "]\ndone_when: [\"true\"]\n```\n"
	task, ok := backlog.Parse(text).Task("TSK-30.1.1")
	if !ok {
		t.Fatal("no task")
	}
	return task
}

func TestATaskBranchDoesNotCarryRebuiltBinaries(t *testing.T) {
	cwd := gitRepo(t)
	commit(t, cwd, "a/one.go", "package a\n", "seed")
	commit(t, cwd, "bin/komodo-linux-amd64", "old\n", "binaries")
	if err := os.WriteFile(filepath.Join(cwd, "bin", "komodo-linux-amd64"), []byte("rebuilt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "a", "one.go"), []byte("package a\n\n// Two is two.\nfunc Two() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := commitTask(cwd, taskWith(t, "a/one.go")); err != nil {
		t.Fatal(err)
	}
	changed, err := git(cwd, "show", "--name-only", "--format=", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(changed, "bin/") {
		t.Fatalf("committed %q; every branch rebuilds bin, so carrying it conflicts on every wave merge", changed)
	}
	if !strings.Contains(changed, "a/one.go") {
		t.Fatalf("committed %q; the task's own file must still land", changed)
	}
}

func TestATaskThatOwnsABuiltPathStillCommitsIt(t *testing.T) {
	cwd := gitRepo(t)
	commit(t, cwd, "bin/MANIFEST.sha256", "old\n", "manifest")
	if err := os.WriteFile(filepath.Join(cwd, "bin", "MANIFEST.sha256"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := commitTask(cwd, taskWith(t, "bin/MANIFEST.sha256")); err != nil {
		t.Fatal(err)
	}
	changed, err := git(cwd, "show", "--name-only", "--format=", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(changed, "bin/MANIFEST.sha256") {
		t.Fatalf("committed %q; a task that declares a built path owns it", changed)
	}
}
