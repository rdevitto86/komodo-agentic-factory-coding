package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
)

// sampleGroupFile is the one group file init writes into docs/backlog/.
const sampleGroupFile = "docs/backlog/TG-01.1-example-group.md"

// starterFiles are the paths init writes into an empty repo.
var starterFiles = []string{
	"AGENTS.md", "CHANGELOG.md", sampleGroupFile,
	"docs/prd.md", "docs/architecture.md", "docs/system-design.md", "docs/decisions.md",
	".github/PULL_REQUEST_TEMPLATE.md", ".komodo/context/example.md", ".gitattributes",
}

// emptyRepo is a fresh git repo holding nothing.
func emptyRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "auth-api")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q")
	return root
}

// TestInitWritesEveryStarterAndTheSampleGroupFileParsesClean proves a fresh repo is ready for the line.
func TestInitWritesEveryStarterAndTheSampleGroupFileParsesClean(t *testing.T) {
	root := emptyRepo(t)
	got := runCLI(t, root, "", "init", "--name", "Auth API")
	if got.code != 0 {
		t.Fatalf("init exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	for _, file := range starterFiles {
		if !strings.Contains(got.stdout, "create "+file+"\n") {
			t.Fatalf("init printed no create %s:\n%s", file, got.stdout)
		}
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "{{") || strings.Contains(string(data), "<Repo Name>") {
			t.Fatalf("%s keeps an unfilled placeholder:\n%s", file, data)
		}
	}
	agents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if !strings.HasPrefix(string(agents), "# Auth API\n") {
		t.Fatalf("AGENTS.md does not carry the name:\n%s", agents)
	}
	if !strings.Contains(got.stdout, "komodo install --host ") || !strings.Contains(got.stdout, "komodo lint") {
		t.Fatalf("init printed no next commands:\n%s", got.stdout)
	}
	data, err := os.ReadFile(filepath.Join(root, sampleGroupFile))
	if err != nil {
		t.Fatal(err)
	}
	group := backlog.ParseGroupFile(string(data))
	if len(group.Problems) != 0 {
		t.Fatalf("the sample group file does not parse clean: %v", group.Problems)
	}
	lint := runCLI(t, root, "", "lint")
	if lint.code != 0 || !strings.Contains(lint.stdout, " 0 problem(s)") {
		t.Fatalf("lint on the written backlog exited %d: %s", lint.code, lint.stdout)
	}
}

// TestInitTwiceCreatesNothingAndKeepsAnExistingFile proves init never overwrites.
func TestInitTwiceCreatesNothingAndKeepsAnExistingFile(t *testing.T) {
	root := emptyRepo(t)
	mine := []byte("# Mine\n\nhand written\n")
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), mine, 0o644); err != nil {
		t.Fatal(err)
	}
	first := runCLI(t, root, "", "init")
	wantMissing := "keep AGENTS.md: missing Quick reference, Commands, Harness, Where things are, Gotchas, Deviations\n"
	if first.code != 0 || !strings.Contains(first.stdout, wantMissing) {
		t.Fatalf("first init exited %d, want %q:\n%s%s", first.code, wantMissing, first.stdout, first.stderr)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); !bytes.Equal(data, mine) {
		t.Fatalf("AGENTS.md changed:\n%s", data)
	}
	second := runCLI(t, root, "", "init")
	if second.code != 0 || strings.Contains(second.stdout, "create ") {
		t.Fatalf("second init exited %d or created a file:\n%s", second.code, second.stdout)
	}
	for _, file := range starterFiles {
		want := "keep " + file + "\n"
		if file == "AGENTS.md" {
			want = wantMissing
		}
		if !strings.Contains(second.stdout, want) {
			t.Fatalf("second init printed no %q for %s:\n%s", want, file, second.stdout)
		}
	}
}

// TestInitNeverWritesThroughASymlinkOrADirectory proves init keeps links and directories and never writes outside the repo.
func TestInitNeverWritesThroughASymlinkOrADirectory(t *testing.T) {
	root := emptyRepo(t)
	outside := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "missing.md"), filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "CHANGELOG.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "init")
	if got.code != 0 {
		t.Fatalf("init exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	want := []string{
		"keep AGENTS.md\n", "keep CHANGELOG.md\n",
		"skip docs/prd.md: outside the repo\n",
		"skip docs/architecture.md: outside the repo\n",
		"skip docs/system-design.md: outside the repo\n",
		"skip docs/decisions.md: outside the repo\n",
		"skip " + sampleGroupFile + ": outside the repo\n",
	}
	for _, line := range want {
		if !strings.Contains(got.stdout, line) {
			t.Fatalf("init printed no %q:\n%s", strings.TrimSpace(line), got.stdout)
		}
	}
	for _, dir := range []string{outside, filepath.Join(root, "CHANGELOG.md")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("init wrote %d entries into %s", len(entries), dir)
		}
	}
}

// TestInitWithoutANameUsesTheDirectoryAndTheDetectedLanguage proves the defaults come from the repo itself.
func TestInitWithoutANameUsesTheDirectoryAndTheDetectedLanguage(t *testing.T) {
	root := emptyRepo(t)
	for file, body := range map[string]string{"go.mod": "module auth\n", "main.go": "package main\n"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := runCLI(t, root, "", "init")
	if got.code != 0 {
		t.Fatalf("init exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# auth-api\n", "| Language | Go |", "`go test ./...`", "| Build | `go build ./...` |",
	} {
		if !strings.Contains(string(agents), want) {
			t.Fatalf("AGENTS.md holds no %q:\n%s", want, agents)
		}
	}
}

// TestMissingSectionsNamesOnlyWhatTheKeptBodyLacks proves the diff is per section, in the template's order.
func TestMissingSectionsNamesOnlyWhatTheKeptBodyLacks(t *testing.T) {
	template := "# T\n\n## One\n\n## Two\n\n## Three\n"
	kept := "# Mine\n\n## Two\n\nbody\n"
	got := missingSections(template, kept)
	if len(got) != 2 || got[0] != "One" || got[1] != "Three" {
		t.Fatalf("missing = %v, want [One Three]", got)
	}
}

// TestMissingSectionsIgnoresAnUnfilledPlaceholderHeading proves a dynamic section, such as a changelog's
// dated release heading, is never reported missing.
func TestMissingSectionsIgnoresAnUnfilledPlaceholderHeading(t *testing.T) {
	template := "# Changelog\n\n## [0.1.0] — {{DATE}}\n"
	kept := "# Changelog\n\n## [0.1.0] — 2024-01-01\n"
	if got := missingSections(template, kept); len(got) != 0 {
		t.Fatalf("missing = %v, want none for a placeholder heading", got)
	}
}

// TestInitLintsAnAdoptedReposExistingBacklog proves init reports a broken backlog it kept, not just wrote.
func TestInitLintsAnAdoptedReposExistingBacklog(t *testing.T) {
	root := emptyRepo(t)
	broken := "## [TG-01.1] A group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 0.1.0\n```\n\n" +
		"- [ ] a task with no bold TSK id\n"
	if err := os.MkdirAll(filepath.Join(root, "docs", "backlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "backlog", "TG-01.1-a.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "init")
	if got.code != 0 {
		t.Fatalf("init exited %d: %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "problem(s)") {
		t.Fatalf("init printed no backlog problems for the broken group file:\n%s", got.stdout)
	}
}

// TestStarterPullRequestTemplateMatchesTheRepos keeps the shipped copy in step with this repo's own.
func TestStarterPullRequestTemplateMatchesTheRepos(t *testing.T) {
	ours, err := os.ReadFile("../../.github/PULL_REQUEST_TEMPLATE.md")
	if err != nil {
		t.Fatal(err)
	}
	shipped, err := os.ReadFile("../../templates/project/.github/PULL_REQUEST_TEMPLATE.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ours, shipped) {
		t.Fatal("templates/project/.github/PULL_REQUEST_TEMPLATE.md drifted from .github/PULL_REQUEST_TEMPLATE.md")
	}
}
