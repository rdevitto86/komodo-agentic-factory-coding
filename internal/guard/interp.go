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

// flagTable is one interpreter's option grammar: code letters, value letters, and whether -ne clusters.
type flagTable struct {
	code      map[string]bool
	value     map[string]bool
	longCode  map[string]bool
	longValue map[string]bool
	module    map[string]bool
	gluedOnly map[string]bool
	digits    map[string]bool
	clusters  bool
	subOnly   bool
}

// flagTables holds each interpreter's own option grammar; python3 and python share one.
var flagTables = map[string]flagTable{
	"python": {
		code:     set("c"),
		value:    set("W", "X", "Q"),
		module:   set("m"),
		clusters: true,
	},
	"node": {
		code:      set("e", "p"),
		value:     set("r", "C"),
		longCode:  set("--eval", "--print"),
		longValue: set("--require", "--import", "--loader", "--experimental-loader", "--conditions", "--input-type", "--env-file", "--title"),
	},
	"ruby": {
		code:      set("e"),
		value:     set("r", "I", "C", "E", "F", "x", "K", "T", "W"),
		gluedOnly: set("F", "x", "K", "T", "W"),
		digits:    set("0"),
		clusters:  true,
	},
	"perl": {
		code:      set("e", "E"),
		value:     set("M", "m", "I", "i", "l", "0", "F", "d", "D", "x", "C"),
		gluedOnly: set("M", "m", "i", "F", "d", "D", "x", "C"),
		digits:    set("l", "0"),
		clusters:  true,
	},
	"php": {
		code:     set("r", "B", "R", "E"),
		value:    set("d", "c", "z", "t"),
		module:   set("f"),
		clusters: false,
	},
	"osascript": {
		code:  set("e"),
		value: set("l", "s", "i"),
	},
	"bun": {
		code:     set("e", "p"),
		longCode: set("--eval", "--print"),
		subOnly:  true,
	},
	"deno": {
		subOnly: true,
	},
}

// set builds a lookup from its members.
func set(members ...string) map[string]bool {
	out := map[string]bool{}
	for _, member := range members {
		out[member] = true
	}
	return out
}

// scriptExtensions mark an interpreter operand as a script file, as opposed to a subcommand or module name.
var scriptExtensions = map[string]bool{
	".py": true, ".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".mts": true, ".tsx": true,
	".rb": true, ".pl": true, ".pm": true, ".php": true, ".scpt": true, ".applescript": true, ".sh": true,
}

// bunSubcommands are bun's own verbs; any other first word is a script or a package.json script name.
var bunSubcommands = set("run", "test", "install", "i", "add", "remove", "rm", "build", "x", "create",
	"upgrade", "pm", "link", "unlink", "init", "outdated", "publish", "patch", "update", "exec", "repl")

// interpCall is what one interpreter invocation will run: inline code, a script operand, or neither.
type interpCall struct {
	code    []string
	operand string
}

// parseInterpreter reads an interpreter's arguments with that interpreter's own flag table.
func parseInterpreter(kept []string) interpCall {
	name := commandName(kept[0])
	if name == "python3" {
		name = "python"
	}
	table := flagTables[name]
	args := kept[1:]
	var call interpCall
	if table.subOnly {
		return parseSubcommandInterpreter(name, args, table)
	}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			if index+1 < len(args) {
				call.operand = args[index+1]
			}
			return call
		}
		if strings.HasPrefix(arg, "--") {
			flag, value, hasEq := strings.Cut(arg, "=")
			switch {
			case table.longCode[flag]:
				code, next := nextValue(args, index, value, hasEq)
				call.code, index = append(call.code, code), next
			case table.longValue[flag] && !hasEq:
				index++
			}
			continue
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			if len(call.code) == 0 {
				call.operand = arg
			}
			return call
		}
		stop, next := call.readShort(args, index, table)
		index = next
		if stop {
			return call
		}
	}
	return call
}

// readShort reads one short option or cluster, returning whether option parsing ends there.
func (call *interpCall) readShort(args []string, index int, table flagTable) (bool, int) {
	arg := args[index]
	letters := arg[1:]
	if !table.clusters && len(letters) > 1 && allIn(letters, table.code) {
		// node reads -pe and -ep as --print --eval, so the next word is code.
		if index+1 < len(args) {
			call.code = append(call.code, args[index+1])
			return false, index + 1
		}
		return false, index
	}
	for position := 0; position < len(letters); position++ {
		letter := string(letters[position])
		if table.digits[letter] {
			position += digitRun(letters[position+1:])
			continue
		}
		rest := strings.TrimPrefix(letters[position+1:], "=")
		switch {
		case table.code[letter]:
			if rest != "" {
				call.code = append(call.code, rest)
				return false, index
			}
			if index+1 < len(args) {
				call.code = append(call.code, args[index+1])
				return false, index + 1
			}
			return false, index
		case table.module[letter]:
			if letter == "f" && index+1 < len(args) && rest == "" {
				call.operand = args[index+1]
			}
			return true, index
		case table.value[letter]:
			if rest == "" && !table.gluedOnly[letter] && index+1 < len(args) {
				return false, index + 1
			}
			return false, index
		}
		if !table.clusters {
			return false, index
		}
	}
	return false, index
}

// digitRun is how many characters an octal run, or x and a hex run, takes at the start of text.
func digitRun(text string) int {
	digits := "01234567"
	count := 0
	if strings.HasPrefix(text, "x") {
		digits, count = "0123456789abcdefABCDEF", 1
	}
	for count < len(text) && strings.ContainsRune(digits, rune(text[count])) {
		count++
	}
	return count
}

// allIn reports whether every letter of a short cluster is in the set.
func allIn(letters string, members map[string]bool) bool {
	for _, letter := range letters {
		if !members[string(letter)] {
			return false
		}
	}
	return true
}

// parseSubcommandInterpreter reads bun and deno, whose first word is usually a verb, not a script.
func parseSubcommandInterpreter(name string, args []string, table flagTable) interpCall {
	var call interpCall
	for index := 0; index < len(args); index++ {
		arg := args[index]
		flag, value, hasEq := strings.Cut(arg, "=")
		if table.longCode[flag] || (len(flag) == 2 && table.code[strings.TrimPrefix(flag, "-")] && strings.HasPrefix(flag, "-")) {
			code, next := nextValue(args, index, value, hasEq)
			call.code, index = append(call.code, code), next
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		switch {
		case name == "deno" && arg == "eval":
			for _, word := range args[index+1:] {
				if !strings.HasPrefix(word, "-") {
					call.code = append(call.code, word)
					return call
				}
			}
			return call
		case arg == "run":
			for _, word := range args[index+1:] {
				if !strings.HasPrefix(word, "-") {
					call.operand = word
					return call
				}
			}
			return call
		case name == "bun" && !bunSubcommands[arg]:
			call.operand = arg
			return call
		}
		return call
	}
	return call
}

// scriptLike reports whether an interpreter operand names a file rather than a module, verb, or package script.
func scriptLike(operand string) bool {
	return strings.Contains(operand, "/") || scriptExtensions[strings.ToLower(filepath.Ext(operand))]
}

// gitVerbs name the git subcommands an interpreter's inline code must not hide.
var gitVerbs = `push|commit|merge|rebase|branch|update-ref|remote|config|tag`

// ghVerbs name the gh subcommands an interpreter's inline code must not hide.
var ghVerbs = `api|pr|repo|ruleset|secret`

// gitHidesRe matches git then a gitVerbs word within 60 characters, so flags and quotes never hide it.
var gitHidesRe = regexp.MustCompile(`\bgit\b[\s\S]{0,60}?\b(?:` + gitVerbs + `)\b`)

// ghHidesRe matches the word gh with one of ghVerbs the same way.
var ghHidesRe = regexp.MustCompile(`\bgh\b[\s\S]{0,60}?\b(?:` + ghVerbs + `)\b`)

// hidesGitOrGh reports whether text holds git or gh followed closely by one of the verbs it hides for.
func hidesGitOrGh(text string) bool {
	return gitHidesRe.MatchString(text) || ghHidesRe.MatchString(text)
}

// interpScriptFindings checks what an interpreter will run: inline code, its stdin, a script this
// line already wrote, or one on disk; a script missing after an earlier command is not visible.
func (s *scanner) interpScriptFindings(kept []string, cwd, stdin string) []string {
	call := parseInterpreter(kept)
	if len(call.code) == 0 && (call.operand == "" || call.operand == "-") {
		return stdinFindings(stdin)
	}
	if len(call.code) > 0 {
		if hidesGitOrGh(strings.Join(call.code, "\n")) {
			return []string{interpreterHidesGit}
		}
		return nil
	}
	if call.operand == "" || !scriptLike(call.operand) {
		return nil
	}
	if write, found := s.recordedWrite(call.operand, cwd); found {
		if !write.known {
			return []string{scriptNotVisible}
		}
		if hidesGitOrGh(write.text) {
			return []string{interpreterHidesGit}
		}
		return nil
	}
	text, exists, ok := readScript(call.operand, cwd)
	switch {
	case !exists && s.createdEarlier():
		return []string{scriptNotVisible}
	case ok && hidesGitOrGh(text):
		return []string{interpreterHidesGit}
	}
	return nil
}

// stdinFindings judges the program an interpreter reads from a heredoc or a pipe.
func stdinFindings(stdin string) []string {
	switch {
	case strings.Contains(stdin, unknownInput):
		return []string{scriptNotVisible}
	case hidesGitOrGh(stdin):
		return []string{interpreterHidesGit}
	}
	return nil
}

// maxScriptBytes caps how much of a script the guard reads; a larger file is judged unreadable.
const maxScriptBytes = 256 << 10

// readScript reads a script relative to cwd, reporting whether it exists and whether it is text
// the guard can judge: under maxScriptBytes and holding no NUL byte.
func readScript(target, cwd string) (text string, exists, ok bool) {
	file := expandHome(target)
	if !filepath.IsAbs(file) {
		if cwd == unresolvedDir {
			return "", true, false
		}
		file = filepath.Join(cwd, file)
	}
	info, err := os.Stat(file)
	if err != nil {
		return "", !os.IsNotExist(err), false
	}
	if info.IsDir() || info.Size() > maxScriptBytes {
		return "", true, false
	}
	data, err := os.ReadFile(file)
	if err != nil || strings.ContainsRune(string(data), 0) {
		return "", true, false
	}
	return string(data), true, true
}
