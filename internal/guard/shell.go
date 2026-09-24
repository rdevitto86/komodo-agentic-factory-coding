package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxDepth caps how deeply substitutions, eval, shell scripts, and sourced files may nest.
const maxDepth = 12

// scanner reads one command line and everything it runs, sharing variables and depth across them.
type scanner struct {
	root   string
	policy Policy
	depth  int
	vars   map[string]string
	writes map[string]scriptWrite
	seen   int
	blind  bool
}

// scriptWrite is what this command line put into one file: the text, or that the guard cannot see it.
type scriptWrite struct {
	text  string
	known bool
}

// scriptNotVisible is the finding an unknown write earns when the same line tries to read it back.
const scriptNotVisible = "a script written and run in one command is not visible to the guard; write it, then run it in a second call"

// commandFindings returns every reason to refuse one shell command.
func commandFindings(command, root, cwd, branch string, policy Policy) []string {
	s := &scanner{root: root, policy: policy, vars: map[string]string{}, writes: map[string]scriptWrite{}}
	findings, _ := s.scan(command, cwd, branch)
	return findings
}

// scan checks a command line and returns its findings and the branch it leaves checked out.
func (s *scanner) scan(command, cwd, branch string) ([]string, string) {
	if s.depth >= maxDepth {
		return []string{"commands nest deeper than the guard reads; run them one at a time"}, branch
	}
	s.depth++
	defer func() { s.depth-- }()
	line := lex(command)
	var findings []string
	for _, body := range line.substitutions {
		nested, _ := s.scan(body, cwd, branch)
		findings = append(findings, nested...)
	}
	commands := parse(line.tokens)
	// current tracks the branch across commands, since a switch or checkout changes it mid-chain.
	current := branch
	for index, cmd := range commands {
		var found []string
		found, cwd, current = s.command(cmd, pipedInput(commands, index), cwd, current)
		findings = append(findings, found...)
	}
	return findings, current
}

// pipedInput approximates what a pipe feeds a command: the heredocs, here-strings, and echoed
// words from every command upstream in the same pipeline, not just the one right before it.
func pipedInput(commands []simpleCommand, index int) []string {
	start := index
	for start > 0 && commands[start-1].pipesTo {
		start--
	}
	if start == index {
		return nil
	}
	var input []string
	literal := true
	for _, previous := range commands[start:index] {
		input = append(input, upstreamWords(previous)...)
		literal = literal && passesLiterally(previous)
	}
	if !literal {
		input = append(input, unknownInput)
	}
	return input
}

// passesLiterally reports whether a pipeline stage emits only text the guard can read: echo, printf,
// or a bare cat of its stdin, and never a filter such as sed that rewrites what flows through.
func passesLiterally(cmd simpleCommand) bool {
	if len(cmd.words) == 0 {
		return true
	}
	name := filepath.Base(cmd.words[0].value)
	if name == "echo" || name == "printf" {
		return true
	}
	if name != "cat" {
		return false
	}
	for _, w := range cmd.words[1:] {
		if !strings.HasPrefix(w.value, "-") || w.value == "-" {
			return false
		}
	}
	return true
}

// upstreamWords is one command's own contribution to piped input: its heredocs, here-strings, and
// echoed words, plus unknownInput when an escape or a format hides what it really prints.
func upstreamWords(cmd simpleCommand) []string {
	input := append([]string{}, cmd.stdin...)
	if text, printed, known := echoedText(cmd); printed {
		input = append(input, text)
		if !known {
			input = append(input, unknownInput)
		}
	}
	return input
}

// unknownInput stands in for piped text the guard cannot see, such as printf output with an escape.
const unknownInput = "\x00unknown input\x00"

// echoedText is what an echo or printf prints, reporting whether the command is one and whether
// its output is literal; a backslash escape, or printf's % format, hides the real text.
func echoedText(cmd simpleCommand) (text string, printed, known bool) {
	if len(cmd.words) < 2 {
		return "", false, false
	}
	name := filepath.Base(cmd.words[0].value)
	if name != "echo" && name != "printf" {
		return "", false, false
	}
	args := make([]string, 0, len(cmd.words)-1)
	for _, w := range cmd.words[1:] {
		args = append(args, w.value)
	}
	args = dropPrintOptions(name, args)
	text = strings.Join(args, " ")
	known = !strings.Contains(text, "\\") && !(name == "printf" && strings.Contains(text, "%")) && !anyExpands(cmd.words[1:])
	return text, true, known
}

// stdinOf is everything a command reads on stdin: heredocs, here-strings, < files, and piped text,
// with unknownInput standing in for a < file the guard cannot see.
func (s *scanner) stdinOf(cmd simpleCommand, upstream []string, cwd string) string {
	parts := append([]string{}, cmd.stdin...)
	for _, input := range cmd.inputs {
		if write, found := s.recordedWrite(input, cwd); found {
			if !write.known {
				parts = append(parts, unknownInput)
			}
			parts = append(parts, write.text)
			continue
		}
		text, exists, ok := readScript(input, cwd)
		switch {
		case exists && ok && !s.blind:
			parts = append(parts, text)
		case exists || s.createdEarlier():
			parts = append(parts, unknownInput)
		}
	}
	return strings.Join(append(parts, upstream...), "\n")
}

// expandingValues are the argument strings that came from words the shell expands at run time,
// so a check can ask whether one particular argument holds a substitution, not the whole command.
func (s *scanner) expandingValues(words []word) map[string]bool {
	values := map[string]bool{}
	for _, w := range words {
		if !w.expands {
			continue
		}
		for _, value := range s.expand([]word{w}) {
			values[value] = true
		}
	}
	return values
}

// messageSource reads one git or gh call's outgoing text through this line's recorded writes.
func (s *scanner) messageSource(cmd simpleCommand, cwd, stdin string) *messageSource {
	return &messageSource{s: s, cwd: cwd, stdin: stdin, expanding: s.expandingValues(cmd.words)}
}

// anyExpands reports whether any word holds a substitution or parameter the shell fills in at run time.
func anyExpands(words []word) bool {
	for _, w := range words {
		if w.expands {
			return true
		}
	}
	return false
}

// echoOptionRe matches one of echo's own leading options, such as -n, -e, or -ne.
var echoOptionRe = regexp.MustCompile(`^-[neE]+$`)

// dropPrintOptions removes echo's leading -n, -e, and -E, or printf's leading -v name and --, keeping every later word.
func dropPrintOptions(name string, args []string) []string {
	for len(args) > 0 {
		switch {
		case name == "echo" && echoOptionRe.MatchString(args[0]):
			args = args[1:]
		case name == "printf" && args[0] == "-v" && len(args) > 1:
			args = args[2:]
		case name == "printf" && args[0] == "--":
			return args[1:]
		default:
			return args
		}
	}
	return args
}

// command checks one simple command and returns its findings, the directory after it, and the branch.
func (s *scanner) command(cmd simpleCommand, upstream []string, cwd, branch string) ([]string, string, string) {
	s.seen++
	stdin := s.stdinOf(cmd, upstream, cwd)
	var findings []string
	for _, target := range cmd.writes {
		findings = append(findings, pathFindings(target, cwd, s.root, s.policy)...)
	}
	if len(cmd.writes) > 0 {
		s.recordWrites(cmd, cwd, stdin)
	}
	tokens := s.expand(cmd.words)
	for len(tokens) > 0 && reservedWords[tokens[0]] {
		tokens = tokens[1:]
	}
	var kept, assigns []string
	leading := true
	for _, token := range tokens {
		if leading && assignRe.MatchString(token) {
			if name, _, _ := strings.Cut(token, "="); isScrubbed(name) {
				findings = append(findings, fmt.Sprintf("%s overrides a variable the headless run scrubs", token))
			}
			assigns = append(assigns, token)
			continue
		}
		leading = false
		kept = append(kept, token)
	}
	if len(kept) == 0 {
		s.remember(assigns)
		return findings, cwd, branch
	}
	var wrapFindings []string
	kept, wrapFindings = unwrapCommand(kept)
	findings = append(findings, wrapFindings...)
	if len(kept) == 0 {
		return findings, cwd, branch
	}

	name := commandName(kept[0])
	s.recordOutputWrites(name, kept, cwd)
	switch {
	case name == "cd":
		cwd = changeDir(kept, cwd)
	case name == "pushd" || name == "popd":
		cwd = unresolvedDir
	case assignBuiltins[name]:
		for _, arg := range kept[1:] {
			if varName, _, _ := strings.Cut(arg, "="); isScrubbed(varName) {
				findings = append(findings, fmt.Sprintf("%s %s changes a variable the headless run scrubs", name, arg))
			}
		}
		if name == "export" || name == "declare" || name == "typeset" || name == "local" || name == "readonly" {
			s.remember(kept[1:])
		}
	case name == "printf" || name == "read":
		for index, arg := range kept[1:] {
			previous := kept[index]
			if (name == "read" && !strings.HasPrefix(arg, "-")) || previous == "-v" {
				if isScrubbed(arg) {
					findings = append(findings, fmt.Sprintf("%s sets %s, a variable the headless run scrubs", name, arg))
				}
			}
		}
	case name == "source" || name == ".":
		var sourced []string
		sourced, branch = s.sourced(kept, cwd, branch, stdin)
		findings = append(findings, sourced...)
	case name == "dd":
		findings = append(findings, ddPaths(kept, cwd, s.root, s.policy)...)
	case pathWriters[name] || isConditionalWriter(name, kept):
		findings = append(findings, writerPaths(name, kept, cwd, s.root, s.policy)...)
		if interpreters[interpName(name)] {
			findings = append(findings, s.interpScriptFindings(kept, cwd, stdin, s.expandingValues(cmd.words))...)
		}
		if name == "tee" {
			s.recordTeeWrites(kept, cwd, stdin)
		}
	case name == "git":
		var gitResult []string
		gitResult, branch = gitFindings(kept, branch, cwd, s.root, s.policy, s.messageSource(cmd, cwd, stdin))
		findings = append(findings, gitResult...)
	case name == "gh":
		findings = append(findings, ghFindings(kept, s.messageSource(cmd, cwd, stdin), s.policy)...)
	case interpreters[interpName(name)]:
		findings = append(findings, s.interpScriptFindings(kept, cwd, stdin, s.expandingValues(cmd.words))...)
	case name == "eval" && len(kept) > 1:
		var evaluated []string
		evaluated, branch = s.scan(strings.Join(kept[1:], " "), cwd, branch)
		findings = append(findings, evaluated...)
	case shells[name]:
		findings = append(findings, s.shell(kept, cwd, branch, stdin)...)
	case looksLikeScriptPath(kept[0]):
		var scriptFindings []string
		scriptFindings, branch = s.scriptCommandFindings(kept[0], cwd, branch)
		findings = append(findings, scriptFindings...)
	}
	return findings, cwd, branch
}

// shell checks what a shell runs: its -c script, the script file it names, or its stdin.
func (s *scanner) shell(kept []string, cwd, branch, stdin string) []string {
	script, operand := shellScript(kept)
	switch {
	case script != "":
		nested, _ := s.scan(script, cwd, branch)
		return nested
	case operand != "":
		nested, _ := s.sourced([]string{kept[0], operand}, cwd, branch, stdin)
		return nested
	case stdin != "":
		nested, _ := s.scan(strings.ReplaceAll(stdin, unknownInput, ""), cwd, branch)
		if len(nested) == 0 && strings.Contains(stdin, unknownInput) {
			return []string{scriptNotVisible}
		}
		return nested
	}
	return nil
}

// sourced checks a sourced or executed file's text as commands, or stdin when it names /dev/stdin.
func (s *scanner) sourced(kept []string, cwd, branch, stdin string) ([]string, string) {
	if len(kept) < 2 || kept[1] == "/dev/stdin" || kept[1] == "-" {
		if stdin == "" {
			return nil, branch
		}
		nested, next := s.scan(strings.ReplaceAll(stdin, unknownInput, ""), cwd, branch)
		if len(nested) == 0 && strings.Contains(stdin, unknownInput) {
			return []string{scriptNotVisible}, branch
		}
		return nested, next
	}
	if write, found := s.recordedWrite(kept[1], cwd); found {
		if !write.known {
			return []string{scriptNotVisible}, branch
		}
		return s.scan(write.text, cwd, branch)
	}
	file := expandHome(kept[1])
	if !filepath.IsAbs(file) {
		if cwd == unresolvedDir {
			return []string{fmt.Sprintf("%s %s follows a cd the guard cannot resolve", kept[0], kept[1])}, branch
		}
		file = filepath.Join(cwd, file)
	}
	data, err := os.ReadFile(file)
	if (os.IsNotExist(err) && s.createdEarlier()) || (err == nil && s.blind) {
		return []string{scriptNotVisible}, branch
	}
	if err != nil {
		return nil, branch
	}
	return s.scan(string(data), cwd, branch)
}

// unwrapCommand drops a wrapper's own name, assignments, flags, and duration argument to reach the real command.
// It also reports every assignment that overrides a scrubbed variable, since those disappear with the flag.
func unwrapCommand(kept []string) ([]string, []string) {
	var findings []string
	for len(kept) > 0 && wrapperCommands[commandName(kept[0])] {
		name := commandName(kept[0])
		kept = kept[1:]
		for len(kept) > 0 && assignRe.MatchString(kept[0]) {
			if varName, _, _ := strings.Cut(kept[0], "="); isScrubbed(varName) {
				findings = append(findings, fmt.Sprintf("%s overrides a variable the headless run scrubs", kept[0]))
			}
			kept = kept[1:]
		}
		for len(kept) > 0 && strings.HasPrefix(kept[0], "-") {
			flag := kept[0]
			kept = kept[1:]
			if name == "env" {
				findings = append(findings, envFlagFindings(flag)...)
			}
			// env -S's argument is a shell command line, not a plain value; split it like sh -c does.
			if name == "env" && flag == "-S" && len(kept) > 0 {
				kept = append(lexWords(kept[0]), kept[1:]...)
				continue
			}
			if wrapperValueFlags[name][flag] && len(kept) > 0 {
				if name == "env" && (flag == "-u" || flag == "--unset") && isScrubbed(kept[0]) {
					findings = append(findings, fmt.Sprintf("env -u %s removes a variable the headless run scrubs", kept[0]))
				}
				kept = kept[1:]
			}
		}
		if (name == "timeout" || name == "nice") && len(kept) > 0 && durationRe.MatchString(kept[0]) {
			kept = kept[1:]
		}
		if name == "xargs" && len(kept) > 0 {
			kept = append(kept, xargsInput)
		}
	}
	return kept, findings
}
