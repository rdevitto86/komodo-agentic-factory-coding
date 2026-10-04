package guard

import "strings"

// call is one simple command between control operators: its words and its redirect targets.
type call struct {
	words  []string
	writes []string
	// stdinBody is the raw heredoc text an interpreter such as python or node reads from stdin.
	stdinBody string
	// opaque marks eval of a script the guard could not resolve, always refused unverified.
	opaque bool
}

// opaqueScript is one heredoc body an interpreter reads from stdin, scanned apart from the main tokens.
type opaqueScript struct {
	words []string
	body  string
}

// controlOperators end one call and start the next in a command line.
var controlOperators = map[string]bool{";": true, "&&": true, "||": true, "&": true, "|": true, "(": true, ")": true}

// tokenize splits a command line into simple calls, stripping quotes from each word and pulling
// every > and >> target out as a write, the only two things the guard still looks for.
func tokenize(command string) []call {
	var calls []call
	var current call
	flush := func() {
		if len(current.words) > 0 || len(current.writes) > 0 {
			calls = append(calls, current)
		}
		current = call{}
	}
	stripped, opaque := stripHeredocs(command)
	tokens := rawTokens(stripped)
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		switch {
		case controlOperators[token]:
			flush()
		case token == ">" || token == ">>":
			if index+1 < len(tokens) {
				index++
				current.writes = append(current.writes, tokens[index])
			}
		default:
			current.words = append(current.words, token)
		}
	}
	flush()
	for _, script := range opaque {
		calls = append(calls, call{words: script.words, stdinBody: script.body})
	}
	return calls
}

// positionalArgs returns words's own operands, skipping every flag.
func positionalArgs(words []string) []string {
	var out []string
	for _, word := range words {
		if !strings.HasPrefix(word, "-") {
			out = append(out, word)
		}
	}
	return out
}

// sedValueFlags are sed's own flags that take a separate value, consumed and never read as a file.
var sedValueFlags = map[string]bool{"-e": true, "-f": true, "--expression": true, "--file": true}

// sedInPlaceTargets returns sed's file operands when -i or --in-place edits them in place, BSD's own
// empty backup suffix consumed like -e and -f's values; with no script flag, the first operand is the script.
func sedInPlaceTargets(args []string) []string {
	inPlace, hasScript := false, false
	var operands []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-i" && index+1 < len(args) && args[index+1] == "":
			inPlace = true
			index++
		case arg == "-i" || strings.HasPrefix(arg, "-i") || arg == "--in-place" || strings.HasPrefix(arg, "--in-place="):
			inPlace = true
		case sedValueFlags[arg]:
			hasScript = true
			index++
		case strings.HasPrefix(arg, "-"):
			// another flag, ignored
		default:
			operands = append(operands, arg)
		}
	}
	if !inPlace {
		return nil
	}
	if !hasScript && len(operands) > 0 {
		operands = operands[1:]
	}
	return operands
}

// commandWrites returns the extra paths a known command writes to, beyond a shell redirect: every
// tee operand, cp and mv's destination, and sed -i's file operands.
func commandWrites(words []string) []string {
	if len(words) == 0 {
		return nil
	}
	switch commandName(words[0]) {
	case "tee":
		return positionalArgs(words[1:])
	case "cp", "mv":
		args := positionalArgs(words[1:])
		if len(args) < 2 {
			return nil
		}
		return args[len(args)-1:]
	case "sed":
		return sedInPlaceTargets(words[1:])
	}
	return nil
}

// isHeredocControl reports whether char ends one simple command and starts the next, outside quotes.
func isHeredocControl(char rune) bool {
	return char == ';' || char == '&' || char == '|' || char == '('
}

// containsWord reports whether words holds any of targets exactly.
func containsWord(words []string, targets ...string) bool {
	for _, word := range words {
		for _, target := range targets {
			if word == target {
				return true
			}
		}
	}
	return false
}

// hasFlagValue reports whether words names flag followed by value, or flag=value in one word.
func hasFlagValue(words []string, flag, value string) bool {
	for index, word := range words {
		if word == flag+"="+value || (word == flag && index+1 < len(words) && words[index+1] == value) {
			return true
		}
	}
	return false
}

// isPlainPath reports whether word is an ordinary file path: never a process substitution, an fd
// duplication, a device or proc node, or one naming a command substitution of its own.
func isPlainPath(word string) bool {
	if word == "" {
		return false
	}
	for _, prefix := range []string{">(", "<(", "&", "/dev/", "/proc/"} {
		if strings.HasPrefix(word, prefix) {
			return false
		}
	}
	return !strings.Contains(word, "$(") && !strings.Contains(word, "`")
}

// hasFileRedirect reports whether words holds a > or >> followed by a plain path.
func hasFileRedirect(words []string) bool {
	for index, word := range words {
		if (word == ">" || word == ">>") && index+1 < len(words) && isPlainPath(words[index+1]) {
			return true
		}
	}
	return false
}

// hasFileArgument reports whether words names a real positional plain-path argument.
func hasFileArgument(words []string) bool {
	for _, word := range words {
		if word == ">" || word == ">>" || word == "|" || word == "||" || strings.HasPrefix(word, "-") {
			continue
		}
		if isPlainPath(word) {
			return true
		}
	}
	return false
}

// heredocAllowlisted reports whether a simple command, named by its words before and after the
// heredoc operator, is a known prose consumer, not piped on: a commit, a PR body, or a file write.
func heredocAllowlisted(before, after []string, piped bool) bool {
	if piped || len(before) == 0 {
		return false
	}
	rest := append(append([]string{}, before[1:]...), after...)
	switch commandName(before[0]) {
	case "git":
		return containsWord(rest, "commit", "tag", "notes") &&
			(hasFlagValue(rest, "-F", "-") || hasFlagValue(rest, "--file", "-"))
	case "gh":
		return hasFlagValue(rest, "--body-file", "-") || hasFlagValue(rest, "-F", "-") ||
			hasFlagValue(rest, "--input", "-")
	case "cat":
		return len(before) == 1 && hasFileRedirect(after)
	case "tee":
		return len(before) == 1 && (hasFileRedirect(after) || hasFileArgument(rest))
	default:
		return false
	}
}

// opaqueInterpreters read a script on stdin the guard cannot parse as shell, scanned separately.
var opaqueInterpreters = map[string]bool{
	"python": true, "python2": true, "python3": true, "node": true, "nodejs": true, "ruby": true, "perl": true,
}

// isOpaqueInterpreterStdin reports whether before names a bare opaque interpreter, with no script
// file among its own or after's operands, so it reads its program from stdin.
func isOpaqueInterpreterStdin(before, after []string, piped bool) bool {
	if piped || len(before) == 0 || !opaqueInterpreters[commandName(before[0])] {
		return false
	}
	for _, arg := range append(append([]string{}, before[1:]...), after...) {
		if !strings.HasPrefix(arg, "-") {
			return false
		}
	}
	return true
}

// restOfLine reads the words left on a heredoc operator's own line, starting right after its
// delimiter, and whether a bare | pipes the command they belong to into another.
func restOfLine(runes []rune, start int) (words []string, piped bool) {
	var buf strings.Builder
	open := false
	inSingle, inDouble := false, false
	flush := func() {
		if open {
			words = append(words, buf.String())
			buf.Reset()
			open = false
		}
	}
	for index := start; index < len(runes); index++ {
		char := runes[index]
		switch {
		case inSingle:
			open = true
			if char == '\'' {
				inSingle = false
			} else {
				buf.WriteRune(char)
			}
		case inDouble:
			open = true
			if char == '"' {
				inDouble = false
			} else if char == '\\' && index+1 < len(runes) {
				index++
				buf.WriteRune(runes[index])
			} else {
				buf.WriteRune(char)
			}
		case char == '\'':
			inSingle, open = true, true
		case char == '"':
			inDouble, open = true, true
		case char == '\n' || char == ';' || char == '&' || char == ')':
			flush()
			return words, piped
		case char == ' ' || char == '\t' || char == '\r':
			flush()
		case char == '|' && index+1 < len(runes) && runes[index+1] == '|':
			flush()
			words = append(words, "||")
			index++
		case char == '|':
			flush()
			piped = true
			words = append(words, "|")
		case (char == '>' || char == '<') && index+1 < len(runes) && runes[index+1] == '(':
			open = true
			depth := 0
			for index < len(runes) {
				buf.WriteRune(runes[index])
				switch runes[index] {
				case '(':
					depth++
				case ')':
					depth--
				}
				if depth == 0 && runes[index] == ')' {
					break
				}
				index++
			}
		case char == '>' && index+1 < len(runes) && runes[index+1] == '>':
			flush()
			words = append(words, ">>")
			index++
		case char == '>':
			flush()
			words = append(words, ">")
		case char == '(':
			flush()
			return words, piped
		default:
			open = true
			buf.WriteRune(char)
		}
	}
	flush()
	return words, piped
}

// countHeredocs counts every << heredoc operator outside quotes, a <<< herestring excluded.
func countHeredocs(runes []rune) int {
	count := 0
	inSingle, inDouble := false, false
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		switch {
		case inSingle:
			inSingle = char != '\''
		case inDouble:
			if char == '\\' && index+1 < len(runes) {
				index++
			} else if char == '"' {
				inDouble = false
			}
		case char == '\'':
			inSingle = true
		case char == '"':
			inDouble = true
		case char == '<' && index+1 < len(runes) && runes[index+1] == '<' &&
			(index+2 >= len(runes) || runes[index+2] != '<'):
			count++
			index++
		}
	}
	return count
}

// pendingHeredoc is one heredoc on the current line, waiting for its body once the line ends.
type pendingHeredoc struct {
	delimiter string
	// opaque names the interpreter reading this heredoc from stdin, or nil for a plain prose skip.
	opaque []string
}

// stripHeredocs removes a heredoc's body only for an allowlisted, unpiped, unnested prose consumer,
// and only when the whole command carries exactly one heredoc; two or more skips neither.
func stripHeredocs(command string) (string, []opaqueScript) {
	var out strings.Builder
	var pending []pendingHeredoc
	var scripts []opaqueScript
	inSingle, inDouble, inBacktick := false, false, false
	subDepth := 0
	runes := []rune(command)
	oneHeredoc := countHeredocs(runes) == 1
	index, cmdStart := 0, 0
	for index < len(runes) {
		char := runes[index]
		switch {
		case inSingle:
			out.WriteRune(char)
			inSingle = char != '\''
			index++
		case inDouble:
			out.WriteRune(char)
			if char == '\\' && index+1 < len(runes) {
				index++
				out.WriteRune(runes[index])
			} else if char == '"' {
				inDouble = false
			}
			index++
		case char == '\'':
			inSingle = true
			out.WriteRune(char)
			index++
		case char == '"':
			inDouble = true
			out.WriteRune(char)
			index++
		case char == '`':
			inBacktick = !inBacktick
			out.WriteRune(char)
			index++
		case char == '$' && index+1 < len(runes) && runes[index+1] == '(':
			out.WriteString("$(")
			index += 2
			subDepth++
			cmdStart = index
		case char == '<' && index+1 < len(runes) && runes[index+1] == '(':
			out.WriteString("<(")
			index += 2
			subDepth++
			cmdStart = index
		case char == ')':
			if subDepth > 0 {
				subDepth--
			}
			out.WriteRune(char)
			index++
			cmdStart = index
		case char == '<' && index+1 < len(runes) && runes[index+1] == '<' &&
			(index+2 >= len(runes) || runes[index+2] != '<'):
			operatorStart := index
			out.WriteString("<<")
			index += 2
			if index < len(runes) && runes[index] == '-' {
				out.WriteRune('-')
				index++
			}
			for index < len(runes) && (runes[index] == ' ' || runes[index] == '\t') {
				out.WriteRune(runes[index])
				index++
			}
			delimiter, consumed := heredocDelimiter(runes[index:])
			for offset := 0; offset < consumed; offset++ {
				out.WriteRune(runes[index+offset])
			}
			index += consumed
			before := strings.Fields(string(runes[cmdStart:operatorStart]))
			after, piped := restOfLine(runes, index)
			switch {
			case delimiter != "" && oneHeredoc && subDepth == 0 && !inBacktick && heredocAllowlisted(before, after, piped):
				pending = append(pending, pendingHeredoc{delimiter: delimiter})
			case delimiter != "" && oneHeredoc && subDepth == 0 && !inBacktick && isOpaqueInterpreterStdin(before, after, piped):
				pending = append(pending, pendingHeredoc{delimiter: delimiter, opaque: before})
			}
		case char == '\n':
			out.WriteRune('\n')
			index++
			for _, item := range pending {
				var body string
				index, body = skipHeredocBody(runes, index, item.delimiter)
				if item.opaque != nil {
					scripts = append(scripts, opaqueScript{words: item.opaque, body: body})
				}
			}
			pending = nil
			cmdStart = index
		case isHeredocControl(char):
			out.WriteRune(char)
			index++
			if (char == '&' && index < len(runes) && runes[index] == '&') ||
				(char == '|' && index < len(runes) && runes[index] == '|') {
				out.WriteRune(runes[index])
				index++
			}
			cmdStart = index
		default:
			out.WriteRune(char)
			index++
		}
	}
	return out.String(), scripts
}

// heredocDelimiter reads a heredoc operator's delimiter word right after it, quoted or bare, and
// how many runes of the input it consumed.
func heredocDelimiter(runes []rune) (string, int) {
	if len(runes) == 0 {
		return "", 0
	}
	if runes[0] == '\'' || runes[0] == '"' {
		quote := runes[0]
		for index := 1; index < len(runes); index++ {
			if runes[index] == quote {
				return string(runes[1:index]), index + 1
			}
		}
		return string(runes[1:]), len(runes)
	}
	index := 0
	for index < len(runes) && runes[index] != ' ' && runes[index] != '\t' && runes[index] != '\n' {
		index++
	}
	return string(runes[:index]), index
}

// skipHeredocBody advances past one heredoc's body: every line up to and including the first one
// that trims to delimiter, returning the index right after it and the body's own raw text.
func skipHeredocBody(runes []rune, start int, delimiter string) (int, string) {
	var body strings.Builder
	index := start
	for index <= len(runes) {
		lineStart := index
		for index < len(runes) && runes[index] != '\n' {
			index++
		}
		line := strings.TrimLeft(string(runes[lineStart:index]), " \t")
		if line == delimiter {
			if index < len(runes) {
				index++
			}
			return index, body.String()
		}
		body.WriteString(string(runes[lineStart:index]))
		if index >= len(runes) {
			return index, body.String()
		}
		body.WriteRune('\n')
		index++
	}
	return index, body.String()
}

// hasClosingBacktick reports whether a closing backtick follows, so a lone one, naming no
// command, stays a literal character instead of opening a quote that runs to the end.
func hasClosingBacktick(runes []rune) bool {
	for _, r := range runes {
		if r == '`' {
			return true
		}
	}
	return false
}

// rawTokens splits a command line into words and operators, honouring single and double quotes, a
// matched pair of backticks, and a backslash escape, the way a shell's own word splitting does.
func rawTokens(command string) []string {
	var tokens []string
	var buf strings.Builder
	open := false
	inSingle, inDouble, inBacktick := false, false, false
	flushWord := func() {
		if open {
			tokens = append(tokens, buf.String())
			buf.Reset()
			open = false
		}
	}
	addOp := func(op string) {
		flushWord()
		tokens = append(tokens, op)
	}
	runes := []rune(command)
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		switch {
		case inSingle:
			open = true
			if char == '\'' {
				inSingle = false
			} else {
				buf.WriteRune(char)
			}
		case inDouble:
			open = true
			switch {
			case char == '"':
				inDouble = false
			case char == '\\' && index+1 < len(runes) && strings.ContainsRune("\"\\$`", runes[index+1]):
				index++
				buf.WriteRune(runes[index])
			default:
				buf.WriteRune(char)
			}
		case inBacktick:
			open = true
			if char == '`' {
				inBacktick = false
			} else {
				buf.WriteRune(char)
			}
		case char == '\'':
			inSingle, open = true, true
		case char == '"':
			inDouble, open = true, true
		case char == '`' && hasClosingBacktick(runes[index+1:]):
			inBacktick, open = true, true
		case char == '\\' && index+1 < len(runes):
			index++
			open = true
			buf.WriteRune(runes[index])
		case char == '\n':
			addOp(";")
		case char == ' ' || char == '\t' || char == '\r':
			flushWord()
		case char == '&' && index+1 < len(runes) && runes[index+1] == '&':
			index++
			addOp("&&")
		case char == '|' && index+1 < len(runes) && runes[index+1] == '|':
			index++
			addOp("||")
		case char == '>' && index+1 < len(runes) && runes[index+1] == '>':
			index++
			addOp(">>")
		case char == ';' || char == '&' || char == '|' || char == '>' || char == '(' || char == ')':
			addOp(string(char))
		default:
			open = true
			buf.WriteRune(char)
		}
	}
	flushWord()
	return tokens
}
