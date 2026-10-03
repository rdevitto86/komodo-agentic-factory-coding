package guard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		{"the orchestrator on an epic branch", "", "feat/1.0.0-alpha.7", true},
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

// detachedWorktreeOn cuts a detached worktree of root tracking branch via komodo.branch config,
// as komodo worktree add leaves one.
func detachedWorktreeOn(t *testing.T, root, branch string) string {
	t.Helper()
	runGit := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	runGit(root, "config", "extensions.worktreeConfig", "true")
	path := t.TempDir() + "-wt"
	runGit(root, "worktree", "add", "--detach", path)
	runGit(path, "config", "--worktree", "komodo.branch", branch)
	return path
}

// TestOnFeatureBranchReadsADetachedWorktreesTrackedBranch proves the orchestrator may edit
// komodo/policy.json in a detached worktree tracking a feature branch, not only an attached one.
func TestOnFeatureBranchReadsADetachedWorktreesTrackedBranch(t *testing.T) {
	registerFakeHost()
	toolkit := toolkitWithoutPolicyPath(t)
	root := gitRepo(t, "main")
	detached := detachedWorktreeOn(t, root, "feat/x")
	request := Request{
		HookEventName: "PreToolUse", ToolName: "Edit", Cwd: detached,
		ToolInput: map[string]any{"file_path": filepath.Join(detached, "komodo", "policy.json")},
	}
	decision := Check(request, Load(toolkit, detached), "feat/x")
	if decision.Deny {
		t.Fatalf("the orchestrator in a detached worktree on feat/x is refused: %v", decision.Findings)
	}
}

// TestTheShippedPolicyProtectsEveryCriticalRef proves komodo/policy.json names every ref AGENTS.md forbids.
func TestTheShippedPolicyProtectsEveryCriticalRef(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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

// TestAMalformedRepoPolicyPanicsNamingItsPath proves a present but broken .komodo/policy.json
// fails loudly instead of silently falling back to the shipped defaults.
func TestAMalformedRepoPolicyPanicsNamingItsPath(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, ".komodo", "policy.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("a malformed repo policy did not panic")
		}
		if !strings.Contains(fmt.Sprint(r), path) {
			t.Fatalf("panic = %v, want it to name %s", r, path)
		}
	}()
	Load(t.TempDir(), repo)
}

// TestARepoPolicyWithAnUnknownFieldPanics proves the decode is strict, not merely tolerant of bad JSON.
func TestARepoPolicyWithAnUnknownFieldPanics(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, ".komodo", "policy.json")
	if err := os.WriteFile(path, []byte(`{"not_a_real_field":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("an unknown field in the repo policy did not panic")
		}
	}()
	Load(t.TempDir(), repo)
}

// TestAMalformedMachineOverlayPanicsReachingLoad proves a broken ~/.komodo/config.json fails
// loudly through Load too, the same as a broken policy file.
func TestAMalformedMachineOverlayPanicsReachingLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".komodo", "config.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("a malformed machine overlay did not panic")
		}
		if !strings.Contains(fmt.Sprint(r), path) {
			t.Fatalf("panic = %v, want it to name %s", r, path)
		}
	}()
	Load(t.TempDir(), t.TempDir())
}

// TestAWriteUnderTheUsersKomodoDirIsRefused proves the shipped policy's ~/.komodo/** rule reaches
// a real write, under the session's own home, on every platform the guard's paths run on.
func TestAWriteUnderTheUsersKomodoDirIsRefused(t *testing.T) {
	registerFakeHost()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(RoleEnv, "builder")
	root := repoOn(t, "feat/x")
	target := filepath.Join(home, ".komodo", "recall.json")
	request := Request{
		HookEventName: "PreToolUse", ToolName: "Write", Cwd: root,
		ToolInput: map[string]any{"file_path": target},
	}
	decision := Check(request, DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatalf("a write under the user's own ~/.komodo/ was allowed: %v", decision.Findings)
	}
}
