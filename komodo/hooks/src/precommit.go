package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// commitMessage reads the message git is about to record, from the override or the git dir.
func commitMessage(root string) string {
	path := os.Getenv("KOMODO_COMMIT_MSG_FILE")
	if path == "" {
		gitDir := gitOutput(root, "rev-parse", "--git-dir")
		if gitDir == "" {
			return ""
		}
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(root, gitDir)
		}
		path = filepath.Join(gitDir, "COMMIT_EDITMSG")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// gofmtProblems reports staged Go files whose staged content is not gofmt-clean.
func gofmtProblems(root string, files []string) []string {
	var goFiles []string
	for _, path := range files {
		if strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
	}
	if len(goFiles) == 0 {
		return nil
	}
	if _, err := exec.LookPath("gofmt"); err != nil {
		return nil
	}
	var problems []string
	for _, path := range goFiles {
		// The staged blob is what the commit records, which can differ from the file on disk.
		staged := exec.Command("git", "show", ":"+path)
		staged.Dir = root
		content, err := staged.Output()
		if err != nil {
			continue
		}
		scratch, err := os.CreateTemp("", "komodo-*.go")
		if err != nil {
			continue
		}
		name := scratch.Name()
		_, writeErr := scratch.Write(content)
		scratch.Close()
		if writeErr == nil {
			if out, err := exec.Command("gofmt", "-l", name).Output(); err == nil && strings.TrimSpace(string(out)) != "" {
				problems = append(problems, fmt.Sprintf("%s is not gofmt-clean; run gofmt -w %s", path, path))
			}
		}
		os.Remove(name)
	}
	return problems
}

type finding struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
	Rule   string `json:"rule"`
	Detail string `json:"detail"`
}

// commentFindings runs the komodo comment lint over staged lines; an unavailable lint skips rather than blocks.
func commentFindings(root string, files []string) []string {
	if len(files) == 0 {
		return nil
	}
	python, toolkit := findPython(), toolkitRoot()
	if python == nil || toolkit == "" {
		fmt.Fprintln(os.Stderr, "pre-commit: comment lint skipped (no komodo package on this machine)")
		return nil
	}
	args := append(append([]string{}, python[1:]...), "-m", "komodo", "comments", "check", "--staged", "--json")
	cmd := exec.Command(python[0], append(args, files...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONPATH="+toolkit)
	out, err := cmd.Output()
	var parsed struct {
		Findings []finding `json:"findings"`
	}
	if json.Unmarshal(out, &parsed) != nil {
		if err != nil {
			fmt.Fprintf(os.Stderr, "pre-commit: comment lint skipped (%v)\n", err)
		}
		return nil
	}
	var problems []string
	for _, item := range parsed.Findings {
		problems = append(problems, fmt.Sprintf("%s:%d: %s %s -- %s", item.File, item.Line, item.Kind, item.Rule, item.Detail))
	}
	return problems
}

// preCommitMain refuses a protected branch or a trailer, checks gofmt, and lints comments on staged lines.
func preCommitMain() {
	root := gitOutput("", "rev-parse", "--show-toplevel")
	if root == "" {
		return
	}
	var problems []string

	if branch := gitOutput(root, "symbolic-ref", "--short", "-q", "HEAD"); branch != "" {
		if isProtected(branch, protectedPatterns(root)) {
			problems = append(problems, fmt.Sprintf("committing on protected branch %q; create a branch first", branch))
		}
	}
	if trailerRe.MatchString(commitMessage(root)) {
		problems = append(problems, "commit message carries a co-author or generated-by trailer")
	}

	files := gitLines(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	problems = append(problems, gofmtProblems(root, files)...)
	problems = append(problems, commentFindings(root, files)...)

	if len(problems) > 0 {
		blocked("pre-commit", problems)
	}
}
