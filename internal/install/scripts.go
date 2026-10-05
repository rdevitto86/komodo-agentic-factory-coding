package install

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"komodo/internal/git"
)

// installers are the only scripts that may hold more than one command: each downloads, verifies and hands off.
var installers = map[string]bool{"install.sh": true, "install.ps1": true}

// installerLines caps an installer's length; anything longer is logic that belongs in komodo install.
const installerLines = 20

// scriptBranch is control flow in a shell or PowerShell script.
var scriptBranch = regexp.MustCompile(`(?m)^\s*(if|case|for|while|until|function|elif)\b|\(\)\s*\{`)

// ScriptProblems names each committed script that holds logic: an installer over its cap or branching past
// its checksum test, or any other script with more than one command.
func ScriptProblems(root string) []string {
	files, err := git.Run(root, "ls-files", "*.sh", "*.bash", "*.ps1", "*.cmd", "*.bat")
	if err != nil {
		return nil
	}
	var problems []string
	for _, rel := range strings.Fields(files) {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		var commands []string
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "::") || strings.HasPrefix(strings.ToLower(trimmed), "rem ") {
				continue
			}
			commands = append(commands, line)
		}
		if !installers[rel] {
			if len(commands) > 1 {
				problems = append(problems, fmt.Sprintf("%s: %d commands; a script is one exec into komodo", rel, len(commands)))
			}
			continue
		}
		if lines := strings.Count(strings.TrimSpace(string(data)), "\n") + 1; lines > installerLines {
			problems = append(problems, fmt.Sprintf("%s: %d lines; an installer is at most %d", rel, lines, installerLines))
		}
		var rest []string
		for _, line := range commands {
			if !strings.Contains(line, "checksum mismatch") {
				rest = append(rest, line)
			}
		}
		if found := scriptBranch.FindString(strings.Join(rest, "\n")); found != "" {
			problems = append(problems, fmt.Sprintf("%s: branches at %q; that logic belongs in komodo install", rel, strings.TrimSpace(found)))
		}
	}
	return problems
}

// pipeline is a shell pipe or command chain inside a skill's code block.
var pipeline = regexp.MustCompile(` \| |&&|\|\|`)

// SkillProblems names each command a model is told to run that chains shell commands; a skill names one
// komodo command per step instead. Standards skills only show code, so they are exempt.
func SkillProblems(root string) []string {
	paths, _ := filepath.Glob(filepath.Join(root, "komodo", "skills", "*", "SKILL.md"))
	var problems []string
	for _, path := range paths {
		name := filepath.Base(filepath.Dir(path))
		if strings.HasPrefix(name, "standards-") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fenced := false
		for number, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fenced = !fenced
				continue
			}
			if fenced && pipeline.MatchString(line) {
				problems = append(problems, fmt.Sprintf("komodo/skills/%s/SKILL.md:%d: a skill chains shell commands; name one komodo command", name, number+1))
			}
		}
	}
	return problems
}
