package guard

// ghFindings checks one gh call against the guard's rules: landing a pull request is the human's
// merge button, the same as a git merge onto a critical ref, and a pull request opens through komodo.
func ghFindings(words []string) []string {
	if len(words) >= 3 && words[1] == "pr" && words[2] == "merge" {
		return []string{"gh pr merge: landing is the human's merge button"}
	}
	if len(words) >= 3 && words[1] == "pr" && words[2] == "create" {
		return []string{"gh pr create: open it with komodo pr create, which checks the title and applies the labels"}
	}
	return nil
}
