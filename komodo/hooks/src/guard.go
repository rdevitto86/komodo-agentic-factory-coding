package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The bracketed literals keep this source from tripping the very rule it implements.
var (
	trailerRe   = regexp.MustCompile(`(?i)co-authored[-]by\s*:|generated[ ]with|generated[ ]by|\x{1F916}`)
	segmentRe   = regexp.MustCompile(`\s*(?:&&|\|\||[;|\n])\s*`)
	assignRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	gitCommitRe = regexp.MustCompile(`\bgit\b[^\n;|&]*\b(?:commit|merge)\b`)
)

var destructive = map[string]string{
	"rebase":        "rebases rewrite history",
	"reset":         "reset discards work",
	"checkout":      "checkout discards uncommitted work; move with switch",
	"restore":       "restore discards uncommitted work",
	"clean":         "clean deletes untracked files",
	"filter-branch": "filter-branch rewrites history",
	"filter-repo":   "filter-repo rewrites history",
	"update-ref":    "update-ref bypasses the branch model",
}

// gitFindings returns every reason to deny one git invocation.
func gitFindings(tokens []string, cwd string, patterns []string) []string {
	args := tokens[1:]
	index := 0
	for index < len(args) && strings.HasPrefix(args[index], "-") {
		switch args[index] {
		case "-C", "-c", "--git-dir", "--work-tree":
			if args[index] == "-C" && index+1 < len(args) {
				cwd = args[index+1]
			}
			index += 2
		default:
			index++
		}
	}
	if index >= len(args) {
		return nil
	}
	sub, rest := args[index], args[index+1:]

	if reason, ok := destructive[sub]; ok {
		return []string{fmt.Sprintf("git %s: %s", sub, reason)}
	}

	switch sub {
	case "push":
		var out []string
		for _, arg := range rest {
			if arg == "-f" || arg == "--force-with-lease" || arg == "--force-if-includes" ||
				arg == "--mirror" || arg == "--delete" || arg == "-d" ||
				strings.HasPrefix(arg, "--force") || strings.HasPrefix(arg, "+") {
				out = append(out, "git push: force or delete is never part of the flow")
				break
			}
		}
		var positional []string
		for _, arg := range rest {
			if !strings.HasPrefix(arg, "-") {
				positional = append(positional, arg)
			}
		}
		if len(positional) > 1 {
			for _, spec := range positional[1:] {
				destination := spec
				if _, after, found := strings.Cut(spec, ":"); found {
					destination = after
				}
				if destination == "HEAD" {
					destination = currentBranch(cwd)
				}
				if isProtected(destination, patterns) {
					out = append(out, fmt.Sprintf("git push to %s: open a pull request instead", destination))
				}
			}
		} else {
			if branch := currentBranch(cwd); branch != "" && isProtected(branch, patterns) {
				out = append(out, fmt.Sprintf("git push from %s: open a pull request instead", branch))
			}
		}
		return out
	case "commit":
		var out []string
		for _, arg := range rest {
			if arg == "--amend" || arg == "--no-verify" || arg == "-n" {
				out = append(out, "git commit: --amend and --no-verify are refused")
				break
			}
		}
		if branch := currentBranch(cwd); branch != "" && isProtected(branch, patterns) {
			out = append(out, fmt.Sprintf("git commit on %s: create a branch first", branch))
		}
		return out
	case "merge":
		if branch := currentBranch(cwd); branch != "" && isProtected(branch, patterns) {
			return []string{fmt.Sprintf("git merge on %s: landing is the human's merge button", branch)}
		}
	case "switch":
		for _, arg := range rest {
			if arg == "-f" || arg == "--force" || arg == "--discard-changes" {
				return []string{"git switch: --discard-changes throws away uncommitted work"}
			}
		}
	case "branch":
		for _, arg := range rest {
			if arg == "-D" || arg == "-f" || arg == "--force" {
				return []string{"git branch: forced delete or move is refused"}
			}
		}
	}
	return nil
}

// analyze returns every reason to deny a Bash command.
func analyze(command, cwd string) []string {
	var findings []string
	patterns := protectedPatterns(cwd)

	if gitCommitRe.MatchString(command) && trailerRe.MatchString(command) {
		findings = append(findings, "commit message carries a co-author or generated-by trailer")
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
		switch name := filepath.Base(kept[0]); name {
		case "sudo":
			findings = append(findings, "sudo is refused; run it yourself")
		case "rm":
			var short strings.Builder
			recursive, force := false, false
			for _, token := range kept[1:] {
				if strings.HasPrefix(token, "-") && !strings.HasPrefix(token, "--") {
					short.WriteString(token[1:])
				}
				if token == "--recursive" {
					recursive = true
				}
				if token == "--force" {
					force = true
				}
			}
			flags := short.String()
			if (strings.Contains(flags, "r") && strings.Contains(flags, "f")) || (recursive && force) {
				findings = append(findings, "rm -rf is refused; delete deliberately by hand")
			}
		case "git":
			findings = append(findings, gitFindings(kept, cwd, patterns)...)
		case "gh":
			if len(kept) > 2 && kept[1] == "pr" && kept[2] == "merge" {
				findings = append(findings, "gh pr merge: landing is the human's merge button")
			}
		}
	}
	return findings
}

type payload struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
	Cwd string `json:"cwd"`
}

// guardMain reads the tool payload, denies on findings, and stays silent otherwise; any internal error fails open.
func guardMain() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	var p payload
	if json.Unmarshal(raw, &p) != nil || p.ToolName != "Bash" {
		return
	}
	cwd := p.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	findings := analyze(p.ToolInput.Command, cwd)
	if len(findings) == 0 {
		return
	}

	var unique []string
	seen := map[string]bool{}
	for _, item := range findings {
		if !seen[item] {
			seen[item] = true
			unique = append(unique, item)
		}
	}
	reason := "Refused. Nothing ran.\n\n  - " + strings.Join(unique, "\n  - ") +
		"\n\nHand the user the exact command if it was intended."

	out, err := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	})
	if err != nil {
		return
	}
	os.Stdout.Write(out)
}
