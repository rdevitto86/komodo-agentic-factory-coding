package backlog

import (
	"strings"
	"testing"
)

const sample = "## [EPIC-01] Epic\n\n### [TG-01.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - go test ./...\n```\n\n" +
	"#### [TSK-01.1.2] Second task [P: M] [DONE]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"go build ./...\"]\ndepends_on: [TSK-01.1.1]\n```\n"

func TestParseReadsGroupsAndTasks(t *testing.T) {
	parsed := Parse(sample)
	if len(parsed.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(parsed.Groups))
	}
	group := parsed.Groups[0]
	if group.ID != "TG-01.1" || group.EpicID != "EPIC-01" || group.Version() != "1.0.0" {
		t.Fatalf("group = %+v", group)
	}
	if len(group.Tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(group.Tasks))
	}
	first := group.Tasks[0]
	if first.Status != "READY" || first.Priority != "C" || first.Title != "First task" {
		t.Fatalf("first = %+v", first)
	}
	if got := first.Files(); len(got) != 1 || got[0] != "a/one.go" {
		t.Fatalf("files = %v", got)
	}
	if got := first.Dirs(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("dirs = %v", got)
	}
	if got := parsed.Groups[0].Tasks[1].DependsOn(); len(got) != 1 || got[0] != "TSK-01.1.1" {
		t.Fatalf("depends_on = %v", got)
	}
}

func TestParseKeepsInlineAndDashedLists(t *testing.T) {
	parsed := Parse(sample)
	second, ok := parsed.Task("TSK-01.1.2")
	if !ok {
		t.Fatal("task not found")
	}
	if got := second.DoneWhen(); len(got) != 1 || got[0] != "go build ./..." {
		t.Fatalf("done_when = %v", got)
	}
}

func TestOpenAndReady(t *testing.T) {
	parsed := Parse(sample)
	first, _ := parsed.Task("TSK-01.1.1")
	second, _ := parsed.Task("TSK-01.1.2")
	if !first.Open() || !first.Ready() {
		t.Fatal("READY task must be open and ready")
	}
	if second.Open() || second.Ready() {
		t.Fatal("DONE task must be neither open nor ready")
	}
	if group, ok := parsed.NextGroup(); !ok || group.ID != "TG-01.1" {
		t.Fatalf("next group = %v %v", group.ID, ok)
	}
}

func TestLintAcceptsAGoodBacklog(t *testing.T) {
	if problems := Lint(Parse(sample)); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestLintNamesEveryBreak(t *testing.T) {
	broken := "### [TG-02.1] No version\n```yaml\ntype: nonsense\n```\n\n" +
		"#### [TSK-02.1.1] No files [P: Z] [READY]\n```yaml\ndone_when:\n  - a prose sentence\n" +
		"depends_on: [TSK-09.9.9]\n```\n"
	problems := Lint(Parse(broken))
	want := []string{"no version", "type must be one of", "priority must be one of",
		"declares no files", "does not look like a command", "unknown task TSK-09.9.9"}
	for _, needle := range want {
		found := false
		for _, problem := range problems {
			if strings.Contains(problem, needle) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no problem mentions %q; got %v", needle, problems)
		}
	}
}

func TestLintReportsAnUnterminatedBlock(t *testing.T) {
	problems := Lint(Parse("### [TG-03.1] Bad\n```yaml\ntype: feat\n"))
	if len(problems) == 0 || !strings.Contains(problems[0], "unterminated") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestSetStatusRewritesOnlyTheToken(t *testing.T) {
	out, err := SetStatus(sample, "TSK-01.1.1", "DONE")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#### [TSK-01.1.1] First task [P: C] [DONE]") {
		t.Fatal("status token not rewritten")
	}
	if strings.Count(out, "\n") != strings.Count(sample, "\n") {
		t.Fatal("line count changed")
	}
	if _, err := SetStatus(sample, "TSK-09.9.9", "DONE"); err == nil {
		t.Fatal("want an error for an unknown task")
	}
	if _, err := SetStatus(sample, "TSK-01.1.1", "SHIPPED"); err == nil {
		t.Fatal("want an error for an unknown status")
	}
}

func TestNextTaskIDCountsFromTheHighest(t *testing.T) {
	parsed := Parse(sample)
	if got := NextTaskID(parsed.Groups[0]); got != "TSK-01.1.3" {
		t.Fatalf("next id = %s", got)
	}
}

func TestAppendTaskLandsInsideItsGroup(t *testing.T) {
	var fields Fields
	fields.Set("files", []any{"c/three.go"})
	fields.Set("done_when", []any{"go test ./..."})
	out, id, err := AppendTask(sample, "TG-01.1", "Third task", fields, "H", "READY")
	if err != nil {
		t.Fatal(err)
	}
	if id != "TSK-01.1.3" {
		t.Fatalf("id = %s", id)
	}
	parsed := Parse(out)
	if len(parsed.Groups[0].Tasks) != 3 {
		t.Fatalf("tasks = %d", len(parsed.Groups[0].Tasks))
	}
	if problems := Lint(parsed); len(problems) != 0 {
		t.Fatalf("appended task does not lint: %v", problems)
	}
}

func TestSlugIsKebabAndCapped(t *testing.T) {
	group := Group{ID: "TG-01.1", Title: "The conveyor and the devices!"}
	if got := group.Slug(); got != "the-conveyor-and-the-devices" {
		t.Fatalf("slug = %s", got)
	}
}

func TestFieldsRoundTrip(t *testing.T) {
	fields, err := ParseFields("a: one\nb: [x, y]\nc:\n  - z\nd: 3\ne: true\n")
	if err != nil {
		t.Fatal(err)
	}
	if fields.String("a") != "one" || len(fields.List("b")) != 2 || len(fields.List("c")) != 1 {
		t.Fatalf("fields = %+v", fields)
	}
	out := DumpFields(fields)
	again, err := ParseFields(out)
	if err != nil {
		t.Fatal(err)
	}
	if again.String("a") != "one" || again.String("d") != "3" || again.String("e") != "true" {
		t.Fatalf("round trip lost a value: %q", out)
	}
}

func TestParseFieldsRejectsIndentation(t *testing.T) {
	if _, err := ParseFields("  a: one\n"); err == nil {
		t.Fatal("want an error for a leading indent")
	}
	if _, err := ParseFields("not a pair\n"); err == nil {
		t.Fatal("want an error for a non-pair line")
	}
}

func TestGroupBaseIsEmptyUntilTheGroupDeclaresOne(t *testing.T) {
	parsed := Parse("### [TG-01.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n")
	group, ok := parsed.Group("TG-01.1")
	if !ok {
		t.Fatal("the group did not parse")
	}
	if group.Base() != "" {
		t.Fatalf("base = %q, want empty so the remote's default branch is used", group.Base())
	}
}

func TestGroupBaseIsWhatTheGroupDeclares(t *testing.T) {
	parsed := Parse("### [TG-01.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\nbase: release/2.0\n```\n")
	group, _ := parsed.Group("TG-01.1")
	if group.Base() != "release/2.0" {
		t.Fatalf("base = %q", group.Base())
	}
}

func TestADeclaredDirectoryIsItsOwnScope(t *testing.T) {
	text := "### [TG-20.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-20.1.1] Wide [P: C] [READY]\n```yaml\nfiles: [internal, README.md]\ndone_when: [\"true\"]\n```\n\n" +
		"#### [TSK-20.1.2] Narrow [P: C] [READY]\n```yaml\nfiles: [internal/profile]\ndone_when: [\"true\"]\n```\n"
	parsed := Parse(text)
	wide, _ := parsed.Task("TSK-20.1.1")
	narrow, _ := parsed.Task("TSK-20.1.2")
	if !contains(wide.Dirs(), "internal") {
		t.Fatalf("dirs = %v; a declared directory owns itself, not its parent", wide.Dirs())
	}
	if !contains(wide.Dirs(), ".") {
		t.Fatalf("dirs = %v; a declared root file still scopes to the root", wide.Dirs())
	}
	if !contains(narrow.Dirs(), "internal/profile") {
		t.Fatalf("dirs = %v", narrow.Dirs())
	}
}
