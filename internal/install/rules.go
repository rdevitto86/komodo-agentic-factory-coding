package install

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// neverBudget is how many times one behavioral rule file may say "never"; a rule reads better as what to do.
const neverBudget = 2

// never matches the word in any case.
var never = regexp.MustCompile(`(?i)\bnever\b`)

// RuleProblems names each always-on rule or role file over the never budget: the rules every agent reads
// should say what to do, since stacked prohibitions read as reasons to stop and ask.
func RuleProblems(root string) []string {
	files := []string{filepath.Join(root, "komodo", "AGENTS.md"), filepath.Join(root, "komodo", "rules", "accessibility.md")}
	roles, _ := filepath.Glob(filepath.Join(root, "komodo", "roles", "*.md"))
	var problems []string
	for _, path := range append(files, roles...) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if count := len(never.FindAll(data, -1)); count > neverBudget {
			rel, _ := filepath.Rel(root, path)
			problems = append(problems, fmt.Sprintf("%s: says never %d times; the budget is %d, so state what to do instead",
				filepath.ToSlash(rel), count, neverBudget))
		}
	}
	return problems
}
