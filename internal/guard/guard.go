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
	writeTools  = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}
	pathWriters = map[string]bool{
		"rm": true, "mv": true, "cp": true, "tee": true, "dd": true,
		"truncate": true, "install": true, "ln": true, "mkdir": true, "touch": true, "chmod": true,
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
	var findings []string
	for _, part := range commandRe.Split(command, -1) {
		if gitWriteRe.MatchString(part) && policy.HasTrailer(part) {
			findings = append(findings, "commit message carries a co-author or generated-by trailer")
			break
		}
	}
	for _, segment := range segmentRe.Split(command, -1) {
		tokens, err := splitWords(segment)
		if err != nil {
			tokens = strings.Fields(segment)
		}
		var kept []string
		for _, token := range tokens {
			if !assignRe.MatchString(token) {
				kept = append(kept, token)
			}
		}
		if len(kept) == 0 {
			continue
		}
		for _, match := range redirectRe.FindAllStringSubmatch(segment, -1) {
			findings = append(findings, pathFindings(match[1], root, policy)...)
		}
		name := filepath.Base(kept[0])
		if pathWriters[name] {
			for _, token := range kept[1:] {
				if strings.HasPrefix(token, "-") {
					continue
				}
				findings = append(findings, pathFindings(token, root, policy)...)
			}
		}
		if name == "git" {
			findings = append(findings, gitFindings(kept, branch, policy)...)
		}
		if name == "gh" && len(kept) > 2 && kept[1] == "pr" && kept[2] == "merge" {
			findings = append(findings, "gh pr merge: landing is the human's merge button")
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

// gitFindings refuses the git operations that touch a critical ref.
func gitFindings(tokens []string, branch string, policy Policy) []string {
	args := tokens[1:]
	index := 0
	for index < len(args) && strings.HasPrefix(args[index], "-") {
		switch args[index] {
		case "-C", "-c", "--git-dir", "--work-tree":
			index += 2
		default:
			index++
		}
	}
	if index >= len(args) {
		return nil
	}
	sub, rest := args[index], args[index+1:]
	var findings []string
	switch sub {
	case "push":
		var positional []string
		deletes := false
		for _, arg := range rest {
			if arg == "--delete" || arg == "-d" || arg == "--mirror" {
				deletes = true
			}
			if !strings.HasPrefix(arg, "-") {
				positional = append(positional, arg)
			}
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
			return nil
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
	}
	return findings
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
