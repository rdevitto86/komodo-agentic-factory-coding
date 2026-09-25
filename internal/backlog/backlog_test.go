package backlog

import (
	"os"
	"path/filepath"
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

func TestTaskTierAndFacets(t *testing.T) {
	parsed := Parse("### [TG-01.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] Heavy task [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - go test ./...\n" +
		"tier: heavy\nfacets: [postgres, aws]\n```\n")
	task, ok := parsed.Task("TSK-01.1.1")
	if !ok {
		t.Fatal("task not found")
	}
	if task.Tier() != "heavy" {
		t.Fatalf("tier = %q", task.Tier())
	}
	if got := task.Facets(); len(got) != 2 || got[0] != "postgres" || got[1] != "aws" {
		t.Fatalf("facets = %v", got)
	}
}

func TestTaskTierDefaultsEmpty(t *testing.T) {
	first, _ := Parse(sample).Task("TSK-01.1.1")
	if first.Tier() != "" {
		t.Fatalf("tier = %q, want empty when unset", first.Tier())
	}
	if got := first.Facets(); len(got) != 0 {
		t.Fatalf("facets = %v, want none when unset", got)
	}
}

func TestLintRejectsAnUnknownTier(t *testing.T) {
	broken := "### [TG-02.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-02.1.1] Bad tier [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - go test ./...\n" +
		"tier: extreme\n```\n"
	problems := Lint(Parse(broken))
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "tier must be one of") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no problem names the bad tier; got %v", problems)
	}
}

func TestDumpFieldsRoundTripsADoubleQuote(t *testing.T) {
	var fields Fields
	fields.Set("note", `say "hi" now`)
	dumped := DumpFields(fields)
	again, err := ParseFields(dumped)
	if err != nil {
		t.Fatalf("dumped block does not parse back: %v (dumped: %q)", err, dumped)
	}
	if got := again.String("note"); got != `say "hi" now` {
		t.Fatalf("note = %q, want %q (dumped: %q)", got, `say "hi" now`, dumped)
	}
}

func TestDumpFieldsRoundTripsBothQuoteKinds(t *testing.T) {
	value := `the "guard" can't see it`
	var fields Fields
	fields.Set("note", value)
	fields.Set("context", []any{value})
	dumped := DumpFields(fields)
	again, err := ParseFields(dumped)
	if err != nil {
		t.Fatalf("dumped block does not parse back: %v (dumped: %q)", err, dumped)
	}
	if got := again.String("note"); got != value {
		t.Fatalf("note = %q, want %q (dumped: %q)", got, value, dumped)
	}
	if got := again.List("context"); len(got) != 1 || got[0] != value {
		t.Fatalf("context = %q, want [%q] (dumped: %q)", got, value, dumped)
	}
}

func TestDumpFieldsKeepsASingleQuoteOnlyValueByteForByte(t *testing.T) {
	var fields Fields
	fields.Set("note", "can't: stop")
	if dumped := DumpFields(fields); dumped != "note: \"can't: stop\"\n" {
		t.Fatalf("dumped = %q", dumped)
	}
}

func TestDumpFieldsCollapsesEmbeddedNewlines(t *testing.T) {
	var fields Fields
	fields.Set("note", "first line\nsecond line")
	dumped := DumpFields(fields)
	if strings.Count(dumped, "\n") != 1 {
		t.Fatalf("dumped block must stay on one line per field, got %q", dumped)
	}
	again, err := ParseFields(dumped)
	if err != nil {
		t.Fatalf("dumped block does not parse back: %v (dumped: %q)", err, dumped)
	}
	if strings.Contains(again.String("note"), "\n") {
		t.Fatalf("note kept a raw newline: %q", again.String("note"))
	}
}

func TestAppendTaskRejectsUnknownPriority(t *testing.T) {
	var fields Fields
	fields.Set("files", []any{"c/three.go"})
	fields.Set("done_when", []any{"go test ./..."})
	if _, _, err := AppendTask(sample, "TG-01.1", "Third task", fields, "high", "READY"); err == nil {
		t.Fatal("want an error for an unknown priority")
	}
}

func TestAppendTaskRejectsUnknownStatus(t *testing.T) {
	var fields Fields
	fields.Set("files", []any{"c/three.go"})
	fields.Set("done_when", []any{"go test ./..."})
	if _, _, err := AppendTask(sample, "TG-01.1", "Third task", fields, "H", "SHIPPED"); err == nil {
		t.Fatal("want an error for an unknown status")
	}
}

func TestParseReportsAMalformedTaskHeading(t *testing.T) {
	broken := "### [TG-01.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] Bad heading [P: high] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test ./...\"]\n```\n"
	parsed := Parse(broken)
	if len(parsed.Groups[0].Tasks) != 0 {
		t.Fatalf("a malformed heading must not parse as a task: %+v", parsed.Groups[0].Tasks)
	}
	found := false
	for _, problem := range parsed.Problems {
		if strings.Contains(problem, "TSK-01.1.1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no problem names the malformed heading; got %v", parsed.Problems)
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

// chainGroup builds one group from task specs of id, status, and depends_on list.
func chainGroup(specs ...[3]string) Backlog {
	text := "### [TG-30.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"
	for _, spec := range specs {
		text += "#### [" + spec[0] + "] Task [P: M] [" + spec[1] + "]\n```yaml\nfiles: [a/" + spec[0] + ".go]\ndone_when: [\"go test ./...\"]\n"
		if spec[2] != "" {
			text += "depends_on: [" + spec[2] + "]\n"
		}
		text += "```\n\n"
	}
	return Parse(text)
}

func TestNotesNameAGroupThatIsOneChain(t *testing.T) {
	parsed := chainGroup(
		[3]string{"TSK-30.1.1", "READY", ""},
		[3]string{"TSK-30.1.2", "READY", "TSK-30.1.1"},
		[3]string{"TSK-30.1.3", "READY", "TSK-30.1.2"},
		[3]string{"TSK-30.1.4", "READY", ""},
		[3]string{"TSK-30.1.5", "READY", ""})
	notes := Notes(parsed)
	if len(notes) != 1 || !strings.Contains(notes[0], "TG-30.1: 3 of 5 tasks are one chain") {
		t.Fatalf("notes = %v; a 3-task chain in 5 tasks is serial", notes)
	}
	if problems := Lint(parsed); len(problems) != 0 {
		t.Fatalf("problems = %v; a note never fails lint", problems)
	}
}

func TestNotesSkipTwoShortChains(t *testing.T) {
	parsed := chainGroup(
		[3]string{"TSK-30.1.1", "READY", ""},
		[3]string{"TSK-30.1.2", "READY", "TSK-30.1.1"},
		[3]string{"TSK-30.1.3", "READY", ""},
		[3]string{"TSK-30.1.4", "READY", "TSK-30.1.3"},
		[3]string{"TSK-30.1.5", "READY", ""})
	if notes := Notes(parsed); len(notes) != 0 {
		t.Fatalf("notes = %v; two 2-task chains still build in parallel", notes)
	}
}

func TestNotesNeverCountDoneTasks(t *testing.T) {
	parsed := chainGroup(
		[3]string{"TSK-30.1.1", "DONE", ""},
		[3]string{"TSK-30.1.2", "DONE", "TSK-30.1.1"},
		[3]string{"TSK-30.1.3", "DONE", "TSK-30.1.2"},
		[3]string{"TSK-30.1.4", "READY", ""},
		[3]string{"TSK-30.1.5", "READY", ""},
		[3]string{"TSK-30.1.6", "READY", ""})
	if notes := Notes(parsed); len(notes) != 0 {
		t.Fatalf("notes = %v; a chain that is all DONE is no longer serial work", notes)
	}
}

func TestFindLooksAtTheRootThenDocs(t *testing.T) {
	root := t.TempDir()
	if _, err := Find(root); err == nil {
		t.Fatal("Find named a BACKLOG.md that does not exist")
	}
	docs := filepath.Join(root, "docs", "BACKLOG.md")
	if err := os.MkdirAll(filepath.Dir(docs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docs, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if found, err := Find(root); err != nil || found != docs {
		t.Fatalf("found = %q, err = %v; docs/BACKLOG.md is the fallback", found, err)
	}
	top := filepath.Join(root, "BACKLOG.md")
	if err := os.WriteFile(top, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if found, err := Find(root); err != nil || found != top {
		t.Fatalf("found = %q, err = %v; the root's BACKLOG.md wins", found, err)
	}
	loaded, err := Load(top)
	if err != nil || len(loaded.Tasks()) != 2 {
		t.Fatalf("loaded %d task(s), err = %v", len(loaded.Tasks()), err)
	}
	if _, err := Load(filepath.Join(root, "missing.md")); err == nil {
		t.Fatal("Load read a file that does not exist")
	}
}

func TestLintContextNamesAnAnchorWithNoHeading(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "spec.md"), []byte("# Spec\n\n## Refunds\n\nText.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text := "### [TG-02.1] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-02.1.1] Good [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"true\"]\ncontext: [spec.md#refunds]\n```\n\n" +
		"#### [TSK-02.1.2] Bad [P: C] [READY]\n```yaml\nfiles: [b.go]\ndone_when: [\"true\"]\ncontext: [spec.md#returns]\n```\n\n" +
		"#### [TSK-02.1.3] Shipped [P: C] [DONE]\n```yaml\nfiles: [c.go]\ndone_when: [\"true\"]\ncontext: [spec.md#returns]\n```\n"
	problems := LintContext(root, Parse(text))
	if len(problems) != 1 || !strings.Contains(problems[0], "TSK-02.1.2") || !strings.Contains(problems[0], "spec.md#returns") {
		t.Fatalf("problems = %v; only the open task's missing anchor is a problem", problems)
	}
}
