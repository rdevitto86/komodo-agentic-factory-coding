package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// repoOn builds the least .git that git reads as a repo on branch, with no config or hooks to write.
func repoOn(t *testing.T, branch string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"objects", filepath.Join("refs", "heads")} {
		if err := os.MkdirAll(filepath.Join(root, ".git", dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	head := []byte("ref: refs/heads/" + branch + "\n")
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), head, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = root
	if out, err := cmd.Output(); err != nil || string(out) != branch+"\n" {
		t.Fatalf("git reads the fixture's branch as %q: %v", out, err)
	}
	return root
}

// toolkitWithoutPolicyPath builds a toolkit whose shipped policy keeps the paths every session is refused.
func toolkitWithoutPolicyPath(t *testing.T) string {
	t.Helper()
	toolkit := t.TempDir()
	if err := os.MkdirAll(filepath.Join(toolkit, "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	shipped := `{"critical_refs":["main","master"],` +
		`"config_paths":["~/.komodo/**","**/.git/config","**/.git/hooks/**","bin/**",".komodo/policy.json"]}`
	if err := os.WriteFile(filepath.Join(toolkit, "komodo", "policy.json"), []byte(shipped), 0o644); err != nil {
		t.Fatal(err)
	}
	return toolkit
}

func TestOnlyTheOrchestratorOnABranchEditsTheShippedPolicy(t *testing.T) {
	registerFakeHost()
	toolkit := toolkitWithoutPolicyPath(t)
	cases := []struct {
		name, role, branch string
		deny               bool
	}{
		{"the orchestrator on feat/x", "", "feat/x", false},
		{"a builder on feat/x", "builder", "feat/x", true},
		{"the orchestrator on main", "", "main", true},
		{"a builder on main", "builder", "main", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(RoleEnv, tc.role)
			repo := repoOn(t, tc.branch)
			request := Request{
				HookEventName: "PreToolUse", ToolName: "Edit", Cwd: repo,
				ToolInput: map[string]any{"file_path": filepath.Join(repo, "komodo", "policy.json")},
			}
			decision := Check(request, Load(toolkit, repo), tc.branch)
			if decision.Deny != tc.deny {
				t.Fatalf("deny = %v, want %v: %v", decision.Deny, tc.deny, decision.Findings)
			}
		})
	}
}

// TestTheShippedPolicyProtectsEveryCriticalRef proves komodo/policy.json names every ref AGENTS.md forbids.
func TestTheShippedPolicyProtectsEveryCriticalRef(t *testing.T) {
	policy := Load(t.TempDir(), t.TempDir())
	for _, ref := range []string{"main", "master", "trunk", "prod", "production", "release/2.0", "hotfix/urgent"} {
		if !policy.IsCritical(ref) {
			t.Errorf("%s is not protected", ref)
		}
	}
}

func TestEverySessionIsRefusedTheHostsGitAndBinaryPaths(t *testing.T) {
	registerFakeHost()
	toolkit := toolkitWithoutPolicyPath(t)
	for _, role := range []string{"", "builder"} {
		t.Run("role "+role, func(t *testing.T) {
			t.Setenv(RoleEnv, role)
			repo := repoOn(t, "feat/x")
			policy := Load(toolkit, repo)
			for _, rel := range []string{"bin/komodo", ".git/config", ".git/hooks/pre-commit"} {
				if !policy.IsConfigPath(filepath.Join(repo, rel), repo) {
					t.Errorf("%s is writable on feat/x", rel)
				}
			}
		})
	}
}

// TestASharedBranchClaimIsAGlobalTierCheckNotOnlyALineRole documents that a claim check runs for
// every session, so IsLineSession alone must never be the only gate on it.
func TestASharedBranchClaimIsAGlobalTierCheckNotOnlyALineRole(t *testing.T) {
	t.Setenv(RoleEnv, "")
	if IsLineSession() {
		t.Fatal("the orchestrator carries no role; a claim check gated on IsLineSession would skip it")
	}
	t.Setenv(RoleEnv, "builder")
	if !IsLineSession() {
		t.Fatal("a builder carries a role; a claim check still applies to it as well")
	}
}
