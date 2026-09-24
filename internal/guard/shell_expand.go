package guard

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

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
