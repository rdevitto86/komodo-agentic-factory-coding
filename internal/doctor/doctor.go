// Package doctor audits the repo: references, roles, leaks, drift, budgets, and leftovers.
package doctor

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"komodo/internal/git"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/release"
	"komodo/internal/toolkit"
)

// Budgets, in tokens for context; a standard's own cap is internal/line.CapStandard.
const (
	AlwaysOnTokens = 1500
	RunSkillTokens = 800
)

// Problem is one thing the doctor found, named by the check that found it.
type Problem struct {
	Check  string `json:"check"`
	Where  string `json:"where"`
	Detail string `json:"detail"`
}

// Options are the switches the command passes in.
type Options struct {
	NoGit  bool
	Prune  bool
	Remote bool
}

// Run walks every check and returns what it found.
func Run(root string, options Options) ([]Problem, error) {
	var problems []Problem
	rendered := renderInstalled(root, pinLocalDown)
	problems = append(problems, checkReferences(root)...)
	problems = append(problems, checkRoles(root)...)
	problems = append(problems, checkLeaks(root)...)
	problems = append(problems, checkBudgets(root, rendered)...)
	problems = append(problems, checkDrift(rendered, renderInstalled(root, pinLocalUp))...)
	problems = append(problems, checkHookBinary(root, rendered)...)
	problems = append(problems, checkProfileDrift(root)...)
	problems = append(problems, checkPromises(root)...)
	if !options.NoGit {
		found, err := checkGit(root)
		if err != nil {
			return problems, err
		}
		problems = append(problems, found...)
	}
	if options.Remote {
		problems = append(problems, CheckRulesets(root, base(root), pr.Run)...)
	}
	return problems, nil
}

// HostLeftovers lists what an installed host's user settings carry from a retired setup; it never fails a check.
func HostLeftovers(root string) []string {
	var notes []string
	for _, host := range mount.Hosts() {
		if host.Leftovers != nil && host.Installed != nil && host.Installed(root) {
			notes = append(notes, host.Leftovers()...)
		}
	}
	return notes
}

// StrayWorktrees names each linked worktree parked outside .komodo/wt by its path and branch; it never fails a check.
func StrayWorktrees(root string) []string {
	worktrees, err := git.Worktrees(root)
	if err != nil {
		return nil
	}
	var notes []string
	for index, current := range worktrees {
		if index == 0 || strings.Contains(current.Path, filepath.Join(".komodo", "wt")) {
			continue
		}
		notes = append(notes, current.Path+" on branch "+current.Branch)
	}
	return notes
}

// base is the remote's default branch, or main.
func base(root string) string {
	out, err := git.Run(root, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err != nil || out == "" {
		return "main"
	}
	return strings.TrimPrefix(strings.TrimSpace(out), "refs/remotes/origin/")
}

// ruleset is the part of a forge branch ruleset the audit reads.
type ruleset struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Target      string `json:"target"`
	Enforcement string `json:"enforcement"`
	Conditions  struct {
		RefName struct {
			Include []string `json:"include"`
		} `json:"ref_name"`
	} `json:"conditions"`
}

// CheckRulesets reads the forge's branch rulesets through gh and reports a default branch nothing
// protects, or any active ruleset that reaches past it, which would block the push of every group branch.
func CheckRulesets(root, defaultBranch string, run pr.Runner) []Problem {
	out, err := run(root, "api", "repos/{owner}/{repo}/rulesets")
	if err != nil {
		return []Problem{{"ruleset", "gh", "could not list rulesets: " + err.Error()}}
	}
	var listed []ruleset
	if json.Unmarshal([]byte(out), &listed) != nil {
		return []Problem{{"ruleset", "gh", "rulesets did not parse"}}
	}
	var problems []Problem
	covered := false
	for _, item := range listed {
		if item.Target != "branch" || item.Enforcement != "active" {
			continue
		}
		detail, err := run(root, "api", fmt.Sprintf("repos/{owner}/{repo}/rulesets/%d", item.ID))
		if err != nil || json.Unmarshal([]byte(detail), &item) != nil {
			problems = append(problems, Problem{"ruleset", item.Name, "could not be read"})
			continue
		}
		for _, ref := range item.Conditions.RefName.Include {
			covered = covered || ref == "~ALL"
			if ref == "~DEFAULT_BRANCH" || ref == "refs/heads/"+defaultBranch {
				covered = true
				continue
			}
			problems = append(problems, Problem{"ruleset", item.Name,
				fmt.Sprintf("includes %s; scope it to refs/heads/%s so a group branch can be pushed", ref, defaultBranch)})
		}
	}
	if !covered && !branchProtected(root, defaultBranch, run) {
		problems = append(problems, Problem{"ruleset", defaultBranch,
			fmt.Sprintf("no active ruleset or branch protection covers refs/heads/%s; the forge is the boundary the guard cannot be", defaultBranch)})
	}
	return problems
}

// branchProtected reports whether the forge's classic branch protection covers the branch.
func branchProtected(root, branch string, run pr.Runner) bool {
	_, err := run(root, "api", fmt.Sprintf("repos/{owner}/{repo}/branches/%s/protection", branch))
	return err == nil
}

var reference = regexp.MustCompile("`([A-Za-z0-9_./-]+\\.(?:md|json|go|yaml|yml|toml|sh|sha256))`")

// checkReferences reports every backticked path in the markdown that resolves to nothing.
func checkReferences(root string) []Problem {
	var problems []Problem
	for _, path := range markdown(root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		relative := rel(root, path)
		seen := map[string]bool{}
		for _, match := range reference.FindAllStringSubmatch(string(data), -1) {
			target := match[1]
			if seen[target] || strings.HasPrefix(target, "http") {
				continue
			}
			seen[target] = true
			if resolves(root, filepath.Dir(path), target) {
				continue
			}
			problems = append(problems, Problem{"references", relative, target + " resolves to nothing"})
		}
	}
	return problems
}

// resolves reports whether a referenced path exists beside the file, at the root, or under komodo.
func resolves(root, dir, target string) bool {
	if strings.ContainsAny(target, "<>*") {
		return true
	}
	for _, base := range []string{dir, root, filepath.Join(root, "komodo"), filepath.Join(root, "internal")} {
		if _, err := os.Stat(filepath.Join(base, target)); err == nil {
			return true
		}
	}
	return false
}

// checkRoles reports every role whose frontmatter or schema is not well formed.
func checkRoles(root string) []Problem {
	var problems []Problem
	roles, err := mount.LoadRoles(root)
	if err != nil {
		return []Problem{{"roles", "komodo/roles", err.Error()}}
	}
	loaded := map[string]bool{}
	for _, role := range roles {
		loaded[role.Name] = true
	}
	matches, _ := filepath.Glob(filepath.Join(root, "komodo", "roles", "*.md"))
	for _, path := range matches {
		if stem := strings.TrimSuffix(filepath.Base(path), ".md"); !loaded[stem] {
			problems = append(problems, Problem{"roles", rel(root, path), "did not load: missing or malformed frontmatter"})
		}
	}
	for _, role := range roles {
		where := filepath.Join("komodo", "roles", role.Name+".md")
		if role.Name == "" || role.Description == "" || role.Tier == "" {
			problems = append(problems, Problem{"roles", where, "name, description, and tier are required"})
		}
		if role.Tier != "light" && role.Tier != "standard" && role.Tier != "heavy" {
			problems = append(problems, Problem{"roles", where, "tier " + role.Tier + " is not light, standard, or heavy"})
		}
		for _, tool := range role.Tools {
			if !contains(mount.Verbs, tool) {
				problems = append(problems, Problem{"roles", where, tool + " is not one of the five Komodo verbs"})
			}
		}
		if role.Returns == "" {
			problems = append(problems, Problem{"roles", where, "the role names no schema"})
			continue
		}
		schema := filepath.Join("komodo", "roles", role.Returns)
		data, err := fs.ReadFile(toolkit.FS(root), path.Join("roles", role.Returns))
		if err != nil {
			problems = append(problems, Problem{"roles", where, role.Returns + " does not exist"})
			continue
		}
		var parsed map[string]any
		if json.Unmarshal(data, &parsed) != nil {
			problems = append(problems, Problem{"roles", schema, "is not JSON"})
		}
	}
	return problems
}

// checkLeaks reports a vendor name, host path, or host flag outside internal/mount.
func checkLeaks(root string) []Problem {
	vendors := mount.Vendors()
	if len(vendors) == 0 {
		return nil
	}
	var problems []Problem
	for _, path := range textFiles(root) {
		relative := rel(root, path)
		if strings.HasPrefix(relative, filepath.Join("internal", "mount")) || strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for index, line := range strings.Split(string(data), "\n") {
			if isMountImport(line) {
				continue
			}
			lowered := strings.ToLower(line)
			for _, vendor := range vendors {
				if !strings.Contains(lowered, strings.ToLower(vendor)) {
					continue
				}
				problems = append(problems, Problem{"leaks",
					fmt.Sprintf("%s:%d", relative, index+1),
					vendor + " is a host's name and belongs inside internal/mount"})
				break
			}
		}
	}
	return problems
}

// isMountImport reports whether a line is the blank import that registers a mount.
func isMountImport(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "_ \"komodo/internal/mount/") || strings.Contains(trimmed, "\"komodo/internal/mount")
}

// checkGit reports conflict markers and the leftovers a run can strand.
func checkGit(root string) ([]Problem, error) {
	var problems []Problem
	out, err := git.Run(root, "ls-files", "-u")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) != "" {
		problems = append(problems, Problem{"git", "index", "the index holds unmerged paths"})
	}
	for _, path := range textFiles(root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		if strings.HasPrefix(text, "<<<<<<< ") || strings.Contains(text, "\n<<<<<<< ") {
			problems = append(problems, Problem{"git", rel(root, path), "a conflict marker is still in the file"})
		}
	}
	changelog, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return problems, err
	}
	tags := lines(git.Run(root, "tag", "--list"))
	for _, drift := range release.Check(changelog, tags, nil) {
		problems = append(problems, Problem{"changelog", drift.Subject, drift.Detail})
	}
	return problems, nil
}

// markdown lists the files whose references must resolve: the rules and the roles, never a plan.
func markdown(root string) []string {
	var out []string
	if path := filepath.Join(root, "AGENTS.md"); exists(path) {
		out = append(out, path)
	}
	_ = filepath.WalkDir(filepath.Join(root, "komodo"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if strings.Contains(path, string(os.PathSeparator)+"standards-") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out
}

// exists reports whether a path is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// sources lists the Go files the doctor reads.
func sources(root string) []string {
	var out []string
	for _, dir := range []string{"cmd", "internal"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".go") {
				out = append(out, path)
			}
			return nil
		})
	}
	return out
}

// textFiles lists every Go source, every file under komodo and templates, and the root AGENTS.md.
func textFiles(root string) []string {
	out := sources(root)
	for _, dir := range []string{"komodo", "templates"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				out = append(out, path)
			}
			return nil
		})
	}
	if path := filepath.Join(root, "AGENTS.md"); exists(path) {
		out = append(out, path)
	}
	return out
}

// rel is a path relative to the root, or the path itself.
func rel(root, path string) string {
	if relative, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(relative, "..") {
		return relative
	}
	return path
}

// lines splits a command's output, dropping the error and the blanks.
func lines(out string, _ error) []string {
	var result []string
	for _, line := range strings.Split(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// contains reports whether the slice holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
