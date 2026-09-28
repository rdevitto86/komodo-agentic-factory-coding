package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/profile"
)

const requestsBuilderRole = "---\nname: builder\ndescription: Writes code.\ntier: standard\n" +
	"tools: [read, edit, write, shell, search]\nsession: true\nreturns: builder.schema.json\n---\n\n" +
	"You are a builder.\n\nTask {{task_id}}: {{title}}\n\n{{task_block}}\n{{repo_rules}}\n{{repo_context}}\n" +
	"{{context}}\n{{files}}\n{{repo_profile}}\n{{standards}}\n{{done_when}}\n{{failure}}\n"

const requestsReviewerRole = "---\nname: reviewer\ndescription: Reviews.\ntier: heavy\n" +
	"tools: [read, search]\nsession: true\nreturns: reviewer.schema.json\n---\n\n" +
	"You are a reviewer.\n\n{{group_id}}: {{title}}\n\n{{tasks}}\n\n{{standards}}\n\n{{diff}}\n\nBase: {{base}}\n"

const requestsSchema = `{"type":"object","required":["result"]}`

const requestsBacklog = "### [TG-20.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-20.1.1] First task [P: C] [READY]\n```yaml\nfiles: [a/one.go]\n" +
	"done_when:\n  - go test ./a/...\n```\n\n" +
	"#### [TSK-20.1.2] Second task [P: C] [READY]\n```yaml\nfiles: [b/two.go]\n" +
	"done_when:\n  - go test ./b/...\n```\n"

// requestsRepo builds a git repo with a two-task group, a builder and reviewer role, and their
// schemas, then commits a change to a/one.go so the reviewer request carries a real diff.
func requestsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "test")
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("BACKLOG.md", requestsBacklog)
	write(filepath.Join(line.RolesDir, "builder.md"), requestsBuilderRole)
	write(filepath.Join(line.RolesDir, "builder.schema.json"), requestsSchema)
	write(filepath.Join(line.RolesDir, "reviewer.md"), requestsReviewerRole)
	write(filepath.Join(line.RolesDir, "reviewer.schema.json"), requestsSchema)
	write("a/one.go", "package a\n\nfunc One() int { return 1 }\n")
	write("b/two.go", "package b\n\nfunc Two() {}\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "base")
	write("a/one.go", "package a\n\n// One returns one.\nfunc One() int { return 1 }\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "the change")
	return root
}

// requestsPlan is the two-task group both the builder and reviewer request read from.
func requestsPlan() *line.Plan {
	return &line.Plan{
		Group: "TG-20.1", Title: "A group", Base: "main~1", Branch: "main", Worktree: ".",
		Tasks: []line.PlanTask{{ID: "TSK-20.1.1", Title: "First task"}, {ID: "TSK-20.1.2", Title: "Second task"}},
		Waves: [][]string{{"TSK-20.1.1", "TSK-20.1.2"}},
		Profile: profile.Profile{
			Roles: map[string]profile.RoleProfile{"builder": {Tier: "standard", Effort: "high"}},
			Tiers: mount.Tiers{
				Standard: mount.Machine{Provider: "anthropic", Model: "claude-sonnet-4"},
				Reviewer: mount.Machine{Provider: "anthropic", Model: "claude-opus-4", Effort: "high"},
			},
		},
	}
}

func TestStartRequestsNameEveryTaskAndTheBuilderSchema(t *testing.T) {
	root := requestsRepo(t)
	req, err := BuilderRequest(root, requestsPlan())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TSK-20.1.1", "TSK-20.1.2"} {
		if !strings.Contains(req.Brief, want) {
			t.Errorf("builder brief is missing %q:\n%s", want, req.Brief)
		}
	}
	if req.Role != "builder" {
		t.Fatalf("role = %q", req.Role)
	}
	if strings.Join(req.Tools, ",") != "read,edit,write,shell,search" {
		t.Fatalf("tools = %v", req.Tools)
	}
	if string(req.Schema) != requestsSchema {
		t.Fatalf("schema = %q", req.Schema)
	}
	if req.Model != "claude-sonnet-4" || req.Effort != "high" {
		t.Fatalf("model = %q, effort = %q", req.Model, req.Effort)
	}
}

func TestReReviewInputCarriesTheDiffSinceTheReviewedCommitAndTheOpenFindings(t *testing.T) {
	root := requestsRepo(t)
	reviewed := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	path := filepath.Join(root, "b", "two.go")
	if err := os.WriteFile(path, []byte("package b\n\n// Two does nothing.\nfunc Two() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "commit", "-q", "-am", "the repair")
	s := conductor.State{Reviewed: reviewed, Findings: []conductor.Finding{
		{Severity: "high", Verified: true, File: "b/two.go", Line: 3, Title: "Two has no comment"},
		{Severity: "low", File: "a/one.go", Line: 1, Title: "below the floor"},
	}}
	input, err := ReReviewInput(root, requestsPlan(), s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input, "Two does nothing") || strings.Contains(input, "One returns one") {
		t.Fatalf("re-review input = %q, want only the repair's diff", input)
	}
	if !strings.Contains(input, "`b/two.go:3` high: Two has no comment") || strings.Contains(input, "below the floor") {
		t.Fatalf("re-review input = %q, want only the open finding by file and line", input)
	}
}

func TestStartRequestsCarryTheGroupsDiffForTheReviewer(t *testing.T) {
	root := requestsRepo(t)
	req, err := ReviewerRequest(root, requestsPlan())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(req.Brief, "One returns one") {
		t.Fatalf("reviewer brief is missing the group's diff:\n%s", req.Brief)
	}
	if req.Role != "reviewer" {
		t.Fatalf("role = %q", req.Role)
	}
	if strings.Join(req.Tools, ",") != "read,search" {
		t.Fatalf("tools = %v", req.Tools)
	}
	if string(req.Schema) != requestsSchema {
		t.Fatalf("schema = %q", req.Schema)
	}
	if req.Model != "claude-opus-4" || req.Effort != "high" {
		t.Fatalf("model = %q, effort = %q", req.Model, req.Effort)
	}
}
