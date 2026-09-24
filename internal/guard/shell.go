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
}

// commandFindings returns every reason to refuse one shell command.
func commandFindings(command, root, cwd, branch string, policy Policy) []string {
	s := &scanner{root: root, policy: policy, vars: map[string]string{}}
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
	for _, previous := range commands[start:index] {
		input = append(input, upstreamWords(previous)...)
	}
	return input
}

// upstreamWords is one command's own contribution to piped input: its heredocs, here-strings, and echoed words.
func upstreamWords(cmd simpleCommand) []string {
	input := append([]string{}, cmd.stdin...)
	if len(cmd.words) > 1 {
		if name := filepath.Base(cmd.words[0].value); name == "echo" || name == "printf" {
			var args []string
			for _, w := range cmd.words[1:] {
				if !strings.HasPrefix(w.value, "-") {
					args = append(args, w.value)
				}
			}
			input = append(input, strings.Join(args, " "))
		}
	}
	return input
}

// command checks one simple command and returns its findings, the directory after it, and the branch.
func (s *scanner) command(cmd simpleCommand, upstream []string, cwd, branch string) ([]string, string, string) {
	var findings []string
	for _, target := range cmd.writes {
		findings = append(findings, pathFindings(target, cwd, s.root, s.policy)...)
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
	case name == "git":
		var gitResult []string
		gitResult, branch = gitFindings(kept, branch, cwd, s.policy, stdin)
		findings = append(findings, gitResult...)
	case name == "gh" && len(kept) > 2 && kept[1] == "pr" && kept[2] == "merge":
		findings = append(findings, "gh pr merge: landing is the human's merge button")
	case name == "eval" && len(kept) > 1:
		var evaluated []string
		evaluated, branch = s.scan(strings.Join(kept[1:], " "), cwd, branch)
		findings = append(findings, evaluated...)
	case shells[name]:
		findings = append(findings, s.shell(kept, cwd, branch, stdin)...)
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
		nested, _ := s.scan(stdin, cwd, branch)
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
		return s.scan(stdin, cwd, branch)
	}
	file := expandHome(kept[1])
	if !filepath.IsAbs(file) {
		if cwd == unresolvedDir {
			return []string{fmt.Sprintf("%s %s follows a cd the guard cannot resolve", kept[0], kept[1])}, branch
		}
		file = filepath.Join(cwd, file)
	}
	data, err := os.ReadFile(file)
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
