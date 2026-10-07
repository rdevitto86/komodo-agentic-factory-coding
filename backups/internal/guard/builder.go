package guard

// builderCommands are the komodo subcommands a builder's own session may run.
var builderCommands = []string{"check task", "check scope"}

// builderRows are the builder's own restrictions: an epic branch push or merge, a write outside
// its worktree, a spawn that tries to cut its own worktree, and the structural paths REQ-41 refuses.
func builderRows() []Case {
	return []Case{
		asRole("builder", bash("a harness session's push to an epic branch is refused (decision 0006)", "git push origin feat/1.0.0-alpha.7", "feat/x", true, "open a pull request")),
		asRole("builder", bash("a harness session's merge onto an epic branch is refused (decision 0006)", "git merge feat/x", "feat/1.0.0-alpha.7", true, "merge button")),

		asRole("builder", write("edit above the root", "../outside/file.go", true, "outside the worktree")),
		asRole("builder", write("a new source file", "internal/harness/new.go", false, "")),
		asRole("builder", bash("redirect above the root", "echo x > ../outside.txt", "feat/x", true, "outside the worktree")),
		asRole("builder", bash("redirect into the worktree", "echo x > out.txt", "feat/x", false, "")),
		asRole("builder", bash("a shell -c redirect above the root", "sh -c 'echo x > ../outside.txt'", "feat/x", true, "outside the worktree")),

		asRole("builder", spawn("a harness spawn with isolation set is denied", "worktree", true, "a spawn never cuts its own worktree")),
		asRole("builder", spawn("a harness spawn without isolation is allowed", "", false, "")),

		asRole("builder", write("a harness role's write to docs/prd.md is refused (REQ-41)", "docs/prd.md", true, "host or toolkit config")),
		asRole("builder", write("a harness role's write to eval/** is refused (REQ-41)", "eval/probe", true, "host or toolkit config")),
		asRole("builder", bash("sed -i edits a protected path in place", "sed -i s/a/b/ eval/golden.json", "feat/x", true, "host or toolkit config")),
		asRole("builder", bash("BSD sed -i with its own empty backup suffix still finds the real file",
			"sed -i '' s/a/b/ eval/golden.json", "feat/x", true, "host or toolkit config")),
		asRole("builder", bash("tee writes a protected path", "tee docs/prd.md", "feat/x", true, "host or toolkit config")),
		asRole("builder", bash("cp overwrites a protected path", "cp x eval/case.json", "feat/x", true, "host or toolkit config")),
		asRole("builder", bash("mv overwrites a protected path", "mv x eval/case.json", "feat/x", true, "host or toolkit config")),

		asRole("builder", bash("a builder may check its own task", "komodo check task TG-1.1", "feat/x", false, "")),
		asRole("builder", bash("a builder may not run the orchestrator's run command", "komodo run", "feat/x", true, "may not run")),
	}
}

func init() {
	registerSuite(Suite{
		Name:     "builder",
		Commands: builderCommands,
		Rows:     builderRows(),
		Hooks:    []string{"format", "taskchecks", "timewarn"},
	})
}
