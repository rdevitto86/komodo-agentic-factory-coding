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
		code:      set("e", "p"),
		value:     set("c", "r", "d", "l"),
		longCode:  set("--eval", "--print"),
		longValue: set("--cwd", "--config", "--env-file", "--preload", "--tsconfig-override", "--define", "--loader", "--port", "--filter"),
		subOnly:   true,
	},
	"deno": {
		value:     set("c", "L"),
		longValue: set("--config", "--import-map", "--env-file", "--ext", "--lock", "--cert", "--location", "--seed", "--log-level"),
		subOnly:   true,
	},
}

// versionSuffixRe matches the version a command name carries after its interpreter, as in python3.12.
var versionSuffixRe = regexp.MustCompile(`^(python3|python|ruby|perl|node|php)[0-9][0-9.]*$`)

// interpName maps a versioned interpreter name such as python3.12 or perl5.36 to the one the guard knows.
func interpName(name string) string {
	if match := versionSuffixRe.FindStringSubmatch(name); match != nil {
		return match[1]
	}
	return name
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
	module  bool
}

// parseInterpreter reads an interpreter's arguments with that interpreter's own flag table.
func parseInterpreter(kept []string) interpCall {
	name := interpName(commandName(kept[0]))
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
			} else {
				// python -m names an installed module, which reads stdin as data, not as a program.
				call.module = true
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
			if !hasEq && (table.longValue[flag] || (len(flag) == 2 && table.value[flag[1:]])) {
				index++
			}
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
		case name == "deno" && scriptLike(arg):
			call.operand = arg
			return call
		case arg == "run":
			rest := args[index+1:]
			for at := 0; at < len(rest); at++ {
				word := rest[at]
				flag, _, hasEq := strings.Cut(word, "=")
				if strings.HasPrefix(word, "-") {
					if !hasEq && (table.longValue[flag] || (len(flag) == 2 && table.value[flag[1:]])) {
						at++
					}
					continue
				}
				call.operand = word
				return call
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

// gitHidesRe matches git then a gitVerbs word anywhere after it, for short inline code.
var gitHidesRe = regexp.MustCompile(`\bgit\b[\s\S]*?\b(?:` + gitVerbs + `)\b`)

// ghHidesRe matches gh then a ghVerbs word anywhere after it, the same way.
var ghHidesRe = regexp.MustCompile(`\bgh\b[\s\S]*?\b(?:` + ghVerbs + `)\b`)

// gitHidesInFileRe bounds the gap to 200 characters, so a whole script's unrelated words never pair.
var gitHidesInFileRe = regexp.MustCompile(`\bgit\b[\s\S]{0,200}?\b(?:` + gitVerbs + `)\b`)

// ghHidesInFileRe bounds the gh gap the same way.
var ghHidesInFileRe = regexp.MustCompile(`\bgh\b[\s\S]{0,200}?\b(?:` + ghVerbs + `)\b`)

// hidesGitOrGhInFile reports a hidden git or gh call in a script file's text, within the file bound.
func hidesGitOrGhInFile(text string) bool {
	return gitHidesInFileRe.MatchString(text) || ghHidesInFileRe.MatchString(text)
}

// hidesGitOrGh reports whether text holds git or gh followed closely by one of the verbs it hides for.
func hidesGitOrGh(text string) bool {
	return gitHidesRe.MatchString(text) || ghHidesRe.MatchString(text)
}

// interpScriptFindings checks what an interpreter will run: inline code, its stdin, a script this
// line already wrote, or one on disk; a script missing after an earlier command is not visible.
func (s *scanner) interpScriptFindings(kept []string, cwd, stdin string, active bool) []string {
	call := parseInterpreter(kept)
	if call.module {
		return nil
	}
	if len(call.code) == 0 && (call.operand == "" || call.operand == "-") {
		return stdinFindings(stdin)
	}
	if len(call.code) > 0 {
		code := strings.Join(call.code, "\n")
		switch {
		case hidesGitOrGh(code):
			return []string{interpreterHidesGit}
		case active && unresolvedText(code):
			return []string{scriptNotVisible}
		}
		return nil
	}
	if !scriptLike(call.operand) {
		text, visible := s.resolvedScript(interpName(commandName(kept[0])), call.operand, cwd)
		switch {
		case !visible:
			return []string{scriptNotVisible}
		case hidesGitOrGhInFile(text):
			return []string{interpreterHidesGit}
		}
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
	case !exists && s.createdEarlier(), exists && s.blind:
		return []string{scriptNotVisible}
	case ok && hidesGitOrGhInFile(text):
		return []string{interpreterHidesGit}
	}
	return nil
}

// resolvedScript reads an extensionless operand the way its interpreter resolves it: the file,
// node's .js, .mjs, .cjs, and index.js, or python's __main__.py; false when the line may have hidden it.
func (s *scanner) resolvedScript(name, operand, cwd string) (string, bool) {
	candidates := []string{operand}
	switch name {
	case "node":
		candidates = append(candidates, operand+".js", operand+".mjs", operand+".cjs", filepath.Join(operand, "index.js"))
	case "python", "python3":
		candidates = append(candidates, filepath.Join(operand, "__main__.py"))
	}
	for _, candidate := range candidates {
		if write, found := s.recordedWrite(candidate, cwd); found {
			return write.text, write.known
		}
		if text, _, ok := readScript(candidate, cwd); ok {
			return text, !s.blind
		}
	}
	if name == "bun" || name == "deno" {
		// bun run build and deno task names a package.json or deno.json script, not a file.
		return "", true
	}
	return "", !s.createdEarlier()
}

// stdinFindings judges the program an interpreter reads from a heredoc or a pipe.
func stdinFindings(stdin string) []string {
	switch {
	case hidesGitOrGh(stdin):
		return []string{interpreterHidesGit}
	case strings.Contains(stdin, unknownInput):
		return []string{scriptNotVisible}
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
