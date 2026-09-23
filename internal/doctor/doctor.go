// Package doctor audits the repo: references, roles, leaks, drift, budgets, and leftovers.
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/mount"
	"komodo/internal/release"
)

// Budgets, in tokens for context and bytes for a standard.
const (
	AlwaysOnTokens = 1500
	RunSkillTokens = 800
	StandardBytes  = 8192
)

// Problem is one thing the doctor found, named by the check that found it.
type Problem struct {
	Check  string `json:"check"`
	Where  string `json:"where"`
	Detail string `json:"detail"`
}

// Options are the switches the command passes in.
type Options struct {
	NoGit bool
	Prune bool
}

// Run walks every check and returns what it found.
func Run(root string, options Options) ([]Problem, error) {
	var problems []Problem
	problems = append(problems, checkReferences(root)...)
	problems = append(problems, checkRoles(root)...)
	problems = append(problems, checkLeaks(root)...)
	problems = append(problems, checkBudgets(root)...)
	problems = append(problems, checkDrift(root)...)
	problems = append(problems, checkProfileDrift(root)...)
	problems = append(problems, checkPromises(root)...)
	if !options.NoGit {
		found, err := checkGit(root)
		if err != nil {
			return problems, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
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
		schema := filepath.Join(root, "komodo", "roles", role.Returns)
		data, err := os.ReadFile(schema)
		if err != nil {
			problems = append(problems, Problem{"roles", where, role.Returns + " does not exist"})
			continue
		}
		var parsed map[string]any
		if json.Unmarshal(data, &parsed) != nil {
			problems = append(problems, Problem{"roles", rel(root, schema), "is not JSON"})
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
	for _, path := range sources(root) {
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

// checkBudgets reports the always-on context, the run skill, and any standard over its cap.
func checkBudgets(root string) []Problem {
	var problems []Problem
	total := 0
	if data, err := os.ReadFile(filepath.Join(root, "AGENTS.md")); err == nil {
		total += tokens(len(data))
	}
	if rules, err := mount.Rules(root); err == nil {
		total += tokens(len(rules))
	}
	if total > AlwaysOnTokens {
		problems = append(problems, Problem{"budgets", "always-on context",
			fmt.Sprintf("about %d tokens; the cap is %d", total, AlwaysOnTokens)})
	}
	if data, err := os.ReadFile(filepath.Join(root, "komodo", "skills", "run", "SKILL.md")); err == nil {
		if count := tokens(len(data)); count > RunSkillTokens {
			problems = append(problems, Problem{"budgets", "komodo/skills/run/SKILL.md",
				fmt.Sprintf("about %d tokens; the cap is %d", count, RunSkillTokens)})
		}
	}
	skills, err := mount.LoadSkills(root)
	if err != nil {
		return problems
	}
	for _, skill := range skills {
		if !strings.HasPrefix(skill.Name, "standards-") {
			continue
		}
		if len(skill.Body) > StandardBytes {
			problems = append(problems, Problem{"budgets",
				filepath.Join("komodo", "skills", skill.Name, "SKILL.md"),
				fmt.Sprintf("%d bytes; the cap is %d", len(skill.Body), StandardBytes)})
		}
	}
	return problems
}

// checkDrift reports a rendered host file that differs from what the source renders now.
func checkDrift(root string) []Problem {
	var problems []Problem
	for _, host := range mount.Hosts() {
		if host.Render == nil {
			continue
		}
		if host.Installed != nil && !host.Installed(root) {
			continue
		}
		plan, err := host.Render(root, mount.BinaryPath())
		if err != nil {
			problems = append(problems, Problem{"drift", host.Name, err.Error()})
			continue
		}
		for _, action := range plan.Actions() {
			if action.Seed {
				continue
			}
			switch action.Verb {
			case "update", "remove":
				problems = append(problems, Problem{"drift", action.Path,
					"differs from what the source renders now; run komodo install"})
			case "create":
				problems = append(problems, Problem{"drift", action.Path,
					"the source renders this file now but the mount has never written it; run komodo install"})
			}
		}
	}
	return problems
}

// checkProfileDrift reports when the cached repo profile does not match a fresh detection.
func checkProfileDrift(root string) []Problem {
	cached, ok := detect.LoadCached(root)
	if !ok {
		return nil
	}
	fresh, _ := detect.Detect(root)
	if reflect.DeepEqual(cached, fresh) {
		return nil
	}
	return []Problem{{"profile", ".komodo/profile.json", "differs from a fresh detection; run komodo detect"}}
}

// promiseBullet matches a grammar rule bullet naming the snake_case key its accessor is built from.
var promiseBullet = regexp.MustCompile("(?m)^- \\*\\*`([a-z][a-z0-9_]*)`\\*\\*")

// checkPromises reports a grammar key whose accessor exists but is never called outside its own tests.
func checkPromises(root string) []Problem {
	var problems []Problem
	files := sources(root)
	for _, path := range markdown(root) {
		relative := rel(root, path)
		if !strings.HasPrefix(relative, filepath.Join("komodo", "rules")) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, match := range promiseBullet.FindAllStringSubmatch(string(data), -1) {
			key := match[1]
			symbol := pascal(key)
			if !declared(files, symbol) || called(files, symbol) {
				continue
			}
			problems = append(problems, Problem{"promises", relative,
				key + " promises " + symbol + ", but nothing outside its own tests calls it"})
		}
	}
	return problems
}

// pascal turns a snake_case grammar key into the accessor name it promises.
func pascal(key string) string {
	var out strings.Builder
	for _, part := range strings.Split(key, "_") {
		if part == "" {
			continue
		}
		out.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return out.String()
}

// declared reports whether any source file declares a function or method named symbol.
func declared(files []string, symbol string) bool {
	decl := regexp.MustCompile(`func\s+(?:\([^)]*\)\s+)?` + regexp.QuoteMeta(symbol) + `\(`)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err == nil && decl.MatchString(string(data)) {
			return true
		}
	}
	return false
}

// called reports whether a non-test file uses symbol beyond the line that declares it.
func called(files []string, symbol string) bool {
	decl := regexp.MustCompile(`func\s+(?:\([^)]*\)\s+)?` + regexp.QuoteMeta(symbol) + `\(`)
	usage := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		if len(usage.FindAllString(text, -1)) > len(decl.FindAllString(text, -1)) {
			return true
		}
	}
	return false
}

// checkGit reports conflict markers and the leftovers a run can strand.
func checkGit(root string) ([]Problem, error) {
	var problems []Problem
	out, err := git(root, "ls-files", "-u")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) != "" {
		problems = append(problems, Problem{"git", "index", "the index holds unmerged paths"})
	}
	for _, path := range sources(root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "\n<<<<<<< ") {
			problems = append(problems, Problem{"git", rel(root, path), "a conflict marker is still in the file"})
		}
	}
	changelog, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return problems, err
	}
	tags := lines(git(root, "tag", "--list"))
	for _, drift := range release.Check(changelog, tags, nil) {
		problems = append(problems, Problem{"changelog", drift.Subject, drift.Detail})
	}
	return problems, nil
}

// Prune removes stale worktrees under the state directory and deletes merged branches.
func Prune(root, base string) ([]string, error) {
	var done []string
	out, err := git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		path, found := strings.CutPrefix(strings.TrimSpace(line), "worktree ")
		if !found || !strings.Contains(path, filepath.Join(".komodo", "wt")) {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if _, err := git(root, "worktree", "remove", "--force", path); err == nil {
			done = append(done, "removed worktree "+rel(root, path))
		}
	}
	if _, err := git(root, "worktree", "prune"); err == nil {
		done = append(done, "pruned the worktree list")
	}
	merged := lines(git(root, "branch", "--merged", base, "--format=%(refname:short)"))
	for _, branch := range merged {
		if branch == base || branch == "" || strings.HasPrefix(branch, "*") {
			continue
		}
		if _, err := git(root, "branch", "-d", branch); err == nil {
			done = append(done, "deleted merged branch "+branch)
		}
	}
	return done, nil
}

// tokens is the rough token count of a byte length.
func tokens(size int) int { return (size + 3) / 4 }

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

// rel is a path relative to the root, or the path itself.
func rel(root, path string) string {
	if relative, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(relative, "..") {
		return relative
	}
	return path
}

// git runs one read-only git command in the repo root.
func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
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
