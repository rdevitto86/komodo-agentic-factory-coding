package guard

import (
	"regexp"
	"regexp/syntax"
	"runtime"
	"strings"

	"komodo/internal/mount"
)

// privateCases builds, per registered private pattern, rows that send a matching link out and plain text beside them.
func privateCases() []Case {
	var out []Case
	for _, pattern := range mount.GuardPrivatePatterns() {
		link, ok := privateSample(pattern)
		if !ok {
			continue
		}
		out = append(out,
			bash("a commit message carrying "+pattern, "git commit -m 'feat: x\n\n"+link+"'", "feat/x", true, leakFinding),
			bash("a commit message with plain text beside "+pattern, "git commit -m 'feat: x\n\nPlain text.'", "feat/x", false, ""),
			bash("a pull request body carrying "+pattern, "gh pr create --base main --head feat/x --title t --body '"+link+"'", "feat/x", true, leakFinding),
			bash("a pull request body with plain text beside "+pattern, "gh pr create --base main --head feat/x --title t --body 'Plain text.'", "feat/x", false, ""),
			bash("a comment body file carrying "+pattern, "gh pr comment 1 --body-file - <<'EOF'\n"+link+"\nEOF", "feat/x", true, leakFinding),
			bash("a comment body file with plain text beside "+pattern, "gh pr comment 1 --body-file - <<'EOF'\nPlain text.\nEOF", "feat/x", false, ""),
			bash("an api comment field carrying "+pattern, "gh api repos/o/r/issues/1/comments -f body='"+link+"'", "feat/x", true, leakFinding),
			bash("an api comment field with plain text beside "+pattern, "gh api repos/o/r/issues/1/comments -f body='Plain text.'", "feat/x", false, ""),
			bash("a heredoc body file carrying "+pattern+" sent by --body-file", "cat > pr.md <<'EOF'\n"+link+"\nEOF\ngh pr create --base main --head feat/x --title t --body-file pr.md", "feat/x", true, leakFinding),
			bash("a heredoc body file with plain text sent by --body-file beside "+pattern, "cat > pr.md <<'EOF'\nPlain text.\nEOF\ngh pr create --base main --head feat/x --title t --body-file pr.md", "feat/x", false, ""),
			bash("a heredoc body file carrying "+pattern+" sent by -F", "cat > pr.md <<'EOF'\n"+link+"\nEOF\ngh pr create --base main --head feat/x --title t -F pr.md", "feat/x", true, leakFinding),
			bash("a heredoc commit message carrying "+pattern+" sent by -F", "cat > msg.txt <<'EOF'\nfeat: x\n\n"+link+"\nEOF\ngit commit -F msg.txt", "feat/x", true, leakFinding),
			bash("a heredoc commit message with plain text sent by -F beside "+pattern, "cat > msg.txt <<'EOF'\nfeat: x\n\nPlain text.\nEOF\ngit commit -F msg.txt", "feat/x", false, ""),
		)
	}
	return out
}

// privateSample builds a link a private pattern matches, walking its syntax tree for one shortest match.
func privateSample(pattern string) (string, bool) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return "", false
	}
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return "", false
	}
	var sample strings.Builder
	if !writeSample(&sample, parsed.Simplify()) {
		return "", false
	}
	link := "https://" + sample.String() + "_0123"
	if !compiled.MatchString(link) {
		link = sample.String()
	}
	return link, compiled.MatchString(link) && !strings.ContainsAny(link, "'\n")
}

// writeSample writes one short string a syntax node matches, reporting false for a node it cannot build.
func writeSample(out *strings.Builder, node *syntax.Regexp) bool {
	switch node.Op {
	case syntax.OpLiteral:
		out.WriteString(string(node.Rune))
	case syntax.OpCharClass:
		if len(node.Rune) < 2 {
			return false
		}
		out.WriteRune(node.Rune[0])
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		out.WriteByte('x')
	case syntax.OpConcat:
		for _, sub := range node.Sub {
			if !writeSample(out, sub) {
				return false
			}
		}
	case syntax.OpCapture, syntax.OpPlus, syntax.OpAlternate:
		return writeSample(out, node.Sub[0])
	case syntax.OpStar, syntax.OpQuest, syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine,
		syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
	default:
		return false
	}
	return true
}

// foldedCaseCases builds the config-path rows that only a case-insensitive disk mismatches,
// so the table only runs them on darwin and windows.
func foldedCaseCases() []Case {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return nil
	}
	return []Case{
		write("a case-folded bin path on a case-insensitive disk", "Bin/komodo-darwin-arm64", true, "host or toolkit config"),
		bash("a case-folded git hooks path on a case-insensitive disk", "touch .GIT/hooks/pre-commit", "feat/x", true, "host or toolkit config"),
	}
}
