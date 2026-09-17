package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	taskRe    = regexp.MustCompile(`^####\s+\[(TSK-[\w.]+)\]\s+(.+?)\s*\[P:\s*[A-Z]\]\s*\[([A-Z_]+)\]\s*$`)
	groupRe   = regexp.MustCompile(`^###\s+\[(TG-[\w.]+)\]`)
	versionRe = regexp.MustCompile(`^##\s*\[([^\]]+)\]`)
)

// readLines returns a file's lines, or nothing when it cannot be read.
func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// truncate cuts a title to at most n characters, counting them the way the Python hook does.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// backlogLines summarises open, blocked, and in-progress work from a backlog file.
func backlogLines(path string) []string {
	var inProgress []string
	var current, nextGroup string
	open, blocked := 0, 0
	for _, line := range readLines(path) {
		if group := groupRe.FindStringSubmatch(line); group != nil {
			current = group[1]
			continue
		}
		match := taskRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		status := match[3]
		if status == "DONE" {
			continue
		}
		open++
		switch {
		case status == "BLOCKED":
			blocked++
		case status == "IN_PROGRESS" || status == "WIP":
			inProgress = append(inProgress, match[1]+" "+truncate(match[2], 100))
		case nextGroup == "":
			nextGroup = current
		}
	}
	var out []string
	if len(inProgress) > 0 {
		if len(inProgress) > 3 {
			inProgress = inProgress[:3]
		}
		out = append(out, "In progress: "+strings.Join(inProgress, "; "))
	}
	blockedPart := ""
	if blocked > 0 {
		blockedPart = ", " + itoa(blocked) + " blocked"
	}
	if nextGroup == "" {
		nextGroup = "none"
	}
	return append(out, "Backlog: "+itoa(open)+" open"+blockedPart+". Next group: "+nextGroup+".")
}

// itoa renders a non-negative count without pulling in a formatter.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// injectMain prints the repo work state for SessionStart, and stays silent outside a repo.
func injectMain() {
	cwd, _ := os.Getwd()
	root := repoRoot(cwd)
	if root == "" {
		return
	}
	lines := []string{"Work state, read from disk at session start:"}
	backlog := ""
	for _, relative := range []string{"BACKLOG.md", filepath.Join("docs", "BACKLOG.md")} {
		if info, err := os.Stat(filepath.Join(root, relative)); err == nil && !info.IsDir() {
			backlog = filepath.Join(root, relative)
			break
		}
	}
	if backlog == "" {
		lines = append(lines, "No BACKLOG.md; the harness has nothing to run here.")
	} else {
		lines = append(lines, backlogLines(backlog)...)
	}
	for _, line := range readLines(filepath.Join(root, "CHANGELOG.md")) {
		match := versionRe.FindStringSubmatch(line)
		if match != nil && strings.ToLower(match[1]) != "unreleased" {
			lines = append(lines, "Released version: "+match[1]+".")
			break
		}
	}
	gate := ""
	// The names stay slash-separated because they are printed as well as stat'd, and Windows accepts either.
	for _, relative := range []string{".claude/verify.py", "scripts/verify.py", ".claude/verify.sh", "Makefile"} {
		if info, err := os.Stat(filepath.Join(root, relative)); err == nil && !info.IsDir() {
			gate = relative
			if relative == "Makefile" {
				gate = "make verify"
			}
			break
		}
	}
	if gate == "" {
		lines = append(lines, "No verify gate declared.")
	} else {
		lines = append(lines, "Verify gate: "+gate+".")
	}
	lines = append(lines, "Run a task group: python3 -m komodo run [group] [--dry-run].")
	os.Stdout.WriteString(strings.Join(lines, "\n") + "\n")
}
