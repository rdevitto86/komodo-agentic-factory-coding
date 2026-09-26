package line

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/facet"
	profilepkg "komodo/internal/profile"
	repopkg "komodo/internal/repo"
)

const briefBacklog = "### [TG-07.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-07.1.1] Build the thing [P: C] [READY]\n```yaml\nfiles: [a/one.go, bin/komodo-linux-amd64]\n" +
	"done_when:\n  - go test ./a/...\ncontext:\n  - docs/spec/SDD.md#The plan\n```\n"

const briefRole = "---\nname: builder\ndescription: Writes code.\ntier: standard\ntools: [read, edit, write, shell, search]\n" +
	"session: true\nreturns: builder.schema.json\n---\n\nYou are a builder.\n\n# Brief\n\nTask {{task_id}}: {{title}}\n\n" +
	"```yaml\n{{task_block}}\n```\n\n## Repo rules\n{{repo_rules}}\n\n## Repo context\n{{repo_context}}\n\n" +
	"## Context\n{{context}}\n\n## Files\n{{files}}\n\n## Repo profile\n{{repo_profile}}\n\n" +
	"## Standards\n{{standards}}\n\n## Done when\n{{done_when}}\n{{failure}}\n"

// briefRepo builds a repo root with a backlog, a role, a standards skill, and the task's files.
func briefRepo(t *testing.T) string {
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
	write("BACKLOG.md", briefBacklog)
	write(filepath.Join(RolesDir, "builder.md"), briefRole)
	write(filepath.Join(SkillsDir, "standards-go", "SKILL.md"),
		"---\nname: standards-go\ndescription: Go.\nglobs: [\"**/*.go\"]\n---\n\n# Go\n\nGodoc on every export.\n")
	write(filepath.Join(SkillsDir, "standards-comments", "SKILL.md"),
		"---\nname: standards-comments\ndescription: Comments.\nglobs: []\nroles: [builder, reviewer]\n---\n\n# Comments\n\nOne line.\n")
	write(filepath.Join(SkillsDir, "standards-rust", "SKILL.md"),
		"---\nname: standards-rust\ndescription: Rust.\nglobs: [\"**/*.rs\"]\n---\n\n# Rust\n\nOwnership.\n")
	write("AGENTS.md", "# Repo rules\n\nDo the thing.\n")
	write("a/one.go", "package a\n\nfunc One() {}\n")
	write("docs/spec/SDD.md", "# Spec\n\n## The plan\n\nBuild it.\n\n## Other\n\nNo.\n")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "komodo-linux-amd64"), []byte{0, 1, 2, 0}, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBuildBriefFillsEverySlot(t *testing.T) {
	root := briefRepo(t)
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(brief.Text, "{{") {
		t.Fatalf("an unfilled slot remains:\n%s", brief.Text)
	}
	for _, want := range []string{"Build the thing", "Do the thing", "Godoc on every export",
		"One line.", "Build it.", "go test ./a/...", "package a"} {
		if !strings.Contains(brief.Text, want) {
			t.Errorf("brief is missing %q", want)
		}
	}
	if strings.Contains(brief.Text, "Ownership") {
		t.Error("a standard no file triggers reached the brief")
	}
}

func TestBuildBriefNamesTheResultPath(t *testing.T) {
	root := briefRepo(t)
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(StateDir, "results", "TSK-07.1.1.json")
	if brief.Result != want || !strings.Contains(brief.Text, want) {
		t.Fatalf("result path = %q, brief names it = %v", brief.Result, strings.Contains(brief.Text, want))
	}
}

func TestBuildBriefNamesABinaryAndNeverReadsIt(t *testing.T) {
	root := briefRepo(t)
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, "never read") {
		t.Fatalf("the binary was not named by size:\n%s", brief.Text)
	}
	if strings.Contains(brief.Text, "\x00") {
		t.Fatal("binary bytes reached the brief")
	}
}

func TestBuildBriefWritesNothing(t *testing.T) {
	root := briefRepo(t)
	if _, err := BuildBrief(root, root, "TSK-07.1.1", "builder", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, StateDir)); err == nil {
		t.Fatal("building a brief wrote to .komodo")
	}
}

func TestBuildBriefCarriesAFailureIntoARepair(t *testing.T) {
	root := briefRepo(t)
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "exit 1: undefined: One")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, "Previous attempt failed") || !strings.Contains(brief.Text, "undefined: One") {
		t.Fatal("the failure slot did not reach the brief")
	}
}

func TestBuildBriefFillsRepoProfile(t *testing.T) {
	root := briefRepo(t)
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, "languages: Go") || !strings.Contains(brief.Text, "cloud: none") {
		t.Fatalf("repo profile slot missing:\n%s", brief.Text)
	}
}

// facetBacklog is briefBacklog's task with a facets key added.
const facetBacklog = "### [TG-07.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-07.1.1] Build the thing [P: C] [READY]\n```yaml\nfiles: [a/one.go, bin/komodo-linux-amd64]\n" +
	"done_when:\n  - go test ./a/...\ncontext:\n  - docs/spec/SDD.md#The plan\nfacets: [testfacet]\n```\n"

func TestBuildBriefAddsTheFacetsATaskNames(t *testing.T) {
	root := briefRepo(t)
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("BACKLOG.md", facetBacklog)
	write(filepath.Join(facet.FacetsDir, "testfacet", "facet.md"),
		"# Testfacet\n\n## Builder appendix\nBuilder rule text.\n\n## Reviewer appendix\nReviewer rule text.\n")
	write(filepath.Join(facet.FacetsDir, "testfacet", "skill", "SKILL.md"),
		"---\nname: facet-testfacet\ndescription: Test.\n---\n\n# Testfacet\n")
	write(filepath.Join(facet.FacetsDir, "testfacet", "detect.json"), "{}")
	write(filepath.Join(facet.FacetsDir, "testfacet", "commands.json"), "{}")

	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, "Builder rule text.") {
		t.Fatalf("the task's facets key never reached the standards slot:\n%s", brief.Text)
	}
	if strings.Contains(brief.Text, "Reviewer rule text.") {
		t.Fatal("a builder's brief carried the reviewer's appendix")
	}
}

func TestBuildBriefMergesARepoStandardsOverride(t *testing.T) {
	root := briefRepo(t)
	dir := filepath.Join(root, repopkg.StandardsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "house.md"), []byte("# House\n\nOur own extra rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, "Our own extra rule.") {
		t.Fatalf("a repo standards override under .komodo/standards never reached the brief:\n%s", brief.Text)
	}
}

// manyFiles builds a task with count files of length each, on disk under root, and returns the task.
func manyFiles(t *testing.T, root string, count, length int) backlog.Task {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	var names []string
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("a/f%02d.go", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte(strings.Repeat("x", length)), 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	text := "### [TG-08.1] Many\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-08.1.1] Many [P: C] [READY]\n```yaml\nfiles: [" + strings.Join(names, ", ") + "]\ndone_when: [\"go test\"]\n```\n"
	parsed := backlog.Parse(text)
	task, _ := parsed.Task("TSK-08.1.1")
	return task
}

func TestFilesSlotEnforcesTheTotalCapPastTwelveFiles(t *testing.T) {
	root := t.TempDir()
	task := manyFiles(t, root, 15, 5000)
	got := filesSlot(root, task, CapPerFile, CapFilesTotal)
	if len(got) > CapFilesTotal+500 {
		t.Fatalf("files slot = %d chars, want it clipped near the %d total cap", len(got), CapFilesTotal)
	}
}

func TestContextSlotEnforcesItsTotalCap(t *testing.T) {
	root := t.TempDir()
	text := "### [TG-08.2] Many\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-08.2.1] Many [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test\"]\n" +
		"context:\n  - one.md\n  - two.md\n  - three.md\n```\n"
	parsed := backlog.Parse(text)
	task, _ := parsed.Task("TSK-08.2.1")
	for _, name := range []string{"one.md", "two.md", "three.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(strings.Repeat("y", 9000)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := contextSlot(root, task, CapPerFile, CapFilesTotal)
	if len(got) > CapFilesTotal+500 {
		t.Fatalf("context slot = %d chars, want it clipped near the %d total cap", len(got), CapFilesTotal)
	}
}

func TestRepoContextSlotEnforcesItsTotalCap(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, repopkg.ContextDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\npaths: []\n---\n" + strings.Repeat("z", 7000)
	for _, name := range []string{"one.md", "two.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	text := "### [TG-08.3] Many\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-08.3.1] Many [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"go test\"]\n```\n"
	parsed := backlog.Parse(text)
	task, _ := parsed.Task("TSK-08.3.1")
	got := repoContextSlot(root, task, CapRepoContext)
	if len(got) > CapRepoContext+500 {
		t.Fatalf("repo context slot = %d chars, want it clipped near the %d cap", len(got), CapRepoContext)
	}
}

func TestSingleModeBriefsShareTheGroupWorktree(t *testing.T) {
	root := briefRepo(t)
	text := strings.Replace(briefBacklog, "type: feat\nversion: 2.0.0", "type: feat\nversion: 2.0.0\nmode: single", 1)
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(StateDir, "wt", "TG-07.1")
	if brief.Worktree != want {
		t.Fatalf("worktree = %q, want the group's own worktree %q so single mode never splits at QC", brief.Worktree, want)
	}
}

func TestFillRefusesAnUnsuppliedSlot(t *testing.T) {
	if _, err := Fill("{{known}} {{unknown}}", map[string]string{"known": "x"}); err == nil {
		t.Fatal("want an error naming the unsupplied slot")
	}
}

func TestClipMarksTheCut(t *testing.T) {
	text := strings.Repeat("x", 500)
	got := Clip(text, 100, "body")
	if len(got) >= len(text) || !strings.Contains(got, "body truncated") {
		t.Fatalf("clip = %q", got)
	}
	if Clip("short", 100, "body") != "short" {
		t.Fatal("text under the cap must not be clipped")
	}
}

func TestSectionReadsOneHeading(t *testing.T) {
	text := "# Top\n\n## The plan\n\nBuild it.\n\n## Other\n\nNo.\n"
	got := Section(text, "The plan")
	if !strings.Contains(got, "Build it.") || strings.Contains(got, "No.") {
		t.Fatalf("section = %q", got)
	}
	if Section(text, "Missing") != "" {
		t.Fatal("an absent anchor must return nothing")
	}
}

func TestMatchGlobHandlesTheShippedShapes(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"**/*.go", "a/b/c.go", true},
		{"**/*.go", "a/b/c.rs", false},
		{"**/Dockerfile", "deploy/Dockerfile", true},
		{"**/Dockerfile", "deploy/Dockerfile.dev", false},
		{"**/migrations/**", "db/migrations/001.sql", true},
		{"**/migrations/**", "db/schema/001.sql", false},
		{"internal/repo/**", "internal/repo/context.go", true},
		{"internal/repo/**", "internal/other/context.go", false},
	}
	for _, item := range cases {
		if got := MatchGlob(item.pattern, item.path); got != item.want {
			t.Errorf("MatchGlob(%q, %q) = %v", item.pattern, item.path, got)
		}
	}
}

func TestIsTextRejectsInvalidUTF8WhenTheWholeFileWasRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.bin")
	if err := os.WriteFile(path, []byte{0x81, 0x82, 0x83}, 0o644); err != nil {
		t.Fatal(err)
	}
	if IsText(path) {
		t.Fatal("invalid UTF-8 read in full must not pass as text")
	}
}

func TestStandardsForPicksByGlobAndByRole(t *testing.T) {
	root := briefRepo(t)
	all, err := LoadStandards(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("standards = %d", len(all))
	}
	picked := StandardsFor(all, []string{"a/one.go"}, "builder")
	var names []string
	for _, standard := range picked {
		names = append(names, standard.Name)
	}
	if strings.Join(names, ",") != "comments,go" {
		t.Fatalf("picked = %v", names)
	}
	if got := StandardsFor(all, []string{"a/one.go"}, "scout"); len(got) != 1 {
		t.Fatalf("a role with no extra standard picked %d", len(got))
	}
}

func TestTokensEstimatesFourCharacters(t *testing.T) {
	if got := Tokens(strings.Repeat("x", 400)); got != 100 {
		t.Fatalf("tokens = %d", got)
	}
}

func TestTheBriefLandsInTheWorktreeTheBuilderWorksIn(t *testing.T) {
	root := t.TempDir()
	brief := &Brief{
		Task: "TSK-01.1.1", Text: "the brief",
		Path:     filepath.Join(StateDir, "briefs", "TSK-01.1.1.md"),
		Worktree: filepath.Join(StateDir, "wt", "TSK-01.1.1"),
	}
	worktree := filepath.Join(root, brief.Worktree)
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteBrief(root, brief, "feat/a-group"); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{root, worktree} {
		data, err := os.ReadFile(filepath.Join(base, brief.Path))
		if err != nil {
			t.Fatalf("no brief under %s: %v", base, err)
		}
		if string(data) != "the brief" {
			t.Fatalf("brief under %s = %q", base, data)
		}
	}
}

func TestTheBriefCarriesTheSchemaItWillBeJudgedAgainst(t *testing.T) {
	root := briefRepo(t)
	schema := "{\n  \"type\": \"object\",\n  \"required\": [\"result\", \"summary\", \"verified\"]\n}"
	if err := os.WriteFile(filepath.Join(root, RolesDir, "builder.schema.json"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, "\"required\"") || !strings.Contains(brief.Text, "\"summary\"") {
		t.Fatalf("a result cannot be judged against a schema the brief never showed:\n%s", brief.Text)
	}
	if brief.Slots["schema"] == 0 {
		t.Fatal("the schema must be a counted slot")
	}
}

func TestABriefWithoutASchemaStillNamesTheResultPath(t *testing.T) {
	root := briefRepo(t)
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, brief.Result) {
		t.Fatal("the brief must name where the result goes even when no schema ships")
	}
}

// TestBuildBriefFillsFilesAndContextFromTheQueueCard proves a brief reads the group's compiled
// ingest card from .komodo/queue, expanding the task's own glob, not a sibling's files.
func TestBuildBriefFillsFilesAndContextFromTheQueueCard(t *testing.T) {
	root := briefRepo(t)
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The task itself declares a glob; the card carries what it expanded to on disk.
	globBacklog := "### [TG-07.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-07.1.1] Build the thing [P: C] [READY]\n```yaml\nfiles: [a/*.go]\n" +
		"done_when:\n  - go test ./a/...\ncontext:\n  - docs/spec/SDD.md#The plan\n  - docs/spec/SDD.md#Other\n```\n"
	write("BACKLOG.md", globBacklog)
	write("a/two.go", "package a\n\nfunc Two() {}\n")
	write(filepath.Join(StateDir, "queue", "TG-07.1.json"),
		`{"group":"TG-07.1","files":["a/one.go","a/two.go"],"context":["docs/spec/SDD.md#Other"]}`)

	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, "func Two()") {
		t.Fatalf("the brief did not read the queue card's expanded file list:\n%s", brief.Text)
	}
	if !strings.Contains(brief.Text, "### docs/spec/SDD.md#Other") || strings.Contains(brief.Text, "Build it.") {
		t.Fatalf("the brief did not swap in the queue card's own context references:\n%s", brief.Text)
	}
}

// TestBuildBriefKeepsOnlyATasksOwnFilesFromTheCard proves that in a multi-task group, each task's
// brief carries only the files its own pattern matches, never a sibling's.
func TestBuildBriefKeepsOnlyATasksOwnFilesFromTheCard(t *testing.T) {
	root := briefRepo(t)
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	twoTaskBacklog := "### [TG-07.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-07.1.1] First task [P: C] [READY]\n```yaml\nfiles: [a/one.go]\n" +
		"done_when:\n  - go test ./a/...\n```\n\n" +
		"#### [TSK-07.1.2] Second task [P: C] [READY]\n```yaml\nfiles: [b/two.go]\n" +
		"done_when:\n  - go test ./b/...\n```\n"
	write("BACKLOG.md", twoTaskBacklog)
	write("b/two.go", "package b\n\nfunc Two() {}\n")
	write(filepath.Join(StateDir, "queue", "TG-07.1.json"),
		`{"group":"TG-07.1","files":["a/one.go","b/two.go"],"context":[],`+
			`"tasks":[{"id":"TSK-07.1.1","title":"First task"},{"id":"TSK-07.1.2","title":"Second task"}]}`)

	first, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.Text, "### a/one.go") || strings.Contains(first.Text, "### b/two.go") {
		t.Fatalf("the first task's brief must list only a/one.go, not its sibling:\n%s", first.Text)
	}

	second, err := BuildBrief(root, root, "TSK-07.1.2", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(second.Text, "### b/two.go") || strings.Contains(second.Text, "### a/one.go") {
		t.Fatalf("the second task's brief must list only b/two.go, not its sibling:\n%s", second.Text)
	}
}

// TestBuildBriefFallsBackToTheTaskWithoutAQueueCard proves a brief still works before komodo
// ingest has ever run for the group.
func TestBuildBriefFallsBackToTheTaskWithoutAQueueCard(t *testing.T) {
	root := briefRepo(t)
	brief, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(brief.Text, "Build it.") {
		t.Fatalf("the task's own context must still fill the brief with no queue card:\n%s", brief.Text)
	}
}

// TestBuildBriefIsDeterministic proves the same card and tree give the same brief bytes on every
// machine: building twice from the same root must never depend on map iteration or a clock.
func TestBuildBriefIsDeterministic(t *testing.T) {
	root := briefRepo(t)
	first, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildBrief(root, root, "TSK-07.1.1", "builder", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Text != second.Text {
		t.Fatalf("brief bytes differ between two builds of the same card and tree:\n%s\n---\n%s", first.Text, second.Text)
	}
}

func TestAFixBriefCarriesTheGroupsTasksAndBlockingFindings(t *testing.T) {
	root := briefRepo(t)
	review := ResultPath(root, "TG-07.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"findings":[{"severity":"high","class":"bug","file":"a/one.go","line":3,"title":"nil deref","detail":"d","fix":"f"},` +
		`{"severity":"low","class":"simplify","file":"a/one.go","line":9,"title":"minor nit","detail":"d","fix":"f"}]}`
	if err := os.WriteFile(review, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Group: "TG-07.1", Title: "A group", Worktree: ".", Tasks: []PlanTask{{ID: "TSK-07.1.1"}},
		Profile: profilepkg.Profile{SeverityFloor: "high"}}
	brief, err := FixBrief(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TG-07.1-fix", "TSK-07.1.1: Build the thing", "### a/one.go", "go test ./a/...", "nil deref"} {
		if !strings.Contains(brief.Text, want) {
			t.Fatalf("fix brief lacks %q:\n%s", want, brief.Text)
		}
	}
	if strings.Contains(brief.Text, "minor nit") {
		t.Fatal("a finding under the floor is filed, never carried into the fix brief")
	}
	if _, err := os.Stat(filepath.Join(root, StateDir, "briefs", "TG-07.1-fix.md")); err != nil {
		t.Fatalf("the fix brief was not written: %v", err)
	}
}
