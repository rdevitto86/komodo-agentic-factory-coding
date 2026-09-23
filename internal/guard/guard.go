package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"komodo/internal/mount"
)

var (
	assignRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	durationRe = regexp.MustCompile(`^[0-9]+[a-zA-Z]*$`)
	// unresolvedVarRe matches a shell variable reference a write target still holds unexpanded.
	unresolvedVarRe = regexp.MustCompile(`\$\{?[A-Za-z_][A-Za-z0-9_]*\}?`)
	// otherHomeRe matches ~name, another user's home, as opposed to ~/ or a bare ~.
	otherHomeRe = regexp.MustCompile(`^~[^/\s]`)
	pathWriters = map[string]bool{
		"rm": true, "mv": true, "cp": true, "tee": true, "dd": true,
		"truncate": true, "install": true, "ln": true, "mkdir": true, "touch": true, "chmod": true,
	}
	// destOnlyWriters read every argument but the last; only the last is where they write.
	destOnlyWriters = map[string]bool{"cp": true, "mv": true, "ln": true, "install": true}
	// wrapperCommands run something else; the guard inspects what they run, not their own name.
	wrapperCommands = map[string]bool{
		"env": true, "sudo": true, "nice": true, "timeout": true, "xargs": true,
		"command": true, "exec": true, "nohup": true, "time": true, "stdbuf": true, "builtin": true,
	}
	// wrapperValueFlags names, per wrapper, the short flags that consume a separate following token.
	wrapperValueFlags = map[string]map[string]bool{
		"sudo":    {"-u": true, "-g": true, "-C": true, "-D": true, "-h": true, "-p": true, "-r": true, "-t": true, "-U": true},
		"env":     {"-u": true, "-C": true},
		"timeout": {"-s": true, "-k": true},
		"nice":    {"-n": true},
		"xargs":   {"-I": true, "-L": true, "-n": true, "-P": true, "-d": true, "-E": true, "-s": true, "-a": true},
		"stdbuf":  {"-i": true, "-o": true, "-e": true},
	}
	// scrubbedVars are the environment variables a headless run's credential scrub sets.
	scrubbedVars = map[string]bool{
		"GIT_CONFIG_COUNT": true, "GIT_CONFIG_PARAMETERS": true, "GIT_SSH_COMMAND": true, "GIT_ASKPASS": true,
	}
	// remotePushConfigRe matches a -c key that redirects where a bare push lands.
	remotePushConfigRe = regexp.MustCompile(`^remote\.[^.]+\.(push|pushurl)$`)
	// globalValueFlags are git's global options that take a value, other than -c and -C.
	globalValueFlags = map[string]bool{
		"--git-dir": true, "--work-tree": true, "--namespace": true,
		"--super-prefix": true, "--exec-path": true, "--config-env": true,
	}
	// configReadFlags mark a git config call as read-only rather than a write.
	configReadFlags = map[string]bool{
		"--get": true, "--get-all": true, "--get-regexp": true, "--get-urlmatch": true,
		"--list": true, "-l": true, "--name-only": true,
	}
)

// Request is the hook payload a host sends on stdin before a tool runs.
type Request struct {
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	Cwd           string         `json:"cwd"`
	ToolInput     map[string]any `json:"tool_input"`
}

// Decision is what the guard concluded and why.
type Decision struct {
	Deny     bool     `json:"deny"`
	Findings []string `json:"findings,omitempty"`
}

// WorktreeRoot walks up from a directory to the nearest one holding .git.
func WorktreeRoot(dir string) string {
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Check returns every reason to refuse one tool call, and nothing when it may run.
func Check(request Request, policy Policy, branch string) Decision {
	root := WorktreeRoot(request.Cwd)
	var findings []string
	for _, tools := range mount.GuardHosts() {
		if tools.WriteTools[request.ToolName] {
			for _, field := range tools.PathFields {
				findings = append(findings, pathFindings(stringField(request.ToolInput, field), root, root, policy)...)
			}
		}
		if tools.ShellTool != "" && request.ToolName == tools.ShellTool {
			command := stringField(request.ToolInput, tools.CommandField)
			findings = append(findings, commandFindings(command, root, root, branch, policy)...)
		}
	}
	findings = unique(findings)
	return Decision{Deny: len(findings) > 0, Findings: findings}
}

// stringField reads one string out of a tool input.
func stringField(input map[string]any, key string) string {
	if value, ok := input[key].(string); ok {
		return value
	}
	return ""
}

// commandFindings returns every reason to refuse one shell command.
func commandFindings(command, root, cwd, branch string, policy Policy) []string {
	// >| is a noclobber override; folding it into >> keeps the pipe splitter from misreading it.
	command = strings.ReplaceAll(stripHeredocs(command), ">|", ">>")
	var findings []string
	// current tracks the branch across segments, since a switch or checkout changes it mid-chain.
	current := branch
	for _, chunk := range splitChain(command) {
		for _, raw := range splitBackground(chunk) {
			segment := unwrapParens(raw)
			tokens, err := splitWords(segment)
			if err != nil {
				tokens = strings.Fields(segment)
			}
			if len(tokens) > 0 && tokens[0] == "{" {
				tokens = tokens[1:]
			}
			var kept []string
			leading := true
			for _, token := range tokens {
				if leading && assignRe.MatchString(token) {
					if name, _, _ := strings.Cut(token, "="); scrubbedVars[name] {
						findings = append(findings, fmt.Sprintf("%s overrides a variable the headless run scrubs", token))
					}
					continue
				}
				leading = false
				kept = append(kept, token)
			}
			if len(kept) == 0 {
				continue
			}
			for _, target := range redirectTargets(segment) {
				findings = append(findings, pathFindings(target, cwd, root, policy)...)
			}
			var wrapFindings []string
			kept, wrapFindings = unwrapCommand(kept)
			findings = append(findings, wrapFindings...)
			if len(kept) == 0 {
				continue
			}
			name := filepath.Base(kept[0])
			switch {
			case name == "cd":
				cwd = changeDir(kept, cwd)
			case name == "dd":
				findings = append(findings, ddPaths(kept, cwd, root, policy)...)
			case pathWriters[name] || isConditionalWriter(name, kept):
				findings = append(findings, writerPaths(name, kept, cwd, root, policy)...)
			}
			if name == "git" {
				var gitResult []string
				gitResult, current = gitFindings(kept, current, cwd, policy)
				findings = append(findings, gitResult...)
			}
			if name == "gh" && len(kept) > 2 && kept[1] == "pr" && kept[2] == "merge" {
				findings = append(findings, "gh pr merge: landing is the human's merge button")
			}
			if name == "eval" && len(kept) > 1 {
				findings = append(findings, commandFindings(kept[1], root, cwd, current, policy)...)
			}
			if script, ok := shellScript(name, kept); ok {
				findings = append(findings, commandFindings(script, root, cwd, current, policy)...)
			}
		}
	}
	return findings
}

// splitChain splits a command into the same segments segmentRe once did, on &&, ||, ;, |, and a
// newline, but never inside a quoted string, so a chain operator quoted into a message stays put.
func splitChain(command string) []string {
	var parts []string
	var current strings.Builder
	quote := rune(0)
	runes := []rune(command)
	flush := func() {
		parts = append(parts, current.String())
		current.Reset()
	}
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		switch {
		case quote != 0:
			current.WriteRune(char)
			if char == quote {
				quote = 0
			}
		case char == '\'' || char == '"':
			quote = char
			current.WriteRune(char)
		case char == '&' && index+1 < len(runes) && runes[index+1] == '&':
			flush()
			index++
		case char == '|' && index+1 < len(runes) && runes[index+1] == '|':
			flush()
			index++
		case char == ';' || char == '|' || char == '\n':
			flush()
		default:
			current.WriteRune(char)
		}
	}
	return append(parts, current.String())
}

// splitBackground further splits a segment on a bare & job-control operator, not &&, >&, or &>.
func splitBackground(segment string) []string {
	runes := []rune(segment)
	var parts []string
	start := 0
	for index, char := range runes {
		if char != '&' {
			continue
		}
		var prev, next rune
		if index > 0 {
			prev = runes[index-1]
		}
		if index+1 < len(runes) {
			next = runes[index+1]
		}
		if prev == '&' || prev == '>' || next == '&' || next == '>' {
			continue
		}
		parts = append(parts, string(runes[start:index]))
		start = index + 1
	}
	return append(parts, string(runes[start:]))
}

// unwrapParens drops the ( and ) a subshell leaves on a segment, which the chain split may separate.
func unwrapParens(segment string) string {
	return strings.TrimRight(strings.TrimLeft(strings.TrimSpace(segment), "( "), ") ")
}

// shells are the interpreters whose -c argument is a command of its own.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true}

// shellScript returns the argument after a shell's -c, alone or in a cluster such as -lc, past any option values.
func shellScript(name string, kept []string) (string, bool) {
	if !shells[name] {
		return "", false
	}
	command := false
	for _, token := range kept[1:] {
		if strings.HasPrefix(token, "-") && !strings.HasPrefix(token, "--") {
			command = command || strings.Contains(token, "c")
			continue
		}
		if command {
			return token, true
		}
	}
	return "", false
}

// unwrapCommand drops a wrapper's own name, assignments, flags, and duration argument to reach the real command.
// It also reports every assignment that overrides a scrubbed variable, since those disappear with the flag.
func unwrapCommand(kept []string) ([]string, []string) {
	var findings []string
	for len(kept) > 0 && wrapperCommands[filepath.Base(kept[0])] {
		name := filepath.Base(kept[0])
		kept = kept[1:]
		for len(kept) > 0 && assignRe.MatchString(kept[0]) {
			if varName, _, _ := strings.Cut(kept[0], "="); scrubbedVars[varName] {
				findings = append(findings, fmt.Sprintf("%s overrides a variable the headless run scrubs", kept[0]))
			}
			kept = kept[1:]
		}
		for len(kept) > 0 && strings.HasPrefix(kept[0], "-") {
			flag := kept[0]
			kept = kept[1:]
			// env -S's argument is a shell command line, not a plain value; split it like sh -c does.
			if name == "env" && flag == "-S" && len(kept) > 0 {
				parsed, err := splitWords(kept[0])
				if err != nil {
					parsed = strings.Fields(kept[0])
				}
				kept = append(parsed, kept[1:]...)
				continue
			}
			if wrapperValueFlags[name][flag] && len(kept) > 0 {
				kept = kept[1:]
			}
		}
		if (name == "timeout" || name == "nice") && len(kept) > 0 && durationRe.MatchString(kept[0]) {
			kept = kept[1:]
		}
	}
	return kept, findings
}

// isConditionalWriter reports whether sed, perl, or find writes a path in this invocation.
func isConditionalWriter(name string, kept []string) bool {
	switch name {
	case "sed", "perl":
		return hasPrefixToken(kept[1:], "-i")
	case "find":
		return containsToken(kept[1:], "-delete")
	}
	return false
}

// hasPrefixToken reports whether any token carries the given prefix.
func hasPrefixToken(tokens []string, prefix string) bool {
	for _, token := range tokens {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}

// containsToken reports whether a token equals the given value.
func containsToken(tokens []string, value string) bool {
	for _, token := range tokens {
		if token == value {
			return true
		}
	}
	return false
}

// changeDir resolves a cd's target against the current directory, so a later write judges
// correctly against wherever the chain now stands, even outside the worktree root.
func changeDir(kept []string, cwd string) string {
	if len(kept) < 2 {
		return cwd
	}
	target := kept[1]
	if unresolvedVarRe.MatchString(target) || otherHomeRe.MatchString(target) || target == "-" {
		return unresolvedDir
	}
	resolved := expandHome(target)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(cwd, resolved)
	}
	return filepath.Clean(resolved)
}

// writerPaths checks the paths a path-writing command actually writes to: every argument,
// unless the command only writes its last, since cp, mv, ln, and install read every other one.
func writerPaths(name string, kept []string, cwd, root string, policy Policy) []string {
	var targets []string
	for _, token := range kept[1:] {
		if strings.HasPrefix(token, "-") {
			continue
		}
		targets = append(targets, token)
	}
	if destOnlyWriters[name] && len(targets) > 1 {
		targets = targets[len(targets)-1:]
	}
	var findings []string
	for _, token := range targets {
		findings = append(findings, pathFindings(token, cwd, root, policy)...)
	}
	return findings
}

// ddPaths checks dd's of= target, the only argument dd writes to.
func ddPaths(kept []string, cwd, root string, policy Policy) []string {
	var findings []string
	for _, token := range kept[1:] {
		if value, ok := strings.CutPrefix(token, "of="); ok {
			findings = append(findings, pathFindings(value, cwd, root, policy)...)
		}
	}
	return findings
}

// isAllowedWrite reports whether a write may always land here: the null device, or a scratch
// file placed directly in a system temp directory, which cost real time to deny.
func isAllowedWrite(path string) bool {
	compare := path
	if foldsCase() {
		compare = strings.ToLower(compare)
	}
	if compare == os.DevNull {
		return true
	}
	for _, dir := range []string{os.TempDir(), "/tmp", "/private/tmp"} {
		dir = strings.TrimRight(dir, string(filepath.Separator))
		if dir == "" {
			continue
		}
		if foldsCase() {
			dir = strings.ToLower(dir)
		}
		if compare == dir || strings.HasPrefix(compare, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// unresolvedDir stands for the working directory after a cd whose target the guard cannot resolve.
const unresolvedDir = "\x00unresolved"

// redirectTargets returns each path an unquoted > or >> writes to, skipping a >& descriptor copy.
func redirectTargets(segment string) []string {
	var out []string
	runes := []rune(segment)
	quote := rune(0)
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			} else if char == '\\' && quote == '"' {
				index++
			}
			continue
		case char == '\\':
			index++
			continue
		case char == '\'' || char == '"':
			quote = char
			continue
		case char != '>':
			continue
		}
		next := index + 1
		if next < len(runes) && runes[next] == '>' {
			next++
		}
		index = next
		if next < len(runes) && runes[next] == '&' {
			continue
		}
		words, err := splitWords(string(runes[next:]))
		if err != nil {
			words = strings.Fields(string(runes[next:]))
		}
		if len(words) > 0 {
			out = append(out, words[0])
		}
	}
	return out
}

// heredocRe matches a heredoc operator and its terminator word, quoted or not.
var heredocRe = regexp.MustCompile(`<<-?\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

// stripHeredocs drops each terminated heredoc body, which is data, unless the line feeds it to a shell.
func stripHeredocs(command string) string {
	lines := strings.Split(command, "\n")
	var out []string
	for index := 0; index < len(lines); index++ {
		out = append(out, lines[index])
		match := heredocRe.FindStringSubmatchIndex(lines[index])
		if match == nil || quotedAt(lines[index], match[0]) || feedsShell(lines[index][:match[0]]) {
			continue
		}
		tag := lines[index][match[2]:match[3]]
		end := index + 1
		for end < len(lines) && strings.TrimSpace(lines[end]) != tag {
			end++
		}
		if end < len(lines) {
			index = end
		}
	}
	return strings.Join(out, "\n")
}

// quotedAt reports whether a byte offset in a line sits inside a quoted string.
func quotedAt(line string, offset int) bool {
	quote := byte(0)
	for index := 0; index < offset; index++ {
		switch char := line[index]; {
		case quote != 0 && char == quote:
			quote = 0
		case quote == 0 && (char == '\'' || char == '"'):
			quote = char
		}
	}
	return quote != 0
}

// feedsShell reports whether the words before a heredoc name a shell, which runs the body as commands.
func feedsShell(prefix string) bool {
	for _, word := range strings.Fields(prefix) {
		name := filepath.Base(word)
		if shells[name] || name == "eval" || name == "source" || name == "." {
			return true
		}
	}
	return false
}

// pathFindings refuses a write that leaves the worktree, lands on a config the hosts own, or
// holds a variable or another user's home the guard cannot resolve, relative to cwd.
func pathFindings(path, cwd, root string, policy Policy) []string {
	if path == "" {
		return nil
	}
	if unresolvedVarRe.MatchString(path) || otherHomeRe.MatchString(path) {
		return []string{fmt.Sprintf("%s holds an unresolved variable or another user's home; the guard cannot judge it", path)}
	}
	resolved := expandHome(path)
	if !filepath.IsAbs(resolved) {
		if cwd == unresolvedDir {
			return []string{fmt.Sprintf("%s follows a cd the guard cannot resolve; use an absolute path", path)}
		}
		resolved = filepath.Join(cwd, resolved)
	}
	resolved = filepath.Clean(resolved)
	var findings []string
	if policy.IsConfigPath(resolved, root) {
		findings = append(findings, fmt.Sprintf("%s is a host or toolkit config; the guard owns it", path))
		return findings
	}
	if isAllowedWrite(resolved) {
		return nil
	}
	if root == "" {
		return findings
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || strings.HasPrefix(relative, "..") {
		findings = append(findings, fmt.Sprintf("%s is outside the worktree root %s", path, root))
	}
	return findings
}

// gitFindings refuses the git operations that touch a critical ref, and reports the branch after the call.
func gitFindings(tokens []string, branch, cwd string, policy Policy) ([]string, string) {
	args := tokens[1:]
	var findings []string
	var configs []string
	index := 0
	for index < len(args) && strings.HasPrefix(args[index], "-") {
		arg := args[index]
		switch {
		case arg == "-C":
			// git -C <dir> reads and writes the branch of that checkout, not this one.
			value := ""
			if index+1 < len(args) {
				value = args[index+1]
			}
			if value != "." {
				return append(findings, fmt.Sprintf("git -C %s: the branch there is not tracked; open a pull request instead", value)), branch
			}
			index += 2
		case arg == "-c":
			if index+1 < len(args) {
				configs = append(configs, args[index+1])
			}
			index += 2
		default:
			name, _, hasValue := strings.Cut(arg, "=")
			index++
			if globalValueFlags[name] && !hasValue {
				index++
			}
		}
	}
	for _, entry := range configs {
		if name, _, _ := strings.Cut(entry, "="); remotePushConfigRe.MatchString(name) {
			findings = append(findings, fmt.Sprintf("git -c %s: a config write reaches push; open a pull request instead", entry))
		}
	}
	if index >= len(args) {
		return findings, branch
	}
	sub, rest := args[index], args[index+1:]
	if alias := aliasValue(configs, sub); strings.HasPrefix(alias, "!") {
		return append(findings, fmt.Sprintf("git -c alias.%s: a shell alias hides its command; run the command directly", sub)), branch
	} else if alias != "" {
		if parts := strings.Fields(alias); len(parts) > 0 {
			sub, rest = parts[0], append(append([]string{}, parts[1:]...), rest...)
		}
	}
	switch sub {
	case "push":
		var positional []string
		deletes := false
		mirrorFlag := ""
		for _, arg := range rest {
			switch arg {
			case "--delete", "-d":
				deletes = true
			case "--mirror", "--all":
				mirrorFlag = arg
			}
			if !strings.HasPrefix(arg, "-") {
				positional = append(positional, arg)
			}
		}
		if mirrorFlag != "" && hasAnyCritical(policy) {
			findings = append(findings, fmt.Sprintf("git push %s: reaches every ref, including a critical one; open a pull request instead", mirrorFlag))
		}
		targets := positional
		if len(positional) > 1 {
			targets = positional[1:]
		} else {
			targets = []string{branch}
		}
		for _, spec := range targets {
			target := strings.TrimPrefix(spec, "+")
			if _, after, found := strings.Cut(target, ":"); found {
				target = after
			}
			if target == "HEAD" {
				target = branch
			}
			if strings.Contains(target, "*") {
				if hasAnyCritical(policy) {
					findings = append(findings, fmt.Sprintf("git push %s: a wildcard refspec reaches every ref, including a critical one; open a pull request instead", spec))
				}
				continue
			}
			if policy.IsCritical(target) {
				verb := "push to"
				if deletes {
					verb = "delete"
				}
				findings = append(findings, fmt.Sprintf("git %s %s: open a pull request instead", verb, target))
			}
		}
	case "commit":
		if policy.IsCritical(branch) {
			findings = append(findings, fmt.Sprintf("git commit on %s: create a branch first", branch))
		}
		if policy.HasTrailer(normalizeMessage(commitMessage(rest, cwd))) {
			findings = append(findings, "commit message carries a co-author or generated-by trailer")
		}
	case "merge":
		if policy.IsCritical(branch) {
			findings = append(findings, fmt.Sprintf("git merge on %s: landing is the human's merge button", branch))
		}
		if policy.HasTrailer(normalizeMessage(commitMessage(rest, cwd))) {
			findings = append(findings, "commit message carries a co-author or generated-by trailer")
		}
	case "branch":
		deleting := false
		for _, arg := range rest {
			if arg == "-d" || arg == "-D" || arg == "--delete" {
				deleting = true
			}
		}
		if !deleting {
			return findings, branch
		}
		for _, arg := range rest {
			if !strings.HasPrefix(arg, "-") && policy.IsCritical(arg) {
				findings = append(findings, fmt.Sprintf("git branch --delete %s: a critical ref is never deleted", arg))
			}
		}
	case "update-ref":
		for _, arg := range rest {
			if policy.IsCritical(arg) {
				findings = append(findings, fmt.Sprintf("git update-ref %s: a critical ref is never moved by hand", arg))
			}
		}
	case "switch", "checkout":
		if target, create, ok := switchTarget(rest); ok {
			if previousBranch(target) && hasAnyCritical(policy) {
				findings = append(findings, fmt.Sprintf("git %s %s: the previous branch is not tracked; name the branch", sub, target))
			}
			if !create && policy.IsCritical(target) {
				findings = append(findings, fmt.Sprintf("git %s %s: a switch onto a critical ref is watched too", sub, target))
			}
			branch = target
		}
	case "config":
		if !hasReadFlag(rest) {
			findings = append(findings, ".git/config is a host or toolkit config; the guard owns it")
		}
	}
	return findings, branch
}

// commitMessage composes the text of a commit or merge's own message: every -m paragraph, an
// -F file's content, and a --trailer's raw key=value line, in the order git reads them.
func commitMessage(rest []string, cwd string) string {
	var parts []string
	for index := 0; index < len(rest); index++ {
		arg := rest[index]
		switch {
		case arg == "-m" || arg == "--message":
			if index+1 < len(rest) {
				index++
				parts = append(parts, rest[index])
			}
		case strings.HasPrefix(arg, "--message="):
			parts = append(parts, strings.TrimPrefix(arg, "--message="))
		case strings.HasPrefix(arg, "-m") && arg != "-m":
			parts = append(parts, strings.TrimPrefix(arg, "-m"))
		case arg == "-F" || arg == "--file":
			if index+1 < len(rest) {
				index++
				parts = append(parts, readMessageFile(rest[index], cwd))
			}
		case strings.HasPrefix(arg, "--file="):
			parts = append(parts, readMessageFile(strings.TrimPrefix(arg, "--file="), cwd))
		case arg == "--trailer":
			if index+1 < len(rest) {
				index++
				parts = append(parts, rest[index])
			}
		case strings.HasPrefix(arg, "--trailer="):
			parts = append(parts, strings.TrimPrefix(arg, "--trailer="))
		}
	}
	return strings.Join(parts, "\n\n")
}

// messageBreakRe is a ; or && a message smuggles a trailer past, read as a line break instead.
var messageBreakRe = regexp.MustCompile(`\s*(?:&&|;)\s*`)

// normalizeMessage turns a message's own ; and && into line breaks before the trailer check.
func normalizeMessage(text string) string {
	return messageBreakRe.ReplaceAllString(text, "\n")
}

// readMessageFile reads a commit message file relative to cwd, tolerating a missing one.
func readMessageFile(path, cwd string) string {
	if path == "-" {
		return ""
	}
	resolved := expandHome(path)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(cwd, resolved)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return ""
	}
	return string(data)
}

// aliasValue returns the git alias a -c option set for a subcommand name, or the empty string.
func aliasValue(configs []string, sub string) string {
	for _, entry := range configs {
		if name, value, found := strings.Cut(entry, "="); found && name == "alias."+sub {
			return value
		}
	}
	return ""
}

// hasAnyCritical reports whether the policy protects any ref at all.
func hasAnyCritical(policy Policy) bool {
	return len(policy.CriticalRefs) > 0
}

// switchTarget finds the ref a switch or checkout targets, and whether it creates a new one.
func switchTarget(rest []string) (target string, create bool, ok bool) {
	for index, arg := range rest {
		switch arg {
		case "-b", "-B", "-c", "--create", "--orphan":
			if index+1 < len(rest) {
				return rest[index+1], true, true
			}
			return "", false, false
		case "--":
			return "", false, false
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return arg, false, true
		}
	}
	return "", false, false
}

// previousBranch reports whether a switch target names an earlier branch, as - and @{-1} do.
func previousBranch(target string) bool {
	return target == "-" || strings.HasPrefix(target, "@{-")
}

// hasReadFlag reports whether a git config call only reads, never writes.
func hasReadFlag(rest []string) bool {
	for _, arg := range rest {
		if configReadFlags[arg] {
			return true
		}
	}
	return false
}

// unique keeps the first occurrence of each finding, in order.
func unique(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// Reason renders the denial a host shows the agent.
func Reason(findings []string) string {
	return "Refused. Nothing ran.\n\n  - " + strings.Join(findings, "\n  - ") +
		"\n\nInside your worktree you are free. Hand the user the command if it was intended."
}
