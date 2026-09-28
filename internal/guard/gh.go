package guard

// ghFindings checks one gh call against the guard's rules: landing a pull request is the human's
// merge button, the same as a git merge onto a critical ref.
func ghFindings(words []string) []string {
	if len(words) >= 3 && words[1] == "pr" && words[2] == "merge" {
		return []string{"gh pr merge: landing is the human's merge button"}
	}
	return nil
}
