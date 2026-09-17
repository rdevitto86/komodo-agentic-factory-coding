package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

var builtinProtected = []string{"main", "master", "trunk", "prod", "production", "release/*", "hotfix/*"}

// repoRoot walks up from dir to the nearest directory holding .git, or returns empty.
func repoRoot(dir string) string {
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

// protectedPatterns reads the repo's komodo.json protected list, falling back to the built-in set.
func protectedPatterns(cwd string) []string {
	root := repoRoot(cwd)
	if root == "" {
		return builtinProtected
	}
	data, err := os.ReadFile(filepath.Join(root, "komodo.json"))
	if err != nil {
		return builtinProtected
	}
	var config struct {
		Protected []string `json:"protected"`
	}
	if json.Unmarshal(data, &config) != nil || len(config.Protected) == 0 {
		return builtinProtected
	}
	return config.Protected
}

// currentBranch reads .git/HEAD for the checked-out branch, or returns empty when detached.
func currentBranch(cwd string) string {
	root := repoRoot(cwd)
	if root == "" {
		return ""
	}
	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	// A worktree or submodule stores a gitdir pointer file where the directory would be.
	if !info.IsDir() {
		data, err := os.ReadFile(gitPath)
		if err != nil {
			return ""
		}
		line := strings.TrimSpace(string(data))
		if !strings.HasPrefix(line, "gitdir: ") {
			return ""
		}
		gitPath = strings.TrimSpace(line[len("gitdir: "):])
		if !filepath.IsAbs(gitPath) {
			gitPath = filepath.Join(root, gitPath)
		}
	}
	data, err := os.ReadFile(filepath.Join(gitPath, "HEAD"))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if strings.HasPrefix(line, "ref: refs/heads/") {
		return line[len("ref: refs/heads/"):]
	}
	return ""
}

// isProtected reports whether a ref name matches any protected pattern after stripping prefixes.
func isProtected(name string, patterns []string) bool {
	name = strings.Trim(strings.TrimSpace(name), "'\"")
	for _, prefix := range []string{"refs/heads/", "origin/"} {
		name = strings.TrimPrefix(name, prefix)
	}
	for _, pattern := range patterns {
		if globMatch(name, pattern) {
			return true
		}
	}
	return false
}
