package guard

// scoutRows are the scout's own restrictions: read-only, surveying the ground before anyone edits it.
func scoutRows() []Case {
	return []Case{
		asRole("scout", write("a scout never writes a file", "notes.md", true, "read-only")),
		asRole("scout", bash("a scout never commits", "git commit -m x", "feat/x", true, "read-only")),
		asRole("scout", bash("a scout greps the tree", "grep -r TODO .", "feat/x", false, "")),
		asRole("scout", bash("a scout may not run the orchestrator's run command", "komodo run", "feat/x", true, "may not run")),
	}
}

func init() {
	registerSuite(Suite{
		Name:  "scout",
		Rows:  scoutRows(),
		Hooks: []string{"timewarn"},
	})
}
