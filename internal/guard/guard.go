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

// commandFindings tokenizes one shell command and checks every git or gh subcommand and write
// target it finds; a wrapper, a substitution, or an interpreter's own code hides nothing from it.
func commandFindings(command, cwd, root, branch string, policy Policy) []string {
	var findings []string
	for _, item := range tokenize(command) {
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
