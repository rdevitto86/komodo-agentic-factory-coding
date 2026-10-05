package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRuleProblemsHoldsEachRuleFileToTheNeverBudget(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "komodo", "AGENTS.md"), "- **Act, then report.** Never stop. Never ask. Never wait.\n", 0o644)
	writeFile(t, filepath.Join(root, "komodo", "roles", "builder.md"), "Never widen a type.\n", 0o644)
	got := RuleProblems(root)
	if len(got) != 1 || !strings.Contains(got[0], "komodo/AGENTS.md: says never 3 times") {
		t.Fatalf("problems = %q, want only AGENTS.md over the budget", got)
	}
}

func TestTheShippedRulesMeetTheNeverBudget(t *testing.T) {
	if got := RuleProblems(filepath.Join("..", "..")); len(got) != 0 {
		t.Fatalf("problems = %q; the rules every agent reads must say what to do", got)
	}
}
