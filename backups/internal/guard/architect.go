package guard

// architectRows are the architect's own restrictions: read-only, planning the design, never the code.
func architectRows() []Case {
	return []Case{
		asRole("architect", write("an architect never writes a file", "design.md", true, "read-only")),
		asRole("architect", bash("an architect never commits", "git commit -m x", "feat/x", true, "read-only")),
		asRole("architect", bash("an architect reads the tree", "git log --oneline", "feat/x", false, "")),
		asRole("architect", bash("an architect may not run the orchestrator's run command", "komodo run", "feat/x", true, "may not run")),
	}
}

func init() {
	registerSuite(Suite{
		Name:  "architect",
		Rows:  architectRows(),
		Hooks: []string{"timewarn"},
	})
}
