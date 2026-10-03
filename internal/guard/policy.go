// Package guard is the one agent hook: five denials and unlimited freedom inside a worktree.
package guard

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"komodo/internal/fsx"
	"komodo/internal/mount"
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
		".komodo/policy.json",
		"~/.komodo/**", "**/.git/config", "**/.git/hooks/**", "bin/**",
	}, mount.ConfigPaths()...)
	paths = append(paths, mount.GuardConfigPaths()...)
	policy := Policy{
		CriticalRefs: []string{"main", "master"},
		ConfigPaths:  paths,
		TrailerPatterns: append([]string{
			`(?im)^co-authored-by\s*[:=]`, `(?im)^generated[ -]with\s*[:=]`,
			`(?im)^generated[ -]by\s*[:=]`, `\x{1F916}`,
		}, mount.GuardPrivatePatterns()...),
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
	policy, _ := LoadChecked(toolkitRoot, repoRoot)
	return policy
}

// LoadChecked is Load plus an error naming each present file that failed to decode and was skipped.
func LoadChecked(toolkitRoot, repoRoot string) (Policy, error) {
	policy := DefaultPolicy()
	shipped, ok, shippedErr := readShippedPolicy(toolkitRoot)
	if ok {
		policy.CriticalRefs = union(policy.CriticalRefs, shipped.CriticalRefs)
		policy.ConfigPaths = union(policy.ConfigPaths, shipped.ConfigPaths)
		policy.TrailerPatterns = union(policy.TrailerPatterns, shipped.TrailerPatterns)
		policy.Mode = tightenMode(policy.Mode, shipped.Mode)
	}
	extra, ok, repoErr := readPolicy(filepath.Join(repoRoot, ".komodo", "policy.json"))
	if ok {
		policy.CriticalRefs = union(policy.CriticalRefs, extra.CriticalRefs)
		policy.ConfigPaths = union(policy.ConfigPaths, extra.ConfigPaths)
		policy.TrailerPatterns = union(policy.TrailerPatterns, extra.TrailerPatterns)
		policy.Mode = tightenMode(policy.Mode, extra.Mode)
	}
	overlay, overlayErr := mount.DecodeOverlayFile(mount.OverlayPath())
	policy.CriticalRefs = union(policy.CriticalRefs, overlay.CriticalRefs)
	policy.Mode = loosenMode(policy.Mode, Mode(overlay.Mode))
	policy.compile()
	return policy, errors.Join(shippedErr, repoErr, overlayErr)
}

// readShippedPolicy parses the toolkit's own policy.json, on disk or embedded.
func readShippedPolicy(toolkitRoot string) (Policy, bool, error) {
	var policy Policy
	data, err := fs.ReadFile(toolkit.FS(toolkitRoot), "policy.json")
	if err != nil {
		return policy, false, nil
	}
	if _, err := fsx.DecodeStrictJSON(filepath.Join(toolkitRoot, "komodo", "policy.json"), data, &policy); err != nil {
		return Policy{}, false, err
	}
	return policy, len(policy.CriticalRefs) > 0 || len(policy.ConfigPaths) > 0 || policy.Mode != "", nil
}

// readPolicy parses one policy file, reporting whether it was usable and why a present one was not.
func readPolicy(path string) (Policy, bool, error) {
	var policy Policy
	if _, err := fsx.ReadStrictJSON(path, &policy); err != nil {
		return Policy{}, false, err
	}
	return policy, len(policy.CriticalRefs) > 0 || len(policy.ConfigPaths) > 0 || policy.Mode != "", nil
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

// epicBranchRe matches an epic branch: feat/ followed by a version number, as a prefix of the ref.
var epicBranchRe = regexp.MustCompile(`^feat/\d+\.\d+\.\d+`)

// IsCritical reports whether a ref is one the guard protects from every session, model or conductor.
func (p Policy) IsCritical(ref string) bool {
	ref = strings.TrimPrefix(strings.TrimPrefix(ref, "refs/heads/"), "origin/")
	for _, pattern := range p.CriticalRefs {
		if matchRef(pattern, ref) {
			return true
		}
	}
	return false
}

// IsEpicBranch reports whether a ref is an epic branch, which only a model session is refused;
// the conductor still pushes to and merges it, so IsCritical excludes it.
func IsEpicBranch(ref string) bool {
	ref = strings.TrimPrefix(strings.TrimPrefix(ref, "refs/heads/"), "origin/")
	return epicBranchRe.MatchString(ref)
}

// matchRef compares a ref to one pattern, honouring a single trailing star.
func matchRef(pattern, ref string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(ref, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == ref
}

// HasTrailer reports whether text carries a co-author or generated-by line, or a host's session link.
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

// RoleEnv is the environment variable a line session's role arrives in; the orchestrator sets none.
const RoleEnv = "KOMODO_ROLE"

// LineRefusedPaths are refused to every line role, but not the orchestrator.
var LineRefusedPaths = []string{"docs/prd.md", "eval/**", "komodo/policy.json"}

// BranchOnlyPaths are the guard's own policy, which the orchestrator edits only off a critical ref.
var BranchOnlyPaths = []string{"komodo/policy.json"}

// IsLineSession reports whether this process runs as a line role, not the orchestrator.
func IsLineSession() bool {
	return os.Getenv(RoleEnv) != ""
}

// IsConfigPath reports whether a path is one the hosts or the toolkit own, one of LineRefusedPaths for a line
// role, or one of BranchOnlyPaths on a critical ref; case folds as the disk does, so Bin/x matches bin/**.
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
	if matchesAnyPattern(p.ConfigPaths, home, compareNormal, compareRelative) {
		return true
	}
	if IsLineSession() && matchesAnyPattern(LineRefusedPaths, home, compareNormal, compareRelative) {
		return true
	}
	return matchesAnyPattern(BranchOnlyPaths, home, compareNormal, compareRelative) && !p.onFeatureBranch(repoRoot)
}

// onFeatureBranch reports whether repoRoot has a branch checked out that is neither a critical
// ref nor an epic branch; a plain detached HEAD, which CurrentBranch may still resolve, names none.
func (p Policy) onFeatureBranch(repoRoot string) bool {
	if repoRoot == "" {
		return false
	}
	branch := CurrentBranch(repoRoot)
	return branch != "" && branch != "HEAD" && !p.IsCritical(branch) && !IsEpicBranch(branch)
}

// matchesAnyPattern reports whether normal or relative matches any pattern, expanding a leading
// ~/ against home and folding case the way IsConfigPath compares its own two forms.
func matchesAnyPattern(patterns []string, home, normal, relative string) bool {
	for _, pattern := range patterns {
		expanded := pattern
		if strings.HasPrefix(pattern, "~/") && home != "" {
			expanded = filepath.ToSlash(filepath.Join(home, pattern[2:]))
		}
		if foldsCase() {
			expanded = strings.ToLower(expanded)
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
