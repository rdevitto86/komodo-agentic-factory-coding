package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// verifyOne verifies a single finding against tree and returns its verdict.
func verifyOne(t *testing.T, tree Tree, finding Finding) Checked {
	t.Helper()
	got, err := Verify(tree, []Finding{finding})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Verify returned %d verdicts, want 1", len(got))
	}
	return got[0]
}

func TestABugOrSecurityFindingBlocksOnlyWhenItsReproducerFailsInAScratchCopy(t *testing.T) {
	cases := []struct {
		name     string
		class    string
		evidence string
		blocks   bool
		why      string
	}{
		{"a reproducer that fails on the tree", "bug", "grep -q broken app.txt && exit 1", true, "fails"},
		{"a security reproducer that fails", "security", "test -f app.txt && exit 1", true, "fails"},
		{"a reproducer that passes", "bug", "grep -q broken app.txt", false, "passes"},
		{"a reproducer that cannot run", "bug", "no-such-command-komodo", false, "did not run"},
		{"no reproducer", "bug", "  ", false, "no reproducer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "app.txt", "broken\n")
			got := verifyOne(t, Tree{Worktree: root}, Finding{Lens: "correctness", RuleID: "COR-2",
				Class: tc.class, File: "app.txt", Line: 1, Evidence: tc.evidence})
			if got.Blocks != tc.blocks || !strings.Contains(got.Why, tc.why) {
				t.Fatalf("verdict = %+v, want blocks %v and a reason naming %q", got, tc.blocks, tc.why)
			}
		})
	}
}

func TestAReproducerNeverTouchesTheRealTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app.txt", "fine\n")
	writeFile(t, root, ".komodo/state.json", "{}\n")
	evidence := "test ! -e .komodo && echo scratch > made.txt && rm app.txt && exit 1"

	got := verifyOne(t, Tree{Worktree: root}, Finding{Class: "bug", Evidence: evidence})
	if !got.Blocks {
		t.Fatalf("verdict = %+v, want the reproducer to fail in a copy with the line's state left out", got)
	}
	if _, err := os.Stat(filepath.Join(root, "made.txt")); !os.IsNotExist(err) {
		t.Fatalf("made.txt reached the real tree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "app.txt")); err != nil {
		t.Fatalf("app.txt left the real tree: %v", err)
	}
}

func TestAReproducerSeesTheTreesSymlinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app.txt", "broken\n")
	if err := os.Symlink("app.txt", filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	got := verifyOne(t, Tree{Worktree: root}, Finding{Class: "bug", Evidence: "test -L link.txt && grep -q broken link.txt && exit 1"})
	if !got.Blocks {
		t.Fatalf("verdict = %+v, want the scratch copy to keep the symlink", got)
	}
}

func TestVerifyReportsATreeItCannotCopy(t *testing.T) {
	cases := []struct {
		name string
		path string
		mode os.FileMode
	}{
		{"an unreadable file", "secret.txt", 0o000},
		{"an unreadable directory", "locked", 0o000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "locked/inside.txt", "x\n")
			writeFile(t, root, "secret.txt", "x\n")
			full := filepath.Join(root, tc.path)
			if err := os.Chmod(full, tc.mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(full, 0o755) })
			if _, err := Verify(Tree{Worktree: root}, []Finding{{Class: "bug", Evidence: "exit 1"}}); err == nil {
				t.Fatal("Verify copied a tree it cannot read")
			}
		})
	}
}

func TestAConventionFindingBlocksOnlyWithItsLensRuleOnAChangedLine(t *testing.T) {
	diff := diffAdding("a/one.go", "package one", "func Bad() {}")
	cases := []struct {
		name    string
		finding Finding
		blocks  bool
		why     string
	}{
		{"its rule on a changed line", Finding{Lens: "quality", RuleID: "QUA-3", Class: "convention", File: "a/one.go", Line: 2},
			true, "QUA-3 on changed line"},
		{"a quality class beside convention", Finding{Lens: "quality", RuleID: "QUA-4", Class: "test-gap", File: "a/one.go", Line: 1},
			true, "QUA-4"},
		{"no rule ID", Finding{Lens: "quality", Class: "convention", File: "a/one.go", Line: 2},
			false, "no checklist rule ID"},
		{"a rule no checklist lists", Finding{Lens: "quality", RuleID: "QUA-9", Class: "simplify", File: "a/one.go", Line: 2},
			false, "no checklist rule ID"},
		{"another lens's rule", Finding{Lens: "quality", RuleID: "SEC-1", Class: "convention", File: "a/one.go", Line: 2},
			false, "belongs to the security lens"},
		{"a line the diff did not change", Finding{Lens: "quality", RuleID: "QUA-1", Class: "convention", File: "a/one.go", Line: 9},
			false, "not on a changed line"},
		{"a file the diff did not change", Finding{Lens: "quality", RuleID: "QUA-1", Class: "convention", File: "b.go", Line: 1},
			false, "not on a changed line"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := verifyOne(t, Tree{Worktree: t.TempDir(), Diff: diff}, tc.finding)
			if got.Blocks != tc.blocks || !strings.Contains(got.Why, tc.why) {
				t.Fatalf("verdict = %+v, want blocks %v and a reason naming %q", got, tc.blocks, tc.why)
			}
		})
	}
}

func TestAPerformanceOrBlastRadiusFindingBlocksOnlyOnAValidatorMeasurement(t *testing.T) {
	report := Report{Measurements: []Measurement{
		{Kind: KindCallers, Passed: true, Detail: "Load: 14 references in 6 files"},
		{Kind: KindTests, Passed: true, Detail: ""},
	}}
	cases := []struct {
		name     string
		class    string
		evidence string
		blocks   bool
		why      string
	}{
		{"blast radius quoting the caller count", "blast-radius", "callers: Load: 14 references in 6 files", true, "callers"},
		{"performance quoting a measurement", "performance", "Load: 14 references in 6 files, each in a loop", true, "callers"},
		{"a number no validator measured", "blast-radius", "Load: 40 references in 20 files", false, "no validator"},
		{"no evidence", "performance", "", false, "no validator"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := verifyOne(t, Tree{Worktree: t.TempDir(), Report: report},
				Finding{Lens: "quality", RuleID: "QUA-5", Class: tc.class, Evidence: tc.evidence})
			if got.Blocks != tc.blocks || !strings.Contains(got.Why, tc.why) {
				t.Fatalf("verdict = %+v, want blocks %v and a reason naming %q", got, tc.blocks, tc.why)
			}
		})
	}
}

func TestAnyOtherFindingBecomesANote(t *testing.T) {
	got := verifyOne(t, Tree{Worktree: t.TempDir()},
		Finding{Lens: "quality", RuleID: "QUA-1", Class: "style", Evidence: "exit 1"})
	if got.Blocks || !strings.Contains(got.Why, `"style"`) {
		t.Fatalf("verdict = %+v, want a note naming its class", got)
	}
}

func TestVerifyKeepsEveryFindingInOrder(t *testing.T) {
	findings := []Finding{{Title: "one", Class: "style"}, {Title: "two", Class: "bug"}, {Title: "three", Class: "performance"}}
	got, err := Verify(Tree{Worktree: t.TempDir()}, findings)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(findings) {
		t.Fatalf("got %d verdicts, want %d", len(got), len(findings))
	}
	for index, each := range got {
		if each.Finding.Title != findings[index].Title {
			t.Fatalf("verdict %d is %q, want %q", index, each.Finding.Title, findings[index].Title)
		}
	}
}
