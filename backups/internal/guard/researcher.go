package guard

// researcherRows are the researcher's own restrictions: read-only, gathering evidence outside the repo.
func researcherRows() []Case {
	return []Case{
		asRole("researcher", write("a researcher never writes a file", "notes.md", true, "read-only")),
		asRole("researcher", bash("a researcher never commits", "git commit -m x", "feat/x", true, "read-only")),
		asRole("researcher", bash("a researcher reads a doc", "cat docs/prd.md", "feat/x", false, "")),
		asRole("researcher", bash("a researcher may not run the orchestrator's run command", "komodo run", "feat/x", true, "may not run")),
	}
}

func init() {
	registerSuite(Suite{
		Name:  "researcher",
		Rows:  researcherRows(),
		Hooks: []string{"timewarn"},
	})
}
