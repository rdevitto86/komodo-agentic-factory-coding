package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// interpreterHidesGit is the finding an interpreter's inline code or named script earns for hiding a git or gh call.
const interpreterHidesGit = "an interpreter running git or gh hides its command; run it in the shell"

// interpreters are the languages whose inline code or named script text the guard reads.
var interpreters = map[string]bool{
	"python": true, "python3": true, "node": true, "ruby": true, "perl": true,
	"php": true, "deno": true, "bun": true, "osascript": true,
}

// inlineCodeShortFlags are the single-letter flags that take inline code, alone or glued to it.
var inlineCodeShortFlags = []string{"-c", "-e", "-E", "-r", "-p"}

// gitVerbs name the git subcommands an interpreter's inline code must not hide.
var gitVerbs = `push|commit|merge|rebase|branch|update-ref|remote|config|tag`

// ghVerbs name the gh subcommands an interpreter's inline code must not hide.
var ghVerbs = `api|pr|repo|ruleset|secret`

// gitHidesRe matches the word git immediately followed by one of gitVerbs, across quotes and brackets.
var gitHidesRe = regexp.MustCompile(`\bgit\b[^A-Za-z0-9]{0,12}\b(?:` + gitVerbs + `)\b`)

// ghHidesRe matches the word gh immediately followed by one of ghVerbs, the same way.
var ghHidesRe = regexp.MustCompile(`\bgh\b[^A-Za-z0-9]{0,12}\b(?:` + ghVerbs + `)\b`)

// interpFindings refuses an interpreter whose inline code or named script text hides a git or gh call.
func interpFindings(kept []string, cwd, root string) []string {
	if len(kept) < 2 || !interpreters[commandName(kept[0])] {
		return nil
	}
	text, ok := interpreterText(kept, cwd, root)
	if !ok || !hidesGitOrGh(text) {
		return nil
	}
	return []string{interpreterHidesGit}
}

// hidesGitOrGh reports whether text holds git or gh immediately followed by one of the verbs it hides for.
func hidesGitOrGh(text string) bool {
	return gitHidesRe.MatchString(text) || ghHidesRe.MatchString(text)
}

// interpreterText is the inline code an interpreter runs, or the text of the script file it
// names, when that file exists in the worktree; it never runs the interpreter.
func interpreterText(kept []string, cwd, root string) (string, bool) {
	if code, ok := inlineCode(kept); ok {
		return code, true
	}
	operand := scriptOperand(kept)
	if operand == "" {
		return "", false
	}
	file := expandHome(operand)
	if !filepath.IsAbs(file) {
		if cwd == unresolvedDir {
			return "", false
		}
		file = filepath.Join(cwd, file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// inlineCode returns the argument an inline-code flag carries, glued to the flag or as the word after it.
func inlineCode(kept []string) (string, bool) {
	for index := 1; index < len(kept); index++ {
		token := kept[index]
		if token == "--eval" {
			if index+1 < len(kept) {
				return kept[index+1], true
			}
			return "", false
		}
		if value, ok := strings.CutPrefix(token, "--eval="); ok {
			return value, true
		}
		for _, flag := range inlineCodeShortFlags {
			if token == flag {
				if index+1 < len(kept) {
					return kept[index+1], true
				}
				return "", false
			}
			if strings.HasPrefix(token, flag) && len(token) > len(flag) {
				return token[len(flag):], true
			}
		}
	}
	return "", false
}

// scriptOperand is the first non-flag word after the interpreter's own name.
func scriptOperand(kept []string) string {
	for _, token := range kept[1:] {
		if !strings.HasPrefix(token, "-") {
			return token
		}
	}
	return ""
}
