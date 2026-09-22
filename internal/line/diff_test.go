package line

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo builds a throwaway repository with one commit on main.
func gitRepo(t *testing.T) string {
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
	return root
}

// commit writes a file and commits it.
func commit(t *testing.T, root, name, body, message string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func TestDiffForCarriesTasksStandardsAndTheDiff(t *testing.T) {
	root := gitRepo(t)
	backlogText := "### [TG-10.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-10.1.1] Add one [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./...\"]\n```\n"
	commit(t, root, "BACKLOG.md", backlogText, "backlog")
	skill := "---\nname: standards-go\ndescription: Go.\nglobs: [\"**/*.go\"]\n---\n\n# Go\n\nGodoc on every export.\n"
	commit(t, root, filepath.Join(SkillsDir, "standards-go", "SKILL.md"), skill, "skill")
	commit(t, root, "a/one.go", "package a\n\n// One returns one.\nfunc One() int { return 1 }\n", "the change")

	plan := &Plan{
		Group: "TG-10.1", Title: "A group", Base: "main~1", Branch: "main",
		Worktree: ".", Tasks: []PlanTask{{ID: "TSK-10.1.1", Title: "Add one"}},
	}
	input, err := DiffFor(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Files) != 1 || input.Files[0] != "a/one.go" {
		t.Fatalf("files = %v", input.Files)
	}
	for _, want := range []string{"func One", "TSK-10.1.1", "Godoc on every export", "Diff against main~1"} {
		if !strings.Contains(input.Text, want) {
			t.Errorf("review input is missing %q", want)
		}
	}
}

func TestDiffNamesABinaryWithoutItsBytes(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "BACKLOG.md", "### [TG-10.1] G\n```yaml\ntype: feat\nversion: 2.0.0\n```\n", "backlog")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "bin/komodo-linux-amd64", "\x00\x01binary\x00", "the binary")
	plan := &Plan{Group: "TG-10.1", Base: "main~1", Worktree: "."}
	input, err := DiffFor(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input.Text, "is a prebuilt binary") {
		t.Fatalf("the binary was not named:\n%s", input.Text)
	}
	if strings.Contains(input.Diff, "binary\x00") {
		t.Fatal("binary bytes reached the review input")
	}
}

func TestBuildReportUsesTheAccessibilityHeadings(t *testing.T) {
	root := t.TempDir()
	body := "### [TG-11.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-11.1.1] One [P: C] [DONE]\n```yaml\nfiles: [a/x.go]\ndone_when: [\"go test\"]\n```\n\n" +
		"#### [TSK-11.1.2] Two [P: C] [BLOCKED]\n```yaml\nfiles: [b/y.go]\ndone_when: [\"go test\"]\n```\n"
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := bumpAttempt(root, "TSK-11.1.2", "done_when `go test` failed: exit 1", ""); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Group: "TG-11.1", Title: "A group",
		Tasks: []PlanTask{{ID: "TSK-11.1.1"}, {ID: "TSK-11.1.2"}}}
	report, err := BuildReport(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Done) != 1 || len(report.Blocked) != 1 {
		t.Fatalf("report = %+v", report)
	}
	for _, want := range []string{"## ✅ Successful Changes", "## ❌ Blocked Changes", "## 📌 Callouts", "go test"} {
		if !strings.Contains(report.Text, want) {
			t.Errorf("report is missing %q:\n%s", want, report.Text)
		}
	}
}

func TestReportOpensWithTheVerdict(t *testing.T) {
	report := &Report{Group: "TG-11.1", Done: []string{"a"}, Blocked: nil}
	text := renderReport(report, nil)
	first, _, _ := strings.Cut(text, "\n")
	if !strings.HasPrefix(first, "TG-11.1: 1 done") {
		t.Fatalf("first line = %q", first)
	}
}
