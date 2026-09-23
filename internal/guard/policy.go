// Package guard is the one agent hook: four denials and unlimited freedom inside a worktree.
package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"komodo/internal/mount"
)

// Policy is what the guard refuses, read from the toolkit and widened by a repo.
type Policy struct {
	CriticalRefs    []string `json:"critical_refs"`
	ConfigPaths     []string `json:"config_paths"`
	TrailerPatterns []string `json:"trailer_patterns"`

	trailers []*regexp.Regexp
}

// DefaultPolicy is what the guard denies when no policy file can be read.
func DefaultPolicy() Policy {
	policy := Policy{
		CriticalRefs: []string{"main", "master"},
		ConfigPaths: append([]string{
			"~/.komodo/**", "**/.git/config", "**/.git/hooks/**", "bin/**",
		}, mount.ConfigPaths()...),
		TrailerPatterns: []string{
			`(?i)co-authored[-]by\s*:`, `(?i)generated[ ]with`, `(?i)generated[ ]by`, `\x{1F916}`,
		},
	}
	policy.compile()
	return policy
}

// compile builds the trailer matchers, dropping any pattern that will not compile.
func (p *Policy) compile() {
	p.trailers = nil
	for _, pattern := range p.TrailerPatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			p.trailers = append(p.trailers, compiled)
		}
	}
}

// Load reads the toolkit's policy and merges the repo's, which may only add.
func Load(toolkitRoot, repoRoot string) Policy {
	policy := DefaultPolicy()
	if shipped, ok := readPolicy(filepath.Join(toolkitRoot, "komodo", "policy.json")); ok {
		policy = shipped
	}
	if extra, ok := readPolicy(filepath.Join(repoRoot, ".komodo", "policy.json")); ok {
		policy.CriticalRefs = union(policy.CriticalRefs, extra.CriticalRefs)
		policy.ConfigPaths = union(policy.ConfigPaths, extra.ConfigPaths)
		policy.TrailerPatterns = union(policy.TrailerPatterns, extra.TrailerPatterns)
	}
	policy.compile()
	return policy
}

// readPolicy parses one policy file, reporting whether it was usable.
func readPolicy(path string) (Policy, bool) {
	var policy Policy
	data, err := os.ReadFile(path)
	if err != nil {
		return policy, false
	}
	if json.Unmarshal(data, &policy) != nil {
		return policy, false
	}
	return policy, len(policy.CriticalRefs) > 0 || len(policy.ConfigPaths) > 0
}

// union adds what the repo names without dropping anything the toolkit names.
func union(base, extra []string) []string {
	seen := map[string]bool{}
	for _, item := range base {
		seen[item] = true
	}
	for _, item := range extra {
		if !seen[item] {
			seen[item] = true
			base = append(base, item)
		}
	}
	return base
}

// IsCritical reports whether a ref is one the guard protects.
func (p Policy) IsCritical(ref string) bool {
	ref = strings.TrimPrefix(strings.TrimPrefix(ref, "refs/heads/"), "origin/")
	for _, pattern := range p.CriticalRefs {
		if matchRef(pattern, ref) {
			return true
		}
	}
	return false
}

// matchRef compares a ref to one pattern, honouring a single trailing star.
func matchRef(pattern, ref string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(ref, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == ref
}

// HasTrailer reports whether a commit message carries a co-author or generated-by line.
func (p Policy) HasTrailer(text string) bool {
	for _, pattern := range p.trailers {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// IsConfigPath reports whether a path is one the hosts or the toolkit own.
func (p Policy) IsConfigPath(path, repoRoot string) bool {
	normal := strings.ReplaceAll(path, "\\", "/")
	home, _ := os.UserHomeDir()
	relative := normal
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, normal); err == nil && !strings.HasPrefix(rel, "..") {
			relative = filepath.ToSlash(rel)
		}
	}
	for _, pattern := range p.ConfigPaths {
		expanded := pattern
		if strings.HasPrefix(pattern, "~/") && home != "" {
			expanded = filepath.ToSlash(filepath.Join(home, pattern[2:]))
		}
		if matchPath(expanded, normal) || matchPath(expanded, relative) {
			return true
		}
	}
	return false
}

// matchPath compares a path to a glob of the shapes the policy uses.
func matchPath(pattern, path string) bool {
	switch {
	case strings.HasSuffix(pattern, "/**"):
		prefix := strings.TrimSuffix(pattern, "/**")
		if strings.HasPrefix(prefix, "**/") {
			needle := "/" + strings.TrimPrefix(prefix, "**/") + "/"
			return strings.Contains(path+"/", needle) || strings.HasPrefix(path+"/", strings.TrimPrefix(prefix, "**/")+"/")
		}
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	case strings.HasPrefix(pattern, "**/"):
		name := strings.TrimPrefix(pattern, "**/")
		return path == name || strings.HasSuffix(path, "/"+name)
	default:
		return path == pattern
	}
}

// homeDir is the user's home directory, which the config paths are relative to.
func homeDir() (string, error) { return os.UserHomeDir() }
