package guard

// plannerCommands are the komodo subcommands a planner's own session may run; lint is the one
// write-adjacent command a planner runs itself, to check a backlog edit before handing it on.
var plannerCommands = []string{"lint"}

// plannerRows are the planner's own restrictions: read-only but for lint, planning the backlog.
func plannerRows() []Case {
	return []Case{
		asRole("planner", write("a planner never writes a file", "docs/backlog/plan.md", true, "read-only")),
		asRole("planner", bash("a planner never commits", "git commit -m x", "feat/x", true, "read-only")),
		asRole("planner", bash("a planner runs lint", "komodo lint", "feat/x", false, "")),
		asRole("planner", bash("a planner may not run the orchestrator's run command", "komodo run", "feat/x", true, "may not run")),
	}
}

func init() {
	registerSuite(Suite{
		Name:     "planner",
		Commands: plannerCommands,
		Rows:     plannerRows(),
		Hooks:    []string{"timewarn"},
	})
}
