package doctor

import (
	"os"
	"path/filepath"
)

// checkWorkflows reports each file under .github/workflows/, since the gate is local and nothing runs on the forge.
func checkWorkflows(root string) []Problem {
	var problems []Problem
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		problems = append(problems, Problem{"workflows", rel(root, filepath.Join(dir, entry.Name())),
			"the gate is local, nothing runs on the forge; delete it and run komodo gate --install"})
	}
	return problems
}
