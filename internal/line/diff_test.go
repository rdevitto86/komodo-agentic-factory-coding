package line

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/git"
	repopkg "komodo/internal/repo"
)

// commitGroupFile commits one group as its own docs/backlog file, the seed backlog.LoadRoot reads.
func commitGroupFile(t *testing.T, root string, group backlog.GroupFile) {
	t.Helper()
	name := filepath.Join("docs", "backlog", group.ID+"-"+backlog.Slug(group.Title)+".md")
	commit(t, root, name, backlog.RenderGroupFileDocument(group), "backlog")
}

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
	commitGroupFile(t, root, backlog.GroupFile{
		ID: "TG-10.1", Title: "A group", Priority: "C", Status: "READY", Type: "feat", Version: "2.0.0",
		Tasks: []backlog.GroupTask{
			{ID: "TSK-10.1.1", Title: "Add one", Files: []string{"a/one.go"}, Checks: []string{"go test ./..."}},
		},
	})
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

func TestReReviewForCarriesOnlyTheRepairAndTheOpenFindings(t *testing.T) {
	root := gitRepo(t)
	commitGroupFile(t, root, backlog.GroupFile{ID: "TG-10.1", Title: "G", Priority: "M", Status: "REFINEMENT", Type: "feat", Version: "2.0.0"})
	commit(t, root, "a/one.go", "package a\n\nfunc One() int { return 1 }\n", "the build")
	reviewed, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit(t, root, "a/two.go", "package a\n\nfunc Two() int { return 2 }\n", "the repair")

	plan := &Plan{Group: "TG-10.1", Title: "G", Base: "main~2", Worktree: "."}
	open := []Finding{{Severity: "high", File: "a/one.go", Line: 3, Title: "One is untested"}}
	input, err := ReReviewFor(root, plan, reviewed, open)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Files) != 1 || input.Files[0] != "a/two.go" {
		t.Fatalf("files = %v, want only the repair's", input.Files)
	}
	if strings.Contains(input.Diff, "func One") || !strings.Contains(input.Diff, "func Two") {
		t.Fatalf("diff = %q, want only the lines since the reviewed commit", input.Diff)
	}
	for _, want := range []string{"`a/one.go:3` high: One is untested", "Close or keep each finding", "evidence"} {
		if !strings.Contains(input.Text, want) {
			t.Errorf("re-review input is missing %q:\n%s", want, input.Text)
		}
	}
}

func TestReReviewForWithNoReviewedCommitReadsTheWholeDiff(t *testing.T) {
	root := gitRepo(t)
	commitGroupFile(t, root, backlog.GroupFile{ID: "TG-10.1", Title: "G", Priority: "M", Status: "REFINEMENT", Type: "feat", Version: "2.0.0"})
	commit(t, root, "a/one.go", "package a\n", "one")
	commit(t, root, "a/two.go", "package a\n", "two")

	input, err := ReReviewFor(root, &Plan{Group: "TG-10.1", Base: "main~2", Worktree: "."}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Files) != 2 || !strings.Contains(input.Text, "No finding is open") {
		t.Fatalf("files = %v, text = %q; want the whole diff and no open finding", input.Files, input.Text)
	}
}

func TestDiffNamesABinaryWithoutItsBytes(t *testing.T) {
	root := gitRepo(t)
	commitGroupFile(t, root, backlog.GroupFile{ID: "TG-10.1", Title: "G", Priority: "M", Status: "REFINEMENT", Type: "feat", Version: "2.0.0"})
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
	backlogtest.SeedText(t, root, body)
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

func TestClipDiffDropsWholeFilesAndNamesTheCount(t *testing.T) {
	pieces := []string{
		strings.Repeat("a", 30000),
		strings.Repeat("b", 30000),
		strings.Repeat("c", 30000),
		strings.Repeat("d", 30000),
	}
	out := clipDiff([]string{"a.go", "b.go", "c.go", "d.go"}, pieces, CapDiff)
	if !strings.Contains(out, "2 of 4 files shown, 2 files omitted") || !strings.Contains(out, "read each omitted file directly: c.go, d.go") {
		t.Fatalf("marker missing or wrong count:\n%s", out[len(out)-200:])
	}
	before, _, found := strings.Cut(out, "\n[... diff clipped")
	if !found {
		t.Fatalf("no clip marker found in:\n%s", out)
	}
	if before != pieces[0]+"\n"+pieces[1] {
		t.Fatal("clip cut a file's diff instead of dropping it whole")
	}
}

func TestDiffForClipsOnAFileBoundary(t *testing.T) {
	root := gitRepo(t)
	commitGroupFile(t, root, backlog.GroupFile{ID: "TG-10.2", Title: "G", Priority: "M", Status: "REFINEMENT", Type: "feat", Version: "2.0.0"})
	for _, name := range []string{"a/big.go", "b/big.go", "c/big.go", "d/big.go"} {
		body := "package p\n\n// Big holds padding.\nvar Big = \"" + strings.Repeat("x", 30000) + "\"\n"
		commit(t, root, name, body, "add "+name)
	}
	plan := &Plan{Group: "TG-10.2", Base: "main~4", Worktree: "."}
	input, err := DiffFor(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	before, marker, found := strings.Cut(input.Diff, "\n[... diff clipped")
	if !found {
		t.Fatalf("diff was not clipped with a marker naming what was dropped:\n%s", input.Diff)
	}
	if !strings.Contains(marker, "files omitted") {
		t.Fatalf("marker does not name how many files were dropped: %q", marker)
	}
	unclipped, err := git.Run(WorktreePath(root, plan.Worktree), "diff", plan.Base+"...HEAD")
	if err != nil {
		t.Fatal(err)
	}
	byFile := fileDiffs(unclipped)
	var wantKept []string
	for _, name := range input.Files {
		wantKept = append(wantKept, byFile[name])
		if strings.Join(wantKept, "\n") == before {
			break
		}
	}
	if strings.Join(wantKept, "\n") != before {
		t.Fatal("clipped diff does not equal a whole prefix of complete files; a hunk was cut")
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

func TestASingleOversizedPieceIsStillClipped(t *testing.T) {
	piece := strings.Repeat("a", CapDiff+1)
	got := clipDiff([]string{"a.go"}, []string{piece}, CapDiff)
	if len(got) > CapDiff+200 {
		t.Fatalf("kept %d chars against a cap of %d; one huge file must not escape the cap", len(got), CapDiff)
	}
	if !strings.Contains(got, "clipped") && !strings.Contains(got, "truncated") {
		t.Fatalf("a clipped diff must say so; got the tail %q", got[max(0, len(got)-120):])
	}
}

// gitCmd runs one git command in dir, failing the test on a non-zero exit.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestDiffForUsesTheRefAddWorktreeResolvedNotAStaleLocalBase(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "shared.go", "package shared\n", "seed")
	gitCmd(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitCmd(t, root, "checkout", "-q", "-b", "other-group")
	commit(t, root, "other/group.go", "package other\n", "another group's commit")
	gitCmd(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitCmd(t, root, "checkout", "-q", "-b", "feat/this-group")
	commit(t, root, "a/one.go", "package a\n", "this group's commit")

	plan := &Plan{Group: "TG-10.1", Base: "main", Worktree: "."}
	input, err := DiffFor(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(input.Files, "a/one.go") {
		t.Fatalf("files = %v, want this group's own file", input.Files)
	}
	if contains(input.Files, "other/group.go") {
		t.Fatalf("files = %v; a stale local base let another group's commit into the review", input.Files)
	}
}

func TestDiffForUsesTheBaseTheRunWasCutFrom(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "shared.go", "package shared\n", "seed")
	gitCmd(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitCmd(t, root, "checkout", "-q", "-b", "feat/1.0.0-alpha.6")
	commit(t, root, "epic/earlier.go", "package epic\n", "an earlier group on the epic branch")
	gitCmd(t, root, "update-ref", "refs/remotes/origin/feat/1.0.0-alpha.6", "HEAD")
	gitCmd(t, root, "checkout", "-q", "-b", "feat/this-group")
	commit(t, root, "a/one.go", "package a\n", "this group's commit")
	if err := SaveRun(root, RunState{Run: "TG-10.1-1", Group: "TG-10.1", Base: "feat/1.0.0-alpha.6", Branch: "feat/this-group"}); err != nil {
		t.Fatal(err)
	}

	input, err := DiffFor(root, &Plan{Group: "TG-10.1", Base: "main", Worktree: "."})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(input.Files, "a/one.go") || contains(input.Files, "epic/earlier.go") {
		t.Fatalf("files = %v; the review must diff against the epic branch the run was cut from", input.Files)
	}
}

func TestDiffForNamesADeletedFileWithoutItsContents(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "old/gone.go", "package old\n\nvar replayed = true\n", "seed")
	gitCmd(t, root, "branch", "base")
	gitCmd(t, root, "rm", "-q", "old/gone.go")
	commit(t, root, "a/one.go", "package a\n", "delete and add")

	input, err := DiffFor(root, &Plan{Group: "TG-10.1", Base: "base", Worktree: "."})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(input.Files, "old/gone.go") || !strings.Contains(input.Diff, "deleted file") {
		t.Fatalf("diff = %q; a deleted file must still be named", input.Diff)
	}
	if strings.Contains(input.Diff, "replayed") {
		t.Fatalf("diff = %q; a deleted file's contents must not fill the reviewer's diff", input.Diff)
	}
}

func TestDiffForCarriesRepoStandardsAndTheFacetReviewerAppendix(t *testing.T) {
	root := gitRepo(t)
	commitGroupFile(t, root, backlog.GroupFile{ID: "TG-10.1", Title: "G", Priority: "M", Status: "REFINEMENT", Type: "feat", Version: "2.0.0"})
	override := "Extract this repo's own error-wrapping convention.\n"
	commit(t, root, filepath.Join(repopkg.StandardsDir, "go.md"), override, "repo standard")
	commit(t, root, "a/one.go", "package a\n", "the change")

	plan := &Plan{Group: "TG-10.1", Base: "main~1", Worktree: "."}
	input, err := DiffFor(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input.Standards, "error-wrapping convention") {
		t.Fatalf("standards is missing the repo override:\n%s", input.Standards)
	}
}

func TestDiffForDecodesAQuotedNonASCIIPathAndKeepsItsChunk(t *testing.T) {
	root := gitRepo(t)
	commitGroupFile(t, root, backlog.GroupFile{ID: "TG-10.1", Title: "G", Priority: "M", Status: "REFINEMENT", Type: "feat", Version: "2.0.0"})
	commit(t, root, "a/one.go", "package a\n", "an ascii file")
	commit(t, root, "a/héllo.go", "package a\n", "a non-ascii file")

	plan := &Plan{Group: "TG-10.1", Base: "main~1", Worktree: "."}
	input, err := DiffFor(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(input.Files, "a/héllo.go") {
		t.Fatalf("files = %v; a C-quoted non-ASCII path must decode to its real name", input.Files)
	}
	if strings.Contains(input.Diff, "could not be matched") {
		t.Fatalf("the non-ASCII file's chunk vanished:\n%s", input.Diff)
	}
	if !strings.Contains(input.Diff, `h\303\251llo.go`) {
		t.Fatalf("the non-ASCII file's own hunk did not reach the diff:\n%s", input.Diff)
	}
}

func TestDiffPiecesMarksAFileWithNoMatchingChunk(t *testing.T) {
	pieces := diffPieces([]string{"a/one.go", "a/missing.go"}, map[string]string{"a/one.go": "diff --git a/one.go b/one.go"})
	if len(pieces) != 2 || !strings.Contains(pieces[1], "could not be matched") {
		t.Fatalf("pieces = %v; a file with no matching chunk must carry a marker, not vanish", pieces)
	}
}
