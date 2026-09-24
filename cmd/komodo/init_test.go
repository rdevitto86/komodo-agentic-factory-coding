package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// starterFiles are the paths init writes into an empty repo.
var starterFiles = []string{
	"AGENTS.md", "BACKLOG.md", "CHANGELOG.md",
	"docs/spec/prd.md", "docs/spec/architecture.md", "docs/spec/system-design.md",
	".github/PULL_REQUEST_TEMPLATE.md", ".komodo/context/example.md",
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

// TestInitWritesEveryStarterAndTheBacklogLintsClean proves a fresh repo is ready for the line.
func TestInitWritesEveryStarterAndTheBacklogLintsClean(t *testing.T) {
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
	if first.code != 0 || !strings.Contains(first.stdout, "keep AGENTS.md\n") {
		t.Fatalf("first init exited %d: %s%s", first.code, first.stdout, first.stderr)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); !bytes.Equal(data, mine) {
		t.Fatalf("AGENTS.md changed:\n%s", data)
	}
	second := runCLI(t, root, "", "init")
	if second.code != 0 || strings.Contains(second.stdout, "create ") {
		t.Fatalf("second init exited %d or created a file:\n%s", second.code, second.stdout)
	}
	for _, file := range starterFiles {
		if !strings.Contains(second.stdout, "keep "+file+"\n") {
			t.Fatalf("second init printed no keep %s:\n%s", file, second.stdout)
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
	if err := os.Mkdir(filepath.Join(root, "BACKLOG.md"), 0o755); err != nil {
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
		"keep AGENTS.md\n", "keep BACKLOG.md\n", "create CHANGELOG.md\n",
		"skip docs/spec/prd.md: outside the repo\n",
		"skip docs/spec/architecture.md: outside the repo\n",
		"skip docs/spec/system-design.md: outside the repo\n",
	}
	for _, line := range want {
		if !strings.Contains(got.stdout, line) {
			t.Fatalf("init printed no %q:\n%s", strings.TrimSpace(line), got.stdout)
		}
	}
	for _, dir := range []string{outside, filepath.Join(root, "BACKLOG.md")} {
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
