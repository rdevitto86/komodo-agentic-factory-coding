package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/mount"
)

// Request is the hook payload a host sends on stdin before a tool runs.
type Request struct {
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	Cwd           string         `json:"cwd"`
	ToolInput     map[string]any `json:"tool_input"`
	SessionID     string         `json:"session_id"`
}

// Decision is what the guard concluded and why.
type Decision struct {
	Deny     bool     `json:"deny"`
	Findings []string `json:"findings,omitempty"`

	// rules is Findings' own rule per index, the stable ID the refusal limit counts by.
	rules []string
}

// finding is one refusal: text is what the agent reads, rule the stable ID the refusal limit counts.
type finding struct {
	text string
	rule string
}

// newFinding renders one finding from format and its args, keying its rule on format itself, so
// the same message with a different target still counts toward one refusal-limit rule.
func newFinding(format string, args ...any) finding {
	return finding{text: fmt.Sprintf(format, args...), rule: format}
}

// decide turns one call's findings into a decision: denied once any remain after deduping, each
// paired with the rule the refusal limit counts it under.
func decide(items []finding) Decision {
	var decision Decision
	for _, item := range unique(items) {
		decision.Findings = append(decision.Findings, item.text)
		decision.rules = append(decision.rules, item.rule)
	}
	decision.Deny = len(decision.Findings) > 0
	return decision
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
	var findings []finding
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
		if IsLineSession() && tools.SpawnTools[request.ToolName] && stringField(request.ToolInput, tools.IsolationField) != "" {
			findings = append(findings, newFinding("a spawn never cuts its own worktree; the line already cut it"))
		}
	}
	return decide(findings)
}

// CheckCommand judges a shell command the line runs for a model, as the hook judges one an agent runs.
func CheckCommand(command, cwd string, policy Policy) Decision {
	root := WorktreeRoot(cwd)
	return decide(commandFindings(command, cwd, root, CurrentBranch(cwd), policy))
}

// commandFindings checks each git, gh, and write target in one command, through wrappers, sh -c,
// and eval; a git call is judged on the branch its -C, cd, or a switch left current.
func commandFindings(command, cwd, root, branch string, policy Policy) []finding {
	var findings []finding
	dir := cwd
	for _, item := range nested(command) {
		if item.stdinBody != "" && IsLineSession() {
			findings = append(findings, newFinding("a script read from stdin is refused in a line session; hand the user the command"))
		}
		if item.opaque {
			findings = append(findings, newFinding("eval of an unresolved variable or command substitution is refused; the guard cannot verify it"))
		}
		if len(item.words) > 0 {
			switch commandName(item.words[0]) {
			case "cd":
				if len(item.words) > 1 {
					dir = resolveDir(dir, item.words[1])
					branch = CurrentBranch(dir)
				}
			case "git":
				findings = append(findings, gitFindings(item.words, dir, branchFor(item.words, dir, branch), policy)...)
				branch = afterGit(item.words, dir, branch)
			case "gh":
				findings = append(findings, ghFindings(item.words)...)
			}
		}
		for _, target := range item.writes {
			findings = append(findings, pathFindings(target, cwd, root, policy)...)
		}
		for _, target := range commandWrites(item.words) {
			findings = append(findings, pathFindings(target, cwd, root, policy)...)
		}
	}
	return findings
}

// wrappers run the rest of their words as a command of its own.
var wrappers = map[string]bool{
	"sudo": true, "doas": true, "env": true, "command": true, "exec": true, "nohup": true, "nice": true,
	"time": true, "timeout": true, "xargs": true, "stdbuf": true, "caffeinate": true, "setsid": true,
	"flock": true, "chronic": true, "ionice": true, "unbuffer": true,
}

// shells run the first operand after their -c flag as a command line of its own.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true}

// keywords open or continue a shell compound command, and the call after one still runs.
var keywords = map[string]bool{
	"!": true, "{": true, "}": true, "if": true, "then": true, "else": true, "elif": true,
	"do": true, "while": true, "until": true,
}

// nested splits a command line into calls, each followed by the calls it hands on, and a heredoc
// script's own body. Every step works on strictly shorter input, so the recursion never loops.
func nested(command string) []call {
	var out []call
	for _, item := range tokenize(command) {
		out = append(out, unwrap(item)...)
		if item.stdinBody != "" {
			out = append(out, nested(item.stdinBody)...)
		}
	}
	return out
}

// unwrap returns a call without its keyword or VAR=value prefix, then what a wrapper, shell -c,
// eval, or a quoted command hiding inside one word, like an interpreter's own string argument, runs.
func unwrap(item call) []call {
	words := item.words
	for len(words) > 0 && (keywords[words[0]] || isAssignment(words[0])) {
		words = words[1:]
	}
	out := []call{{words: words, writes: item.writes, stdinBody: item.stdinBody}}
	if len(words) == 0 {
		return out
	}
	if len(words) == 1 {
		if quotedCommand(words[0]) {
			out = append(out, nested(words[0])...)
		}
		return out
	}
	name := commandName(words[0])
	switch {
	case wrappers[name]:
		// A flag's value can name a command too, so every candidate is unwrapped and the findings unioned.
		for index := 1; index < len(words); index++ {
			if inner := commandName(words[index]); inner == "git" || inner == "gh" || inner == "eval" ||
				wrappers[inner] || shells[inner] {
				out = append(out, unwrap(call{words: words[index:]})...)
			}
		}
	case shells[name]:
		if script, ok := shellScript(words[1:]); ok {
			out = append(out, nested(script)...)
		}
	case name == "eval":
		script := strings.Join(words[1:], " ")
		if expanded, ok := expandVars(script); ok {
			out = append(out, nested(expanded)...)
		} else {
			out = append(out, call{opaque: true})
		}
	}
	return out
}

// quotedCommand reports whether rawTokens splits word into several tokens naming git, gh, a
// wrapper, a shell, or eval first, so re-nesting it always shrinks rather than repeating itself.
func quotedCommand(word string) bool {
	fields := rawTokens(word)
	if len(fields) < 2 {
		return false
	}
	name := commandName(fields[0])
	return name == "git" || name == "gh" || name == "eval" || wrappers[name] || shells[name]
}

// shellScript is the first operand after a shell's -c flag, skipping the options between them.
func shellScript(args []string) (string, bool) {
	for index, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg, "c") {
			for _, operand := range args[index+1:] {
				if !strings.HasPrefix(operand, "-") && !strings.HasPrefix(operand, "+") {
					return operand, true
				}
			}
			return "", false
		}
	}
	return "", false
}

// isAssignment reports whether a word is a NAME=value environment prefix.
func isAssignment(word string) bool {
	name, _, ok := strings.Cut(word, "=")
	if !ok || name == "" {
		return false
	}
	for _, char := range name {
		if char != '_' && (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

// commandName is the base name a word runs as.
func commandName(word string) string {
	return filepath.Base(word)
}

// stringField reads one string out of a tool input.
func stringField(input map[string]any, key string) string {
	if value, ok := input[key].(string); ok {
		return value
	}
	return ""
}

// unique keeps the first occurrence of each finding's text, in order.
func unique(items []finding) []finding {
	seen := map[string]bool{}
	var out []finding
	for _, item := range items {
		if !seen[item.text] {
			seen[item.text] = true
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
