package guard

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
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
	known = !strings.Contains(text, "\\") && !(name == "printf" && strings.Contains(text, "%"))
	return text, true, known
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
	var findings []string
	for _, target := range cmd.writes {
		findings = append(findings, pathFindings(target, cwd, s.root, s.policy)...)
	}
	if len(cmd.writes) > 0 {
		s.recordWrites(cmd, cwd, strings.Join(append(append([]string{}, cmd.stdin...), upstream...), "\n"))
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
	stdin := strings.Join(append(append([]string{}, cmd.stdin...), upstream...), "\n")
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
			findings = append(findings, s.interpScriptFindings(kept, cwd, stdin)...)
		}
		if name == "tee" {
			s.recordTeeWrites(kept, cwd, stdin)
		}
	case name == "git":
		var gitResult []string
		gitResult, branch = gitFindings(kept, branch, cwd, s.policy, stdin)
		findings = append(findings, gitResult...)
	case name == "gh":
		findings = append(findings, ghFindings(kept)...)
	case interpreters[interpName(name)]:
		findings = append(findings, s.interpScriptFindings(kept, cwd, stdin)...)
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

// variableRe matches a whole word that is one plain variable reference, $NAME or ${NAME}.
var variableRe = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)

// variableRefRe matches a variable reference anywhere inside a word.
var variableRefRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// expand turns words into the argument strings the shell passes, substituting variables this line set.
func (s *scanner) expand(words []word) []string {
	var out []string
	for _, w := range words {
		if match := variableRe.FindStringSubmatch(w.value); match != nil {
			if value, ok := s.vars[match[1]]; ok {
				out = append(out, strings.Fields(value)...)
				continue
			}
		}
		out = append(out, variableRefRe.ReplaceAllStringFunc(w.value, func(ref string) string {
			parts := variableRefRe.FindStringSubmatch(ref)
			name := parts[1] + parts[2]
			if value, ok := s.vars[name]; ok {
				return value
			}
			return ref
		}))
	}
	return out
}

// remember records name=value assignments so a later $name in the same line resolves.
func (s *scanner) remember(assigns []string) {
	for _, assign := range assigns {
		if name, value, found := strings.Cut(assign, "="); found && assignRe.MatchString(assign) {
			s.vars[name] = value
		}
	}
}

// inspectedNames are the commands the guard reads the arguments of, so a glob that names one is resolved.
var inspectedNames = func() []string {
	names := []string{"git", "gh", "eval", "source", "cd", "pushd", "popd", "dd", "sed", "perl", "find", "printf", "read"}
	for _, set := range []map[string]bool{shells, pathWriters, wrapperCommands, assignBuiltins} {
		for name := range set {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}()

// commandName is the base name a command runs as, resolving a glob such as gi? to what it can match.
func commandName(first string) string {
	name := filepath.Base(first)
	if !strings.ContainsAny(name, "*?[") {
		return name
	}
	for _, candidate := range inspectedNames {
		if matched, _ := path.Match(name, candidate); matched {
			return candidate
		}
	}
	return name
}

// isScrubbed reports whether a variable is one the headless scrub sets, removes, or relies on.
func isScrubbed(name string) bool {
	return scrubbedVars[name] || scrubbedConfigRe.MatchString(name)
}

// assignBuiltins set, export, or clear a shell variable by name.
var assignBuiltins = map[string]bool{
	"export": true, "unset": true, "declare": true, "typeset": true, "readonly": true, "local": true, "let": true,
}

// shells are the interpreters whose -c argument, script file, or stdin is commands of its own.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true}

// shellValueOptions are a shell's options that consume the next word.
var shellValueOptions = map[string]bool{"-o": true, "+o": true, "-O": true, "+O": true, "--rcfile": true, "--init-file": true}

// shellScript returns a shell's -c argument, alone or in a cluster such as -lc, or else its script file operand.
func shellScript(kept []string) (script, operand string) {
	command := false
	for index := 1; index < len(kept); index++ {
		token := kept[index]
		if shellValueOptions[token] {
			index++
			continue
		}
		if token == "--" {
			continue
		}
		if (strings.HasPrefix(token, "-") || strings.HasPrefix(token, "+")) && len(token) > 1 {
			if !strings.HasPrefix(token, "--") {
				command = command || strings.Contains(token, "c")
			}
			continue
		}
		if command {
			return token, ""
		}
		return "", token
	}
	return "", ""
}

// envFlagFindings refuses an env option that clears the environment or unsets a scrubbed variable.
func envFlagFindings(flag string) []string {
	switch {
	case flag == "-" || flag == "-i" || flag == "--ignore-environment":
		return []string{fmt.Sprintf("env %s clears the variables the headless run scrubs", flag)}
	case strings.HasPrefix(flag, "-u") && len(flag) > 2 && isScrubbed(flag[2:]):
		return []string{fmt.Sprintf("env %s removes a variable the headless run scrubs", flag)}
	case strings.HasPrefix(flag, "--unset="):
		if name := strings.TrimPrefix(flag, "--unset="); isScrubbed(name) {
			return []string{fmt.Sprintf("env %s removes a variable the headless run scrubs", flag)}
		}
	}
	return nil
}

// lexWords splits text into the plain words a shell would pass, ignoring its operators.
func lexWords(text string) []string {
	var out []string
	for _, t := range lex(text).tokens {
		if t.kind == tokenWord {
			for _, w := range expandBraces(t.word) {
				out = append(out, w.value)
			}
		}
	}
	return out
}

// xargsInput stands in for the words xargs feeds its command at run time, whatever its placeholder.
const xargsInput = "{}"

// resolveWritePath resolves a redirect or tee target against cwd the same way pathFindings does,
// reporting false when the guard cannot resolve it.
func resolveWritePath(target, cwd string) (string, bool) {
	if target == "" || unresolvedVarRe.MatchString(target) || otherHomeRe.MatchString(target) {
		return "", false
	}
	resolved := expandHome(target)
	if !filepath.IsAbs(resolved) {
		if cwd == unresolvedDir {
			return "", false
		}
		resolved = filepath.Join(cwd, resolved)
	}
	return filepath.Clean(resolved), true
}

// writeContent is the text a redirect puts into its target: what echo or printf prints, or the
// stdin a bare cat copies; any other command's output is unknown, since it may transform stdin.
func writeContent(cmd simpleCommand, stdin string) (string, bool) {
	if text, printed, known := echoedText(cmd); printed {
		return text, known
	}
	if len(cmd.words) == 0 {
		return "", true
	}
	if !passesLiterally(cmd) || strings.Contains(stdin, unknownInput) {
		return "", false
	}
	return stdin, true
}

// recordWrites remembers what this line put into each redirect target, so a later read judges that
// content, not disk; an append adds to what the file already held.
func (s *scanner) recordWrites(cmd simpleCommand, cwd, stdin string) {
	text, known := writeContent(cmd, stdin)
	for _, target := range cmd.writes {
		resolved, ok := resolveWritePath(target, cwd)
		if !ok {
			continue
		}
		s.put(resolved, cwd, scriptWrite{text: text, known: known}, cmd.appends[target])
	}
}

// put records one write to a resolved path, adding to what the line or disk held when it appends.
func (s *scanner) put(resolved, cwd string, write scriptWrite, appends bool) {
	if appends {
		before, found := s.writes[resolved]
		if !found {
			disk, exists, readable := readScript(resolved, cwd)
			before = scriptWrite{text: disk, known: !exists || readable}
		}
		write = scriptWrite{text: before.text + "\n" + write.text, known: before.known && write.known}
	}
	s.writes[resolved] = write
}

// recordOutputWrites marks files a command writes that the guard cannot read: curl -o, wget -O,
// dd of=, sed -i and perl -i edits, and the destination of cp, mv, install, and ln.
func (s *scanner) recordOutputWrites(name string, kept []string, cwd string) {
	var targets []string
	switch name {
	case "curl", "wget":
		short, long := "-o", "--output"
		if name == "wget" {
			short, long = "-O", "--output-document"
		}
		for index := 1; index < len(kept); index++ {
			arg := kept[index]
			switch {
			case arg == short || arg == long:
				if index+1 < len(kept) {
					targets = append(targets, kept[index+1])
					index++
				}
			case strings.HasPrefix(arg, long+"="):
				targets = append(targets, strings.TrimPrefix(arg, long+"="))
			case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--"):
				// curl -sSLo x and wget -qO x carry the output letter inside a cluster.
				if at := strings.IndexByte(arg[1:], short[1]); at >= 0 {
					if rest := arg[at+2:]; rest != "" {
						targets = append(targets, rest)
					} else if index+1 < len(kept) {
						targets = append(targets, kept[index+1])
						index++
					}
				}
			}
		}
	case "sed", "perl":
		if isConditionalWriter(name, kept) {
			targets = writerTargets(name, kept)
		}
	case "dd":
		for _, arg := range kept[1:] {
			if value, ok := strings.CutPrefix(arg, "of="); ok {
				targets = append(targets, value)
			}
		}
	case "cp", "mv", "install", "ln":
		var operands []string
		for _, arg := range kept[1:] {
			if !strings.HasPrefix(arg, "-") {
				operands = append(operands, arg)
			}
		}
		if len(operands) >= 2 {
			targets = append(targets, operands[len(operands)-1])
		}
	}
	for _, target := range targets {
		if resolved, ok := resolveWritePath(target, cwd); ok {
			s.writes[resolved] = scriptWrite{}
		}
	}
	s.blind = s.blind || writesBlind(name, kept)
}

// blindGitVerbs are git subcommands that rewrite worktree files the guard never sees.
var blindGitVerbs = set("checkout", "restore", "switch", "reset", "apply", "am", "pull", "merge",
	"rebase", "cherry-pick", "stash", "clone", "revert")

// writesBlind reports whether a command rewrites files under names the guard cannot list: curl -O,
// an archive extract, a patch, or a git call that changes the worktree.
func writesBlind(name string, kept []string) bool {
	switch name {
	case "curl":
		for _, arg := range kept[1:] {
			if arg == "--remote-name" || arg == "--remote-name-all" ||
				(strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg, "O")) {
				return true
			}
		}
	case "tar", "bsdtar":
		for index, arg := range kept[1:] {
			short := strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--")
			// tar's mode is a dash cluster, or its first word in the old form tar xf a.tgz.
			if arg == "--extract" || arg == "--get" || ((short || index == 0) && strings.Contains(arg, "x")) {
				return true
			}
		}
	case "unzip", "patch", "rsync", "scp", "gunzip", "bunzip2", "unxz":
		return true
	case "7z", "7za", "7zz":
		return len(kept) > 1 && (kept[1] == "x" || kept[1] == "e")
	case "git":
		for index := 1; index < len(kept); index++ {
			arg := kept[index]
			if arg == "-C" || arg == "-c" {
				index++
				continue
			}
			if !strings.HasPrefix(arg, "-") {
				return blindGitVerbs[arg]
			}
		}
	}
	return false
}

// recordTeeWrites remembers what tee's stdin put into each file it names.
func (s *scanner) recordTeeWrites(kept []string, cwd, stdin string) {
	known := stdin != "" && !strings.Contains(stdin, unknownInput)
	appends := false
	var files []string
	for _, token := range kept[1:] {
		switch {
		case token == "--append" || (strings.HasPrefix(token, "-") && !strings.HasPrefix(token, "--") && strings.Contains(token, "a")):
			appends = true
		case token != "" && !strings.HasPrefix(token, "-"):
			files = append(files, token)
		}
	}
	for _, file := range files {
		if resolved, ok := resolveWritePath(file, cwd); ok {
			s.put(resolved, cwd, scriptWrite{text: stdin, known: known}, appends)
		}
	}
}

// recordedWrite returns what this command line already wrote to target, when the guard saw it happen.
func (s *scanner) recordedWrite(target, cwd string) (scriptWrite, bool) {
	resolved, ok := resolveWritePath(target, cwd)
	if !ok {
		return scriptWrite{}, false
	}
	write, found := s.writes[resolved]
	return write, found
}

// createdEarlier reports whether an earlier command in this line could have written a script
// the guard now finds missing on disk, as tar xf a.tgz && sh install.sh would.
func (s *scanner) createdEarlier() bool {
	return s.seen > 1
}

// looksLikeScriptPath reports whether a command names itself by a path, as ./x.sh or bin/x.sh do,
// rather than a bare name a shell resolves through PATH.
func looksLikeScriptPath(target string) bool {
	return strings.Contains(target, "/")
}

// scriptCommandFindings checks a command run by path: a recorded write, or a text file scanned as sh
// under a shell shebang or none, or read for hidden git or gh under an interpreter's.
func (s *scanner) scriptCommandFindings(target, cwd, branch string) ([]string, string) {
	text, exists, ok := readScript(target, cwd)
	if write, found := s.recordedWrite(target, cwd); found {
		if !write.known {
			return []string{scriptNotVisible}, branch
		}
		text, exists, ok = write.text, true, true
	}
	if !exists && scriptExtensions[strings.ToLower(filepath.Ext(target))] && s.createdEarlier() {
		return []string{scriptNotVisible}, branch
	}
	if _, recorded := s.recordedWrite(target, cwd); !recorded && ok && s.blind {
		return []string{scriptNotVisible}, branch
	}
	if !ok {
		return nil, branch
	}
	switch interp := shebang(text); {
	case interp == "" || shells[interp]:
		return s.scan(text, cwd, branch)
	case interpreters[interpName(interp)] && hidesGitOrGhInFile(text):
		return []string{interpreterHidesGit}, branch
	}
	return nil, branch
}

// shebang names the interpreter a script's first line asks for, through env when it uses it.
func shebang(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	rest, ok := strings.CutPrefix(line, "#!")
	if !ok {
		return ""
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	interp := filepath.Base(fields[0])
	if interp == "env" {
		for _, field := range fields[1:] {
			if !strings.HasPrefix(field, "-") {
				return filepath.Base(field)
			}
		}
	}
	return interp
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
