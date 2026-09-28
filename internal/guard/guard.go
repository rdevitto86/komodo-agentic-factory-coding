package guard

import (
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
		if tools.SpawnTools[request.ToolName] && stringField(request.ToolInput, tools.IsolationField) != "" {
			findings = append(findings, "a spawn never cuts its own worktree; the line already cut it")
		}
	}
	findings = unique(findings)
	return Decision{Deny: len(findings) > 0, Findings: findings}
}

// CheckCommand judges a shell command the line runs for a model, as the hook judges one an agent runs.
func CheckCommand(command, cwd string, policy Policy) Decision {
	root := WorktreeRoot(cwd)
	findings := unique(commandFindings(command, cwd, root, CurrentBranch(cwd), policy))
	return Decision{Deny: len(findings) > 0, Findings: findings}
}

// commandFindings checks each git, gh, and write target in one shell command, looking through wrappers, sh -c, and eval.
func commandFindings(command, cwd, root, branch string, policy Policy) []string {
	var findings []string
	for _, item := range nested(command) {
		if len(item.words) > 0 {
			switch commandName(item.words[0]) {
			case "git":
				findings = append(findings, gitFindings(item.words, branch, policy)...)
			case "gh":
				findings = append(findings, ghFindings(item.words)...)
			}
		}
		for _, target := range item.writes {
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

// nested splits a command line into calls, each followed by the calls it hands on. Every step
// works on strictly shorter input, so the recursion ends without a depth cap.
func nested(command string) []call {
	var out []call
	for _, item := range tokenize(command) {
		out = append(out, unwrap(item)...)
	}
	return out
}

// unwrap returns a call without its keyword or VAR=value prefix, then what a wrapper, shell -c, or eval runs.
func unwrap(item call) []call {
	words := item.words
	for len(words) > 0 && (keywords[words[0]] || isAssignment(words[0])) {
		words = words[1:]
	}
	out := []call{{words: words, writes: item.writes}}
	if len(words) == 0 {
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
		out = append(out, nested(strings.Join(words[1:], " "))...)
	}
	return out
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
