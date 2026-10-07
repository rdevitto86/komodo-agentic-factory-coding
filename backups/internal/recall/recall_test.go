package recall

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"komodo/internal/mount"
	"komodo/internal/mount/ollama"
)

// reviewerBody is a minimal reviewer role body with every slot Brief fills.
const reviewerBody = "---\nname: reviewer\ndescription: x\ntier: heavy\ntools: [read]\nsession: true\n" +
	"returns: reviewer.schema.json\n---\n\nReview of group {{group_id}}: {{title}} {{tasks}} {{standards}} " +
	"{{diff}} {{base}} {{lens}}\n"

// lensesBody is a minimal reviewer.lenses.md with one section.
const lensesBody = "## Correctness\n\nCOR-1\n"

// write puts body at root/komodo/rel, marking root as a disk toolkit tree.
func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, "komodo", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// toolkitRoot builds a disk-backed toolkit root, marked by komodo/AGENTS.md, holding nothing else.
func toolkitRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "AGENTS.md", "# Agent Rules\n")
	return root
}

// classes are the bug classes the corpus must seed, one case each at least.
var classes = []string{
	"off-by-one", "nil-dereference", "dropped-error", "inverted-condition", "wrong-comparison-operator",
	"path-traversal", "shell-injection", "map-race", "defer-in-loop", "guard-early-return",
	"test-asserts-nothing", "prune-open-run",
}

// hunk matches a unified diff hunk header, capturing the new side's first line.
var hunk = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// addedLines maps each added line's new-side number in a diff to true.
func addedLines(t *testing.T, diff string) map[int]bool {
	t.Helper()
	added := map[int]bool{}
	next := 0
	for _, row := range strings.Split(diff, "\n") {
		if match := hunk.FindStringSubmatch(row); match != nil {
			start, err := strconv.Atoi(match[1])
			if err != nil {
				t.Fatal(err)
			}
			next = start
			continue
		}
		switch {
		case next == 0, strings.HasPrefix(row, "+++"), strings.HasPrefix(row, "-"):
		case strings.HasPrefix(row, "+"):
			added[next] = true
			next++
		default:
			next++
		}
	}
	return added
}

func TestCorpusSeedsEveryClassOnAnAddedLine(t *testing.T) {
	cases, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	bugs, clean := 0, 0
	seen := map[string]bool{}
	for _, each := range cases {
		if each.Clean {
			clean++
			continue
		}
		bugs++
		seen[each.Class] = true
		if !strings.Contains(each.Diff, "+++ b/"+each.File) {
			t.Errorf("%s: diff does not touch %s", each.Name, each.File)
		}
		if !addedLines(t, each.Diff)[each.Line] {
			t.Errorf("%s: line %d is not an added line of the diff", each.Name, each.Line)
		}
	}
	if bugs < 12 || clean < 3 {
		t.Fatalf("corpus holds %d bug and %d clean cases, want at least 12 and 3", bugs, clean)
	}
	for _, class := range classes {
		if !seen[class] {
			t.Errorf("no case seeds %s", class)
		}
	}
}

func TestBriefFillsTheReviewerRoleWithTheDiff(t *testing.T) {
	cases, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	brief, err := Brief(t.TempDir(), cases[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief, "Review of group "+cases[0].Name+":") || !strings.Contains(brief, cases[0].Diff) {
		t.Fatalf("brief lacks the case's name or diff:\n%s", brief)
	}
	if strings.Contains(brief, "{{") {
		t.Fatal("brief left a slot unfilled")
	}
	if !strings.Contains(brief, "COR-1") || !strings.Contains(brief, "SEC-1") {
		t.Fatalf("brief lacks the economy lens text:\n%s", brief)
	}
}

// fakeReviewer answers each chat call through answer, keyed by the case name the brief carries.
func fakeReviewer(t *testing.T, answer func(name string) []map[string]any) Post {
	t.Helper()
	t.Setenv(ollama.WindowEnv, "")
	t.Setenv("HOME", t.TempDir())
	group := regexp.MustCompile(`Review of group (\S+):`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var req struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(data, &req); err != nil || len(req.Messages) == 0 {
			t.Errorf("request = %s", data)
			return
		}
		match := group.FindStringSubmatch(req.Messages[0].Content)
		if match == nil {
			t.Error("brief names no case")
			return
		}
		content, err := json.Marshal(map[string]any{
			"summary": "", "blast_radius": "low", "blast_radius_why": "", "findings": answer(match[1]),
		})
		if err != nil {
			t.Error(err)
			return
		}
		body, err := json.Marshal(map[string]any{"message": map[string]string{"role": "assistant", "content": string(content)}})
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return func(model, brief string, schema []byte) (mount.LocalResult, error) {
		result, err := ollama.Post(server.URL, model, brief, schema)
		return mount.LocalResult{Value: result.Value, TokensIn: result.TokensIn, TokensOut: result.TokensOut}, err
	}
}

// byName indexes the corpus by case name.
func byName(t *testing.T) map[string]Case {
	t.Helper()
	cases, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]Case, len(cases))
	for _, each := range cases {
		out[each.Name] = each
	}
	return out
}

func TestRunScoresAReviewerThatNamesEveryBugAsOne(t *testing.T) {
	cases := byName(t)
	post := fakeReviewer(t, func(name string) []map[string]any {
		each := cases[name]
		if each.Clean {
			return []map[string]any{}
		}
		return []map[string]any{{
			"severity": "high", "class": "bug", "file": each.File, "line": each.Line + 2,
			"title": each.Class, "detail": each.Class,
		}}
	})
	score, err := Run(t.TempDir(), "m", post)
	if err != nil {
		t.Fatal(err)
	}
	if score.Recall != 1.0 || score.Caught != score.Cases || score.Cases < 12 || score.FalsePositives != 0 {
		t.Fatalf("score = %+v, want recall 1 over every bug case and no false findings", score)
	}
}

func TestRunScoresASilentReviewerAsZero(t *testing.T) {
	post := fakeReviewer(t, func(string) []map[string]any { return []map[string]any{} })
	score, err := Run(t.TempDir(), "m", post)
	if err != nil {
		t.Fatal(err)
	}
	if score.Recall != 0 || score.Caught != 0 || score.Cases < 12 {
		t.Fatalf("score = %+v, want recall 0", score)
	}
}

func TestRunCountsMediumFindingsOnCleanCasesAsFalse(t *testing.T) {
	cases := byName(t)
	post := fakeReviewer(t, func(name string) []map[string]any {
		if !cases[name].Clean {
			return []map[string]any{}
		}
		return []map[string]any{
			{"severity": "medium", "class": "bug", "file": "x.go", "line": 1, "title": "t", "detail": "d"},
			{"severity": "low", "class": "simplify", "file": "x.go", "line": 2, "title": "t", "detail": "d"},
		}
	})
	score, err := Run(t.TempDir(), "m", post)
	if err != nil {
		t.Fatal(err)
	}
	clean := 0
	for _, each := range cases {
		if each.Clean {
			clean++
		}
	}
	if score.FalsePositives != clean {
		t.Fatalf("false positives = %d, want one per clean case (%d)", score.FalsePositives, clean)
	}
}

func TestCatchesNeedsTheFileWithinTheWindowAtMediumOrAbove(t *testing.T) {
	seeded := Case{File: "internal/a/a.go", Line: 20}
	cases := []struct {
		name    string
		finding Finding
		want    bool
	}{
		{"exact", Finding{Severity: "high", File: "internal/a/a.go", Line: 20}, true},
		{"five below", Finding{Severity: "medium", File: "internal/a/a.go", Line: 15}, true},
		{"five above", Finding{Severity: "critical", File: "internal/a/a.go", Line: 25}, true},
		{"six away", Finding{Severity: "high", File: "internal/a/a.go", Line: 26}, false},
		{"low severity", Finding{Severity: "low", File: "internal/a/a.go", Line: 20}, false},
		{"other file", Finding{Severity: "high", File: "internal/b/a.go", Line: 20}, false},
		{"diff prefix", Finding{Severity: "high", File: "b/internal/a/a.go", Line: 20}, true},
		{"no line", Finding{Severity: "high", File: "internal/a/a.go"}, false},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			if got := Catches(seeded, []Finding{each.finding}); got != each.want {
				t.Fatalf("Catches = %v, want %v", got, each.want)
			}
		})
	}
}

func TestSaveKeysTheScoreByModelAndKeepsTheOthers(t *testing.T) {
	file := filepath.Join(t.TempDir(), ".komodo", "recall.json")
	if err := Save(file, "small", Score{Cases: 12, Recall: 0.25}); err != nil {
		t.Fatal(err)
	}
	if err := Save(file, "large", Score{Cases: 12, Recall: 0.75, FalsePositives: 1}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["small"]["recall"] != 0.25 || saved["large"]["recall"] != 0.75 {
		t.Fatalf("saved = %s", data)
	}
	for _, key := range []string{"recall", "cases", "false_positives", "at"} {
		if _, ok := saved["large"][key]; !ok {
			t.Fatalf("saved score lacks %s: %s", key, data)
		}
	}
}

func TestBriefErrorsWhenTheRoleIsMissing(t *testing.T) {
	root := toolkitRoot(t)
	_, err := Brief(root, Case{Name: "x", Diff: "+++ b/a.go\n"})
	if err == nil {
		t.Fatal("want an error when roles/reviewer.md is missing")
	}
}

func TestBriefErrorsWhenTheLensIsMissing(t *testing.T) {
	root := toolkitRoot(t)
	write(t, root, "roles/reviewer.md", reviewerBody)
	write(t, root, "roles/reviewer.schema.json", "{}")
	_, err := Brief(root, Case{Name: "x", Diff: "+++ b/a.go\n"})
	if err == nil {
		t.Fatal("want an error when roles/reviewer.lenses.md is missing")
	}
}

func TestBriefUsesANoStandardsMessageWhenNoneMatch(t *testing.T) {
	root := toolkitRoot(t)
	write(t, root, "roles/reviewer.md", reviewerBody)
	write(t, root, "roles/reviewer.lenses.md", lensesBody)
	brief, err := Brief(root, Case{Name: "x", Diff: "+++ b/a.go\n"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief, "No language standard matches these files.") {
		t.Fatalf("brief = %s, want the no-standards message", brief)
	}
}

func TestFindingsReturnsNilWhenFindingsDoesNotMarshal(t *testing.T) {
	findings := Findings(map[string]any{"findings": math.NaN()})
	if findings != nil {
		t.Fatalf("findings = %v, want nil", findings)
	}
}

func TestFindingsReturnsNilWhenFindingsIsNotAnArray(t *testing.T) {
	findings := Findings(map[string]any{"findings": "not an array"})
	if findings != nil {
		t.Fatalf("findings = %v, want nil", findings)
	}
}

func TestRunErrorsWhenTheSchemaIsMissing(t *testing.T) {
	root := toolkitRoot(t)
	_, err := Run(root, "m", func(string, string, []byte) (mount.LocalResult, error) {
		t.Fatal("post must not run without a schema")
		return mount.LocalResult{}, nil
	})
	if err == nil {
		t.Fatal("want an error when roles/reviewer.schema.json is missing")
	}
}

func TestRunErrorsWhenBriefFails(t *testing.T) {
	root := toolkitRoot(t)
	write(t, root, "roles/reviewer.schema.json", "{}")
	_, err := Run(root, "m", func(string, string, []byte) (mount.LocalResult, error) {
		t.Fatal("post must not run when the brief fails to build")
		return mount.LocalResult{}, nil
	})
	if err == nil {
		t.Fatal("want an error when roles/reviewer.md is missing")
	}
}

func TestRunWrapsThePostError(t *testing.T) {
	want := errors.New("model unreachable")
	_, err := Run(t.TempDir(), "m", func(string, string, []byte) (mount.LocalResult, error) {
		return mount.LocalResult{}, want
	})
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("err = %v, want it to wrap %v", err, want)
	}
}

func TestSaveErrorsWithNoFile(t *testing.T) {
	if err := Save("", "m", Score{}); err == nil {
		t.Fatal("want an error when file is empty")
	}
}

func TestSaveErrorsOnCorruptExistingFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "recall.json")
	if err := os.WriteFile(file, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(file, "m", Score{}); err == nil {
		t.Fatal("want an error when the existing file is not valid JSON")
	}
}

func TestSaveErrorsWhenTheExistingFileCannotBeRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not block reads on windows")
	}
	file := filepath.Join(t.TempDir(), "recall.json")
	if err := os.WriteFile(file, []byte("{}"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(file, 0o644) })
	if err := Save(file, "m", Score{}); err == nil {
		t.Fatal("want an error when the existing file cannot be read")
	}
}

func TestSaveErrorsWhenItsDirectoryCannotBeMade(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(blocker, "sub", "recall.json")
	if err := Save(file, "m", Score{}); err == nil {
		t.Fatal("want an error when the file's directory cannot be made")
	}
}
