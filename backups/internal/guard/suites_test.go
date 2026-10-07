package guard

import (
	"strings"
	"testing"
)

// TestEverySuiteHasRows proves every registered role suite, global included, carries at least
// one table row; a suite with none would never be exercised by RunTable or Report.
func TestEverySuiteHasRows(t *testing.T) {
	if len(Global().Rows) == 0 {
		t.Fatal("the global suite has no rows")
	}
	for _, name := range Names() {
		suite, ok := For(name)
		if !ok {
			t.Fatalf("%s has no registered suite", name)
		}
		if len(suite.Rows) == 0 {
			t.Fatalf("%s has no table rows", name)
		}
	}
}

// TestForNamesTheOrchestratorByDefault proves an empty role and "orchestrator" resolve the same suite.
func TestForNamesTheOrchestratorByDefault(t *testing.T) {
	empty, ok := For("")
	if !ok {
		t.Fatal("the empty role has no suite")
	}
	named, ok := For("orchestrator")
	if !ok {
		t.Fatal("orchestrator has no suite")
	}
	if empty.Name != named.Name {
		t.Fatalf("For(\"\") = %q, For(\"orchestrator\") = %q", empty.Name, named.Name)
	}
}

// TestGlobalAlwaysApplies proves a critical-ref push is refused under every role's suite, the
// orchestrator included, since the global suite runs first and no role escapes it.
func TestGlobalAlwaysApplies(t *testing.T) {
	root := worktree(t)
	for _, role := range append([]string{""}, Names()...) {
		t.Setenv(RoleEnv, role)
		request := bashRequest(root, "git push origin main")
		if !Check(request, DefaultPolicy(), "feat/x").Deny {
			t.Fatalf("role %q escaped the global push-to-main rule", role)
		}
	}
}

// TestARoleMayNotRunAnothersCommand proves a role calling a komodo subcommand its own suite does
// not list is refused, the orchestrator's own commands most of all.
func TestARoleMayNotRunAnothersCommand(t *testing.T) {
	root := worktree(t)
	cases := []struct {
		role, command string
		deny          bool
	}{
		{"builder", "komodo run", true},
		{"builder", "komodo check task", false},
		{"reviewer", "komodo check findings", false},
		{"reviewer", "komodo run", true},
		{"planner", "komodo lint", false},
		{"planner", "komodo run", true},
		{"", "komodo run", false},
	}
	for _, item := range cases {
		t.Setenv(RoleEnv, item.role)
		decision := Check(bashRequest(root, item.command), DefaultPolicy(), "feat/x")
		if decision.Deny != item.deny {
			t.Fatalf("role %q running %q: deny = %v, want %v (%v)", item.role, item.command, decision.Deny, item.deny, decision.Findings)
		}
	}
}

// TestReadOnlyRolesNeverWrite proves every read-only role is refused a file write and a git commit.
func TestReadOnlyRolesNeverWrite(t *testing.T) {
	root := worktree(t)
	for role := range readOnlyRoles {
		t.Setenv(RoleEnv, role)
		write := Request{ToolName: "Write", Cwd: root, ToolInput: map[string]any{"file_path": "x.go"}}
		if !Check(write, DefaultPolicy(), "feat/x").Deny {
			t.Fatalf("role %q was allowed a file write", role)
		}
		commit := bashRequest(root, "git commit -m x")
		decision := Check(commit, DefaultPolicy(), "feat/x")
		if !decision.Deny || !containsAny(decision.Findings, "read-only") {
			t.Fatalf("role %q was allowed a git commit: %v", role, decision.Findings)
		}
	}
}

// TestBuilderIsNotReadOnly proves the one writing role is not swept into the read-only set.
func TestBuilderIsNotReadOnly(t *testing.T) {
	if readOnlyRoles["builder"] {
		t.Fatal("builder is read-only")
	}
}

// TestNamesListsEveryRoleOnce proves Names holds no duplicate and every name resolves.
func TestNamesListsEveryRoleOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range Names() {
		if seen[name] {
			t.Fatalf("%s is named twice", name)
		}
		seen[name] = true
		if _, ok := For(name); !ok {
			t.Fatalf("%s has no suite", name)
		}
	}
	if !strings.Contains(strings.Join(Names(), ","), "orchestrator") {
		t.Fatal("orchestrator is not named")
	}
}
