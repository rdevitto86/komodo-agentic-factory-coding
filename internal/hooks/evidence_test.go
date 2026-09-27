package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lensDiffAdding is the diff the lens fixtures read: a.go gains lines 2 and 3.
const lensDiffAdding = "--- a/a.go\n+++ b/a.go\n@@ -1,1 +1,3 @@\n package a\n+\n+func Bad() {}\n"

// lensRoot writes a worktree for a group based on feat/base, holding a.go and the lens result.
// It swaps the diff reader for one returning lensDiffAdding.
func lensRoot(t *testing.T, result string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "TG-90.1")
	write := func(name, body string) {
		t.Helper()
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Its own .git stops the worktree walk here; a sandbox's temp dir sits inside the real worktree.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("BACKLOG.md", "# Backlog\n\n### [TG-90.1] A group\n```yaml\ntype: feat\nversion: 3.0.0\nbase: feat/base\n```\n")
	write("a.go", "package a\n\nfunc Bad() {}\n")
	if result != "" {
		write(".komodo/results/TG-90.1-review.json", result)
	}
	previous := lensDiff
	lensDiff = func(worktree, base string) (string, error) {
		if worktree != root || base != "feat/base" {
			return "", fmt.Errorf("diff of %s against %s, want %s against feat/base", worktree, base, root)
		}
		return lensDiffAdding, nil
	}
	t.Cleanup(func() { lensDiff = previous })
	return root
}

func TestEvidenceAllowsALensStopWhenEveryBlockingFindingIsVerified(t *testing.T) {
	cases := []struct {
		name   string
		result string
	}{
		{"no result yet", ""},
		{"no findings", `{"findings":[]}`},
		{"only low findings, which are notes", `{"findings":[{"severity":"low","class":"bug","file":"a.go","line":3}]}`},
		{"a rule ID on a changed line", `{"findings":[{"lens":"quality","rule_id":"QUA-3","severity":"high",` +
			`"class":"convention","file":"a.go","line":3,"title":"vague name"}]}`},
		{"a reproducer that fails on the tree", `{"findings":[{"lens":"correctness","rule_id":"COR-2","severity":"high",` +
			`"class":"bug","file":"a.go","line":3,"evidence":"grep -q Bad a.go && exit 1"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := lensRoot(t, tc.result)
			got := dispatch(t, root, "evidence", map[string]any{"session_id": "ok", "cwd": root})
			if got.code != 0 || got.stdout != "" || got.stderr != "" {
				t.Fatalf("a verified lens was refused: %+v", got)
			}
		})
	}
}

func TestEvidenceAllowsAMeasurementFromTheValidatorsReport(t *testing.T) {
	root := lensRoot(t, `{"findings":[{"lens":"quality","rule_id":"QUA-5","severity":"medium",`+
		`"class":"blast-radius","file":"a.go","line":3,"evidence":"Bad: 12 references in 4 files"}]}`)
	report := `{"measurements":[{"kind":"callers","passed":true,"detail":"Bad: 12 references in 4 files"}]}`
	if err := os.WriteFile(filepath.Join(root, ".komodo", "results", "TG-90.1-validators.json"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	got := dispatch(t, root, "evidence", map[string]any{"session_id": "measured", "cwd": root})
	if got.code != 0 || got.stderr != "" {
		t.Fatalf("a measured finding was refused: %+v", got)
	}
}

func TestEvidenceRefusesAFindingWithoutEvidenceTwiceThenAllows(t *testing.T) {
	root := lensRoot(t, `{"findings":[`+
		`{"lens":"correctness","rule_id":"COR-3","severity":"high","class":"bug","file":"a.go","line":3,`+
		`"title":"empty input panics","evidence":"true"},`+
		`{"lens":"quality","rule_id":"QUA-1","severity":"medium","class":"convention","file":"a.go","line":1,`+
		`"title":"old line"}]}`)
	fields := map[string]any{"session_id": "unproven", "cwd": root}
	for round := 1; round <= 2; round++ {
		got := dispatch(t, root, "evidence", fields)
		if got.code != ExitRefuse {
			t.Fatalf("round %d: code = %d, want %d: %+v", round, got.code, ExitRefuse, got)
		}
		for _, want := range []string{"a.go:3 COR-3 empty input panics", "passes on the current tree",
			"a.go:1 QUA-1 old line", "not on a changed line", "PR notes"} {
			if !strings.Contains(got.stderr, want) {
				t.Fatalf("round %d: refusal %q lacks %q", round, got.stderr, want)
			}
		}
	}
	got := dispatch(t, root, "evidence", fields)
	if got.code != 0 || !strings.Contains(got.stderr, "2 refusals reached; allowing") {
		t.Fatalf("the third stop was not allowed with the findings left as notes: %+v", got)
	}
}

func TestEvidenceFailsOpenOnAValidatorsReportThatIsNotJSON(t *testing.T) {
	root := lensRoot(t, `{"findings":[{"severity":"high","class":"performance","evidence":"slow"}]}`)
	if err := os.WriteFile(filepath.Join(root, ".komodo", "results", "TG-90.1-validators.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := dispatch(t, root, "evidence", map[string]any{"session_id": "bad-report", "cwd": root})
	if got.code != 0 || !strings.Contains(got.stderr, "the validators' report is not JSON") {
		t.Fatalf("a broken report did not fail open: %+v", got)
	}
}

func TestEvidenceFailsOpenWhenItCannotReadWhatItChecks(t *testing.T) {
	bug := `{"findings":[{"severity":"high","class":"bug","evidence":"exit 1"}]}`
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{"a result that is a directory", func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, ".komodo", "results", "TG-90.1-review-x.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"a validators report that is a directory", func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, ".komodo", "results", "TG-90.1-validators.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"a diff that cannot be read", func(t *testing.T, root string) {
			lensDiff = func(string, string) (string, error) { return "", fmt.Errorf("no diff") }
		}},
		{"a tree the reproducer cannot copy", func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, "a.go"), 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "a.go"), 0o644) })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := lensRoot(t, bug)
			tc.setup(t, root)
			got := dispatch(t, root, "evidence", map[string]any{"session_id": "unreadable", "cwd": root})
			if got.code != 0 || !strings.Contains(got.stderr, "allowing") {
				t.Fatalf("an unreadable input did not fail open: %+v", got)
			}
		})
	}
}

func TestEvidenceFailsOpenOnAResultThatIsNotJSON(t *testing.T) {
	root := lensRoot(t, "not json")
	got := dispatch(t, root, "evidence", map[string]any{"session_id": "broken", "cwd": root})
	if got.code != 0 || !strings.Contains(got.stderr, "not a review result") {
		t.Fatalf("a broken result did not fail open: %+v", got)
	}
}
