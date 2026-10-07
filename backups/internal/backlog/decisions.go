package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DecisionsDir is where a repo keeps one file per decision.
const DecisionsDir = "docs/decisions"

var decisionName = regexp.MustCompile(`^(\d{4})-[a-z0-9_-]+\.md$`)

// LintDecisions reports each decision number two files share and each markdown file not named NNNN-<slug>.md.
func LintDecisions(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, DecisionsDir))
	if err != nil {
		return nil
	}
	byNumber := map[string][]string{}
	var problems []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" {
			continue
		}
		match := decisionName.FindStringSubmatch(name)
		if match == nil {
			problems = append(problems, fmt.Sprintf("%s/%s: a decision file is named NNNN-<slug>.md", DecisionsDir, name))
			continue
		}
		byNumber[match[1]] = append(byNumber[match[1]], name)
	}
	numbers := make([]string, 0, len(byNumber))
	for number := range byNumber {
		numbers = append(numbers, number)
	}
	sort.Strings(numbers)
	for _, number := range numbers {
		if names := byNumber[number]; len(names) > 1 {
			problems = append(problems, fmt.Sprintf("%s: decision %s is taken by %s; renumber the later one to the next free number", DecisionsDir, number, strings.Join(names, ", ")))
		}
	}
	return problems
}
