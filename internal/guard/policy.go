// Package guard is the one agent hook: four denials and unlimited freedom inside a worktree.
package guard

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"komodo/internal/mount"
	"komodo/internal/profile"
	"komodo/internal/toolkit"
)

// Policy is what the guard refuses, read from the toolkit and widened by a repo.
type Policy struct {
	CriticalRefs    []string `json:"critical_refs"`
	ConfigPaths     []string `json:"config_paths"`
	TrailerPatterns []string `json:"trailer_patterns"`
	Mode            Mode     `json:"mode"`

	trailers []*regexp.Regexp
}

// Mode scopes which critical-ref rules the guard enforces: safe, default, or unsafe.
type Mode string

// The three modes a policy carries, strictest first.
const (
	ModeSafe    Mode = "safe"
	ModeDefault Mode = "default"
	ModeUnsafe  Mode = "unsafe"
)

// modeRank orders a mode from strictest to loosest, for tightening and loosening a policy.
var modeRank = map[Mode]int{ModeSafe: 0, ModeDefault: 1, ModeUnsafe: 2}

// normalizeMode reads an empty or unknown mode as default.
func normalizeMode(mode Mode) Mode {
	if _, ok := modeRank[mode]; ok {
		return mode
	}
	return ModeDefault
}

// tightenMode keeps the stricter of two modes, the only direction a repo policy may move.
func tightenMode(current, proposed Mode) Mode {
	current, proposed = normalizeMode(current), normalizeMode(proposed)
	if modeRank[proposed] < modeRank[current] {
		return proposed
	}
	return current
}

// loosenMode keeps the looser of two modes, the only direction the machine overlay may move.
func loosenMode(current, proposed Mode) Mode {
	current, proposed = normalizeMode(current), normalizeMode(proposed)
	if modeRank[proposed] > modeRank[current] {
		return proposed
	}
	return current
}

// DefaultPolicy is what the guard denies when no policy file can be read.
func DefaultPolicy() Policy {
	paths := append([]string{
		".komodo/policy.json", "komodo/policy.json",
		"~/.komodo/**", "**/.git/config", "**/.git/hooks/**", "bin/**",
	}, mount.ConfigPaths()...)
	paths = append(paths, mount.GuardConfigPaths()...)
	policy := Policy{
		CriticalRefs: []string{"main", "master"},
		ConfigPaths:  paths,
		TrailerPatterns: []string{
			`(?im)^co-authored-by\s*[:=]`, `(?im)^generated[ -]with\s*[:=]`,
			`(?im)^generated[ -]by\s*[:=]`, `\x{1F916}`,
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

// Load reads the toolkit's policy and merges the repo's and the machine's, which may only add.
func Load(toolkitRoot, repoRoot string) Policy {
	policy := DefaultPolicy()
	if shipped, ok := readShippedPolicy(toolkitRoot); ok {
		policy.CriticalRefs = union(policy.CriticalRefs, shipped.CriticalRefs)
		policy.ConfigPaths = union(policy.ConfigPaths, shipped.ConfigPaths)
		policy.TrailerPatterns = union(policy.TrailerPatterns, shipped.TrailerPatterns)
		policy.Mode = tightenMode(policy.Mode, shipped.Mode)
	}
	if extra, ok := readPolicy(filepath.Join(repoRoot, ".komodo", "policy.json")); ok {
		policy.CriticalRefs = union(policy.CriticalRefs, extra.CriticalRefs)
		policy.ConfigPaths = union(policy.ConfigPaths, extra.ConfigPaths)
		policy.TrailerPatterns = union(policy.TrailerPatterns, extra.TrailerPatterns)
		policy.Mode = tightenMode(policy.Mode, extra.Mode)
	}
	policy.CriticalRefs = union(policy.CriticalRefs, overlayCriticalRefs(profile.MachineOverlayPath()))
	policy.Mode = loosenMode(policy.Mode, overlayMode(profile.MachineOverlayPath()))
	policy.compile()
	return policy
}

// overlayCriticalRefs reads the machine overlay's critical refs, tolerating its absence.
func overlayCriticalRefs(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var overlay struct {
		CriticalRefs []string `json:"critical_refs"`
	}
	if json.Unmarshal(data, &overlay) != nil {
		return nil
	}
	return overlay.CriticalRefs
}

// overlayMode reads the machine overlay's mode, tolerating its absence.
func overlayMode(path string) Mode {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var overlay struct {
		Mode Mode `json:"mode"`
	}
	if json.Unmarshal(data, &overlay) != nil {
		return ""
	}
	return overlay.Mode
}

// readShippedPolicy parses the toolkit's own policy.json, on disk or embedded.
func readShippedPolicy(toolkitRoot string) (Policy, bool) {
	var policy Policy
	data, err := fs.ReadFile(toolkit.FS(toolkitRoot), "policy.json")
	if err != nil {
		return policy, false
	}
	if json.Unmarshal(data, &policy) != nil {
		return policy, false
	}
	return policy, len(policy.CriticalRefs) > 0 || len(policy.ConfigPaths) > 0 || policy.Mode != ""
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
	return policy, len(policy.CriticalRefs) > 0 || len(policy.ConfigPaths) > 0 || policy.Mode != ""
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

// foldsCase reports whether the platform's own disk reads a path case-insensitively.
func foldsCase() bool {
	return runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}

// IsConfigPath reports whether a path is one the hosts or the toolkit own, folding case on a
// platform whose disk does, so Bin/komodo-darwin-arm64 matches bin/**.
func (p Policy) IsConfigPath(path, repoRoot string) bool {
	normal := strings.ReplaceAll(path, "\\", "/")
	home, _ := os.UserHomeDir()
	relative := normal
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, normal); err == nil && !strings.HasPrefix(rel, "..") {
			relative = filepath.ToSlash(rel)
		}
	}
	compareNormal, compareRelative := normal, relative
	if foldsCase() {
		compareNormal, compareRelative = strings.ToLower(normal), strings.ToLower(relative)
	}
	for _, pattern := range p.ConfigPaths {
		expanded := pattern
		if strings.HasPrefix(pattern, "~/") && home != "" {
			expanded = filepath.ToSlash(filepath.Join(home, pattern[2:]))
		}
		if foldsCase() {
			expanded = strings.ToLower(expanded)
		}
		if matchPath(expanded, compareNormal) || matchPath(expanded, compareRelative) {
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
