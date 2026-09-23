package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	segmentRe = regexp.MustCompile(`\s*(?:&&|\|\||[;|\n])\s*`)
	// commandRe splits shell commands without splitting a multi-line commit message.
	commandRe   = regexp.MustCompile(`\s*(?:&&|\|\||[;|])\s*`)
	assignRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	gitWriteRe  = regexp.MustCompile(`\bgit\b[^\n;|&]*\b(?:commit|merge)\b`)
	redirectRe  = regexp.MustCompile(`>>?\s*([^\s;|&<>]+)`)
	durationRe  = regexp.MustCompile(`^[0-9]+[a-zA-Z]*$`)
	writeTools  = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}
	pathWriters = map[string]bool{
		"rm": true, "mv": true, "cp": true, "tee": true, "dd": true,
		"truncate": true, "install": true, "ln": true, "mkdir": true, "touch": true, "chmod": true,
	}
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
	if writeTools[request.ToolName] {
		findings = append(findings, pathFindings(stringField(request.ToolInput, "file_path"), root, policy)...)
		findings = append(findings, pathFindings(stringField(request.ToolInput, "notebook_path"), root, policy)...)
	}
	if request.ToolName == "Bash" {
		findings = append(findings, commandFindings(stringField(request.ToolInput, "command"), root, branch, policy)...)
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
func commandFindings(command, root, branch string, policy Policy) []string {
	// >| is a noclobber override; folding it into >> keeps the pipe splitter from misreading it.
	command = strings.ReplaceAll(command, ">|", ">>")
	var findings []string
	for _, part := range commandRe.Split(command, -1) {
		if gitWriteRe.MatchString(part) && policy.HasTrailer(part) {
			findings = append(findings, "commit message carries a co-author or generated-by trailer")
			break
		}
	}
	// current tracks the branch across segments, since a switch or checkout changes it mid-chain.
	current := branch
	for _, chunk := range segmentRe.Split(command, -1) {
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
			for _, match := range redirectRe.FindAllStringSubmatch(segment, -1) {
				findings = append(findings, pathFindings(match[1], root, policy)...)
			}
			var wrapFindings []string
			kept, wrapFindings = unwrapCommand(kept)
			findings = append(findings, wrapFindings...)
			if len(kept) == 0 {
				continue
			}
			name := filepath.Base(kept[0])
			switch {
			case name == "dd":
				findings = append(findings, ddPaths(kept, root, policy)...)
			case pathWriters[name] || isConditionalWriter(name, kept):
				findings = append(findings, writerPaths(kept, root, policy)...)
			}
			if name == "git" {
				var gitResult []string
				gitResult, current = gitFindings(kept, current, policy)
				findings = append(findings, gitResult...)
			}
			if name == "gh" && len(kept) > 2 && kept[1] == "pr" && kept[2] == "merge" {
				findings = append(findings, "gh pr merge: landing is the human's merge button")
			}
			if name == "eval" && len(kept) > 1 {
				findings = append(findings, commandFindings(kept[1], root, current, policy)...)
			}
			if script, ok := shellScript(name, kept); ok {
				findings = append(findings, commandFindings(script, root, current, policy)...)
			}
		}
	}
	return findings
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

// writerPaths checks every non-flag argument of a path-writing command.
func writerPaths(kept []string, root string, policy Policy) []string {
	var findings []string
	for _, token := range kept[1:] {
		if strings.HasPrefix(token, "-") {
			continue
		}
		findings = append(findings, pathFindings(token, root, policy)...)
	}
	return findings
}

// ddPaths checks dd's of= target, the only argument dd writes to.
func ddPaths(kept []string, root string, policy Policy) []string {
	var findings []string
	for _, token := range kept[1:] {
		if value, ok := strings.CutPrefix(token, "of="); ok {
			findings = append(findings, pathFindings(value, root, policy)...)
		}
	}
	return findings
}

// pathFindings refuses a write that leaves the worktree or lands on a config the hosts own.
func pathFindings(path, root string, policy Policy) []string {
	if path == "" {
		return nil
	}
	resolved := expandHome(path)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, resolved)
	}
	resolved = filepath.Clean(resolved)
	var findings []string
	if policy.IsConfigPath(resolved, root) {
		findings = append(findings, fmt.Sprintf("%s is a host or toolkit config; the guard owns it", path))
		return findings
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
func gitFindings(tokens []string, branch string, policy Policy) ([]string, string) {
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
	case "merge":
		if policy.IsCritical(branch) {
			findings = append(findings, fmt.Sprintf("git merge on %s: landing is the human's merge button", branch))
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
