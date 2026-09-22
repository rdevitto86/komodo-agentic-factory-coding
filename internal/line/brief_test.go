package line

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const briefBacklog = "### [TG-07.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-07.1.1] Build the thing [P: C] [READY]\n```yaml\nfiles: [a/one.go, bin/komodo-linux-amd64]\n" +
	"done_when:\n  - go test ./a/...\ncontext:\n  - docs/spec/SDD.md#The plan\n```\n"

const briefRole = "---\nname: builder\ndescription: Writes code.\ntier: standard\ntools: [read, edit, write, shell, search]\n" +
	"session: true\nreturns: builder.schema.json\n---\n\nYou are a builder.\n\n# Brief\n\nTask {{task_id}}: {{title}}\n\n" +
	"```yaml\n{{task_block}}\n```\n\n## Repo rules\n{{repo_rules}}\n\n## Repo context\n{{repo_context}}\n\n" +
	"## Context\n{{context}}\n\n## Files\n{{files}}\n\n## Standards\n{{standards}}\n\n## Done when\n{{done_when}}\n{{failure}}\n"

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
	}
	for _, item := range cases {
		if got := MatchGlob(item.pattern, item.path); got != item.want {
			t.Errorf("MatchGlob(%q, %q) = %v", item.pattern, item.path, got)
		}
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
