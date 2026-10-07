package guard

// testerRows are the tester's own restrictions: read-only but for running the test and build
// commands it judges by, writing no file and committing nothing itself.
func testerRows() []Case {
	return []Case{
		asRole("tester", write("a tester never writes a file", "fixture.go", true, "read-only")),
		asRole("tester", bash("a tester never commits", "git commit -m x", "feat/x", true, "read-only")),
		asRole("tester", bash("a tester runs the tests", "go test ./...", "feat/x", false, "")),
		asRole("tester", bash("a tester runs a build", "go build ./...", "feat/x", false, "")),
		asRole("tester", bash("a tester may not run the orchestrator's run command", "komodo run", "feat/x", true, "may not run")),
	}
}

func init() {
	registerSuite(Suite{
		Name:  "tester",
		Rows:  testerRows(),
		Hooks: []string{"timewarn"},
	})
}
