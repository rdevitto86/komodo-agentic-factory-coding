package claude

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"komodo/internal/glob"
	"komodo/internal/guard"
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

// TestABuilderWithAShellIsDeniedBranchPinningGitCalls proves a builder never runs the git calls
// that move or pin a branch; only komodo worktree add does (decision 0012).
func TestABuilderWithAShellIsDeniedBranchPinningGitCalls(t *testing.T) {
	req := mount.StartRequest{Role: "builder", Tools: []string{"read", "edit", "write", "shell", "search"}}
	_, deny := rolePermissions(t.TempDir(), goRepo(t), req)
	for _, rule := range []string{"Bash(git update-ref:*)", "Bash(git symbolic-ref:*)", "Bash(git worktree:*)"} {
		if !slices.Contains(deny, rule) {
			t.Fatalf("deny = %v, missing %q", deny, rule)
		}
	}
}

func TestTheGoldenSuiteDenyNeverReachesANestedEvalPackage(t *testing.T) {
	_, deny := rolePermissions(t.TempDir(), "/repo", mount.StartRequest{Role: "builder", Tools: []string{"read"}})
	for _, rule := range deny {
		if strings.Contains(rule, "eval/**") && rule != "Edit(//repo/eval/**)" {
			t.Fatalf("deny %q is not pinned to the root's eval/, so it also refuses internal/eval/", rule)
		}
	}
}

func TestEveryLineRoleIsDeniedThePRDTheGoldenSuiteAndThePolicy(t *testing.T) {
	worktree := t.TempDir()
	root := "//" + strings.TrimPrefix(filepath.ToSlash(worktree), "/")
	for _, role := range []string{"builder", "reviewer", "lens"} {
		_, deny := rolePermissions(t.TempDir(), worktree, mount.StartRequest{Role: role, Tools: []string{"read"}})
		for _, rule := range []string{"Edit(" + root + "/docs/prd.md)", "Edit(" + root + "/eval/**)", "Edit(" + root + "/komodo/policy.json)"} {
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

// TestADenyPatternOutrunsAMatchingAllow proves this host's deny still matches a path an allow also
// names, so a broad deny must be narrowed itself, never papered over with a matching allow.
func TestADenyPatternOutrunsAMatchingAllow(t *testing.T) {
	home := t.TempDir()
	memory := filepath.Join(home, Dir, "projects", "x", "memory", "notes.md")
	allow := []string{"Edit(" + memory + ")"}
	deny := []string{"Edit(" + filepath.Join(home, Dir) + "/**)"}
	// This host's own rule, not this repo's: a denied path is refused even when an allow also names it.
	denied := glob.Match(strings.TrimSuffix(strings.TrimPrefix(deny[0], "Edit("), ")"), memory)
	allowed := slices.Contains(allow, "Edit("+memory+")")
	if !(denied && allowed) {
		t.Fatalf("denied = %v, allowed = %v; want both true, so the deny still wins", denied, allowed)
	}
}

// TestRenderedDenyNeverCoversTheProjectMemoryDirectory proves the rendered deny stops short of the
// project memory directory WritePaths grants, while still naming the host's own known paths.
func TestRenderedDenyNeverCoversTheProjectMemoryDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "work", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	memory := strings.TrimSuffix(WritePaths(root)[0], "/**") + "/notes.md"
	deny := denyList(root, policyFile{})
	for _, rule := range deny {
		pattern := strings.TrimSuffix(strings.TrimPrefix(rule, "Edit("), ")")
		if glob.Match(expandHome(t, pattern, home), memory) {
			t.Fatalf("deny rule %q still covers the project memory file %q", rule, memory)
		}
	}
	for _, want := range []string{
		"Edit(~/.claude/*)", "Edit(~/.claude/CLAUDE.md)", "Edit(~/.claude/agents/**)",
		"Edit(~/.claude/skills/**)", "Edit(~/.claude/hooks/**)",
	} {
		if want == "Edit(~/.claude/CLAUDE.md)" {
			// CLAUDE.md itself is covered by the top-level Edit(~/.claude/*) rule, not named apart.
			if !slices.Contains(deny, "Edit(~/.claude/*)") {
				t.Fatalf("deny = %v, missing the top-level rule that covers %s", deny, want)
			}
			continue
		}
		if !slices.Contains(deny, want) {
			t.Fatalf("deny = %v, missing %q", deny, want)
		}
	}
}

// TestGuardStillRefusesTheHostsOwnConfigFiles is a regression proving the guard's own ConfigPaths
// still refuses CLAUDE.md, agents/ and hooks/, unlike the rendered settings deny.
func TestGuardStillRefusesTheHostsOwnConfigFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := repoWith(t, map[string]string{"go.mod": "module x\n"})
	policy := guard.Load(repo, repo)
	for _, rel := range []string{"CLAUDE.md", filepath.Join("agents", "builder.md"), filepath.Join("hooks", "guard.py")} {
		path := filepath.Join(home, Dir, rel)
		if !policy.IsConfigPath(path, repo) {
			t.Errorf("%s is writable; the guard's own config paths must stay the whole home tree", path)
		}
	}
}

// expandHome turns a rule's leading ~ into home, the form WritePaths and ConfigPaths both compare against.
func expandHome(t *testing.T, pattern, home string) string {
	t.Helper()
	return strings.Replace(pattern, "~", home, 1)
}
