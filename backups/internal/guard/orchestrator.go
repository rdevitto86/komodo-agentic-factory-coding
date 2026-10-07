package guard

import "strings"

// orchestratorCommands are the komodo subcommands only the orchestrator's own session runs; a
// role calling one of these is refused (komodoFindings).
var orchestratorCommands = []string{
	"run", "resume", "status", "abandon", "ship", "release", "tag", "backlog", "list", "add",
	"rephase", "threads", "eval", "metrics", "recall", "gate", "guard", "hook", "doctor", "sync",
	"install", "pr", "worktree", "mount", "check",
}

// orchestratorRows are the global rows that document the orchestrator's own widened permission:
// an epic branch push or merge, a write outside the worktree, and spawning an isolated agent.
func orchestratorRows() []Case {
	var out []Case
	for _, row := range globalRows(DefaultPolicy()) {
		if strings.Contains(row.Name, "orchestrator") {
			out = append(out, row)
		}
	}
	return out
}

func init() {
	registerSuite(Suite{
		Name:     "orchestrator",
		Commands: orchestratorCommands,
		Rows:     orchestratorRows(),
		Hooks:    []string{"status", "prune"},
	})
}
