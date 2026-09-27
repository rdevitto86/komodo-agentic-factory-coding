package claude

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"komodo/internal/mount"
)

// repoWith builds a worktree holding files, with its own .git so no walk up reaches the real one.
func repoWith(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// goRepo is a worktree detection reads as a Go module.
func goRepo(t *testing.T) string {
	return repoWith(t, map[string]string{"go.mod": "module x\n", "main.go": "package main\n"})
}

// typeScriptRepo is a worktree detection reads as an npm TypeScript package.
func typeScriptRepo(t *testing.T) string {
	return repoWith(t, map[string]string{"package.json": "{}\n", "src/index.ts": "export {}\n"})
}

func TestTheBuilderIsAllowedItsRepoCommandsAndNoBareShell(t *testing.T) {
	cases := []struct {
		name     string
		worktree func(*testing.T) string
		want     []string
		absent   []string
	}{
		{
			"Go", goRepo,
			[]string{"Bash(go test:*)", "Bash(go build:*)", "Bash(go vet:*)", "Bash(gofmt:*)"},
			[]string{"Bash(npx eslint:*)"},
		},
		{
			"TypeScript", typeScriptRepo,
			[]string{"Bash(npm test:*)", "Bash(npm run build:*)", "Bash(npx eslint:*)", "Bash(npx prettier:*)"},
			[]string{"Bash(go test:*)"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := mount.StartRequest{Role: "builder", Tools: []string{"read", "edit", "write", "shell", "search"}}
			allow, _ := rolePermissions(t.TempDir(), tc.worktree(t), req)
			always := []string{"Read", "Edit", "Write", "Grep", "Glob", "Bash(git diff:*)", "Bash(komodo check:*)"}
			for _, rule := range append(always, tc.want...) {
				if !slices.Contains(allow, rule) {
					t.Errorf("allow = %v, missing %q", allow, rule)
				}
			}
			for _, rule := range append([]string{"Bash"}, tc.absent...) {
				if slices.Contains(allow, rule) {
					t.Errorf("allow = %v, must not carry %q", allow, rule)
				}
			}
		})
	}
}

func TestTheReviewerIsAllowedOnlyReadsAndReadOnlyGit(t *testing.T) {
	req := mount.StartRequest{Role: "reviewer", Tools: []string{"read", "search"}}
	allow, deny := rolePermissions(t.TempDir(), goRepo(t), req)
	want := []string{
		"Read", "Grep", "Glob",
		"Bash(git status:*)", "Bash(git diff:*)", "Bash(git log:*)", "Bash(git show:*)", "Bash(git blame:*)",
	}
	if !slices.Equal(allow, want) {
		t.Fatalf("allow = %v, want %v", allow, want)
	}
	if !slices.Contains(deny, "Bash(git commit:*)") || !slices.Contains(deny, "Bash(git push:*)") {
		t.Fatalf("deny = %v; a reviewer with a shell is refused git writes", deny)
	}
}

func TestEveryLineRoleIsDeniedThePRDTheGoldenSuiteAndThePolicy(t *testing.T) {
	for _, role := range []string{"builder", "reviewer", "lens"} {
		_, deny := rolePermissions(t.TempDir(), t.TempDir(), mount.StartRequest{Role: role, Tools: []string{"read"}})
		for _, rule := range []string{"Edit(docs/prd.md)", "Edit(eval/**)", "Edit(komodo/policy.json)"} {
			if !slices.Contains(deny, rule) {
				t.Errorf("%s deny = %v, missing %q", role, deny, rule)
			}
		}
	}
	_, deny := rolePermissions(t.TempDir(), t.TempDir(), mount.StartRequest{Role: "lens", Tools: []string{"read"}})
	if slices.Contains(deny, "Bash(git commit:*)") {
		t.Errorf("lens deny = %v; a role with no shell needs no git rules", deny)
	}
}

func TestARoleNamingCommandClassesRunsThemInAShellItDoesNotDeclare(t *testing.T) {
	req := mount.StartRequest{Role: "reviewer", Tools: []string{"read", "search"}, Schema: []byte(`{}`)}
	argv, _, _ := Session(t.TempDir(), goRepo(t), req, "", "", "m", "", 10, 0)
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--tools Read, Grep, Glob, Bash", "Bash(git diff:*)", "Bash(git commit:*)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q:\n%s", want, joined)
		}
	}
}

func TestARoleNamingNoCommandClassesKeepsItsWholeShell(t *testing.T) {
	allow, _ := rolePermissions(t.TempDir(), goRepo(t), mount.StartRequest{Role: "lens", Tools: []string{"read", "shell"}})
	if !slices.Equal(allow, []string{"Read", "Bash"}) {
		t.Fatalf("allow = %v, want the role's tools unchanged", allow)
	}
}

func TestADetectedVerifyScriptIsAllowedWholeNotByItsInterpreter(t *testing.T) {
	worktree := repoWith(t, map[string]string{"go.mod": "module x\n", ".komodo/verify.sh": "go test ./...\n"})
	allow, _ := rolePermissions(t.TempDir(), worktree, mount.StartRequest{Role: "builder", Tools: []string{"shell"}})
	if !slices.Contains(allow, "Bash(sh .komodo/verify.sh:*)") {
		t.Fatalf("allow = %v, missing the repo's verify script", allow)
	}
	if slices.Contains(allow, "Bash(sh:*)") {
		t.Fatalf("allow = %v; a bare interpreter runs anything", allow)
	}
}
