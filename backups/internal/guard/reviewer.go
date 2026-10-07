package guard

// reviewerCommands are the komodo subcommands a reviewer's own session may run.
var reviewerCommands = []string{"check findings"}

// reviewerRows are the reviewer's own restrictions: read-only, so even a revision-aware git call
// that writes a file through --output, -o, or a trailing redirect is refused.
func reviewerRows() []Case {
	return []Case{
		asRole("reviewer", bash("a reviewer's git diff with --output after its revision still writes",
			"git diff HEAD~1 --output=.git/hooks/pre-commit", "feat/x", true, "host or toolkit config")),
		asRole("reviewer", bash("a reviewer's git log -p with a trailing --output writes outside the worktree",
			"git log -p main --output ../notes.txt", "feat/x", true, "outside the worktree")),
		asRole("reviewer", bash("a reviewer's git format-patch -o after its range writes outside the worktree",
			"git format-patch main..HEAD -o ../patches", "feat/x", true, "outside the worktree")),
		asRole("reviewer", bash("a reviewer's git show naming a file called --output after -- writes nothing",
			"git show HEAD -- --output", "feat/x", false, "")),
		asRole("reviewer", write("a reviewer never writes a file", "notes.md", true, "read-only")),
		asRole("reviewer", bash("a reviewer never commits", "git commit -m x", "feat/x", true, "read-only")),
		asRole("reviewer", bash("a reviewer may check findings", "komodo check findings TG-1.1", "feat/x", false, "")),
		asRole("reviewer", bash("a reviewer may not run the orchestrator's run command", "komodo run", "feat/x", true, "may not run")),
	}
}

func init() {
	registerSuite(Suite{
		Name:     "reviewer",
		Commands: reviewerCommands,
		Rows:     reviewerRows(),
		Hooks:    []string{"evidence", "timewarn"},
	})
}
