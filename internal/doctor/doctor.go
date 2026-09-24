// Package doctor audits the repo: references, roles, leaks, drift, budgets, and leftovers.
package doctor

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/detect"
	"komodo/internal/guard"
	"komodo/internal/install"
	"komodo/internal/line"
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

// base is the remote's default branch, or main.
func base(root string) string {
	out, err := git(root, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
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

// CheckRulesets reads the forge's branch rulesets through gh and reports any active one that
// reaches past the default branch, which would block the push of every group branch.
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
			if ref == "~DEFAULT_BRANCH" || ref == "refs/heads/"+defaultBranch {
				continue
			}
			problems = append(problems, Problem{"ruleset", item.Name,
				fmt.Sprintf("includes %s; scope it to refs/heads/%s so a group branch can be pushed", ref, defaultBranch)})
		}
	}
	return problems
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

// checkBudgets reports the always-on context, the run skill, and any standard over its cap.
func checkBudgets(root string, rendered []renderedHost) []Problem {
	var problems []Problem
	total := 0
	if data, err := os.ReadFile(filepath.Join(root, "AGENTS.md")); err == nil {
		total += tokens(len(data))
	}
	if rules, err := mount.Rules(root); err == nil {
		total += tokens(len(rules))
	}
	total += renderedSkillTokens(rendered)
	skills, skillsErr := mount.LoadSkills(root)
	if roles, err := mount.LoadRoles(root); err == nil {
		for _, role := range roles {
			if role.Session {
				total += tokens(len(role.Description))
			}
		}
	}
	if total > AlwaysOnTokens {
		problems = append(problems, Problem{"budgets", "always-on context",
			fmt.Sprintf("about %d tokens; the cap is %d", total, AlwaysOnTokens)})
	}
	if data, err := fs.ReadFile(toolkit.FS(root), path.Join("skills", "run", "SKILL.md")); err == nil {
		if count := tokens(len(data)); count > RunSkillTokens {
			problems = append(problems, Problem{"budgets", "komodo/skills/run/SKILL.md",
				fmt.Sprintf("about %d tokens; the cap is %d", count, RunSkillTokens)})
		}
	}
	if skillsErr != nil {
		return problems
	}
	for _, skill := range skills {
		if !strings.HasPrefix(skill.Name, "standards-") {
			continue
		}
		if size := len(frontmatterBody(skill.Body)); size > line.CapStandard {
			problems = append(problems, Problem{"budgets",
				filepath.Join("komodo", "skills", skill.Name, "SKILL.md"),
				fmt.Sprintf("%d bytes; the cap is %d", size, line.CapStandard)})
		}
	}
	return problems
}

// frontmatterBlock splits a file into its frontmatter and its body.
var frontmatterBlock = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n(.*)\z`)

// frontmatterBody is a file's content after its frontmatter, or the whole text when there is none.
func frontmatterBody(text string) string {
	if match := frontmatterBlock.FindStringSubmatch(text); match != nil {
		return match[2]
	}
	return text
}

// frontmatterField reads one key's value from a file's frontmatter, or "" when it is absent.
func frontmatterField(text, key string) string {
	match := frontmatterBlock.FindStringSubmatch(text)
	if match == nil {
		return ""
	}
	for _, line := range strings.Split(match[1], "\n") {
		name, value, found := strings.Cut(line, ":")
		if found && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// renderedHost is one installed host's render, or the error rendering it produced.
type renderedHost struct {
	Name string
	Plan install.Plan
	Err  error
}

// renderInstalled renders every installed host once with the local machine pinned, so every check reads one plan.
func renderInstalled(root string, pin func() func()) []renderedHost {
	defer freezeProfile(root)()
	defer pin()()
	var out []renderedHost
	for _, host := range mount.Hosts() {
		if host.Render == nil {
			continue
		}
		if host.Installed != nil && !host.Installed(root) {
			continue
		}
		plan, err := host.Render(root, mount.BinaryPath())
		out = append(out, renderedHost{Name: host.Name, Plan: plan, Err: err})
	}
	return out
}

// renderedSkillTokens sums the descriptions of the skills a host renders, the part it preloads every session.
func renderedSkillTokens(rendered []renderedHost) int {
	total := 0
	for _, host := range rendered {
		if host.Err != nil {
			continue
		}
		for _, change := range host.Plan.Changes {
			if change.Project && filepath.Base(change.Path) == "SKILL.md" {
				total += tokens(len(frontmatterField(string(change.Body), "description")))
			}
		}
	}
	return total
}

// checkDrift reports a host file that matches neither the render with the local machine down nor up.
func checkDrift(rendered, renderedUp []renderedHost) []Problem {
	matchesUp := map[string]bool{}
	for _, host := range renderedUp {
		if host.Err != nil {
			continue
		}
		for _, action := range host.Plan.Actions() {
			if action.Verb != "update" && action.Verb != "remove" && action.Verb != "create" {
				matchesUp[action.Path] = true
			}
		}
	}
	var problems []Problem
	for _, host := range rendered {
		if host.Err != nil {
			problems = append(problems, Problem{"drift", host.Name, host.Err.Error()})
			continue
		}
		for _, action := range host.Plan.Actions() {
			if action.Seed || matchesUp[action.Path] {
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

// freezeProfile snapshots the profile cache and returns a func that restores it exactly.
func freezeProfile(root string) func() {
	path := filepath.Join(root, ".komodo", "profile.json")
	data, err := os.ReadFile(path)
	existed := err == nil
	return func() {
		if existed {
			_ = os.WriteFile(path, data, 0o644)
			return
		}
		_ = os.Remove(path)
	}
}

// pinLocalDown points the local machine probe at a closed port for the caller's duration.
func pinLocalDown() func() {
	previous, existed := os.LookupEnv(mount.LocalMachine().Env)
	_ = os.Setenv(mount.LocalMachine().Env, "127.0.0.1:1")
	return func() {
		if existed {
			_ = os.Setenv(mount.LocalMachine().Env, previous)
			return
		}
		_ = os.Unsetenv(mount.LocalMachine().Env)
	}
}

// pinLocalUp points the local machine probe at a loopback listener this audit owns, never the live one.
func pinLocalUp() func() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return pinLocalDown()
	}
	previous, existed := os.LookupEnv(mount.LocalMachine().Env)
	_ = os.Setenv(mount.LocalMachine().Env, "http://"+listener.Addr().String())
	return func() {
		_ = listener.Close()
		if existed {
			_ = os.Setenv(mount.LocalMachine().Env, previous)
			return
		}
		_ = os.Unsetenv(mount.LocalMachine().Env)
	}
}

// promiseBullet matches a grammar rule bullet naming the snake_case key its accessor is built from.
var promiseBullet = regexp.MustCompile("(?m)^- \\*\\*`([a-z][a-z0-9_]*)`\\*\\*")

// jsonTagKey extracts the snake_case key a struct field's JSON tag names.
var jsonTagKey = regexp.MustCompile(`json:"([a-z][a-z0-9_]*)`)

// promise is one grammar key or config field, and where it is promised.
type promise struct {
	key    string
	symbol string
	where  string
}

// checkPromises reports a grammar key or config field whose accessor exists but nothing outside its own tests calls.
func checkPromises(root string) []Problem {
	var problems []Problem
	files := sources(root)
	index := indexSymbols(files)
	for _, made := range promises(root, files) {
		if !index.declared[made.symbol] || index.called[made.symbol] {
			continue
		}
		problems = append(problems, Problem{"promises", made.where,
			made.key + " promises " + made.symbol + ", but nothing outside its own tests calls it"})
	}
	return problems
}

// promises lists every grammar bullet komodo/rules makes and every JSON-tagged struct field the source declares.
func promises(root string, files []string) []promise {
	var out []promise
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
			out = append(out, promise{key, pascal(key), relative})
		}
	}
	return append(out, fieldPromises(root, files)...)
}

// grammarPackages are the packages a repo or a machine's own JSON config populates by field name.
var grammarPackages = []string{filepath.Join("internal", "profile"), filepath.Join("internal", "repo")}

// fieldPromises lists every struct field with a JSON tag in a grammar package, the key it promises to read.
func fieldPromises(root string, files []string) []promise {
	var out []promise
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || !contains(grammarPackages, filepath.Dir(rel(root, path))) {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			field, ok := node.(*ast.Field)
			if !ok || field.Tag == nil {
				return true
			}
			match := jsonTagKey.FindStringSubmatch(field.Tag.Value)
			if match == nil {
				return true
			}
			for _, name := range field.Names {
				position := fset.Position(name.Pos())
				out = append(out, promise{match[1], name.Name, fmt.Sprintf("%s:%d", rel(root, path), position.Line)})
			}
			return true
		})
	}
	return out
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

// symbolIndex records where a symbol is declared and where a real call site uses it.
type symbolIndex struct {
	declared map[string]bool
	called   map[string]bool
}

// indexSymbols parses every source file once, resolving each symbol's declaration and its real call sites.
func indexSymbols(files []string) symbolIndex {
	index := symbolIndex{declared: map[string]bool{}, called: map[string]bool{}}
	for _, path := range files {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		test := strings.HasSuffix(path, "_test.go")
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				index.declared[n.Name.Name] = true
			case *ast.Field:
				if n.Tag != nil {
					for _, name := range n.Names {
						index.declared[name.Name] = true
					}
				}
			case *ast.CallExpr:
				if !test {
					if ident, ok := n.Fun.(*ast.Ident); ok {
						index.called[ident.Name] = true
					}
				}
			case *ast.SelectorExpr:
				if !test {
					index.called[n.Sel.Name] = true
				}
			}
			return true
		})
	}
	return index
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
	tags := lines(git(root, "tag", "--list"))
	for _, drift := range release.Check(changelog, tags, nil) {
		problems = append(problems, Problem{"changelog", drift.Subject, drift.Detail})
	}
	return problems, nil
}

// Prune removes stale worktrees, deletes merged branches, and settles a shipped run origin has merged.
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
	done = append(done, settleShippedRun(root, base)...)
	policy := guard.Load(root, root)
	merged := lines(git(root, "branch", "--merged", base, "--format=%(refname:short)"))
	for _, branch := range merged {
		if branch == base || branch == "" || strings.HasPrefix(branch, "*") || policy.IsCritical(branch) {
			continue
		}
		if _, err := git(root, "branch", "-d", branch); err == nil {
			done = append(done, "deleted merged branch "+branch)
		}
	}
	return done, nil
}

// settleShippedRun sweeps every clean worktree under .komodo/wt whose branch origin's base now holds,
// whatever the run state holds; only the current run's own branch triggers its status flip restore.
func settleShippedRun(root, base string) []string {
	if _, err := git(root, "fetch", "--quiet", "origin", base); err != nil {
		return nil
	}
	remote := "origin/" + base
	var done []string
	if state, err := line.LoadRun(root); err == nil && state.Branch != "" && !line.RunIsOpen(root) {
		if _, err := git(root, "merge-base", "--is-ancestor", state.Branch, remote); err == nil {
			if restoreFlips(root, remote) {
				done = append(done, "restored BACKLOG.md; "+remote+" holds its status flips")
			}
		}
	}
	for _, worktree := range stateWorktrees(root) {
		if status, err := git(worktree.path, "status", "--porcelain"); err != nil || status != "" {
			continue
		}
		if _, err := git(root, "merge-base", "--is-ancestor", worktree.branch, remote); err != nil {
			continue
		}
		if _, err := git(root, "worktree", "remove", "--force", worktree.path); err != nil {
			continue
		}
		done = append(done, "removed worktree "+rel(root, worktree.path))
		if _, err := git(root, "branch", "-D", worktree.branch); err == nil {
			done = append(done, "deleted merged branch "+worktree.branch)
		}
	}
	return done
}

// stateWorktree is one worktree under the state directory and the branch it has checked out.
type stateWorktree struct{ path, branch string }

// stateWorktrees lists the worktrees under .komodo/wt that exist on disk and hold a branch.
func stateWorktrees(root string) []stateWorktree {
	out, err := git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	var found []stateWorktree
	for _, block := range strings.Split(out, "\n\n") {
		var current stateWorktree
		for _, field := range strings.Split(block, "\n") {
			if path, ok := strings.CutPrefix(field, "worktree "); ok {
				current.path = path
			}
			if ref, ok := strings.CutPrefix(field, "branch refs/heads/"); ok {
				current.branch = ref
			}
		}
		if current.branch != "" && strings.Contains(current.path, filepath.Join(".komodo", "wt")) && exists(current.path) {
			found = append(found, current)
		}
	}
	return found
}

// restoreFlips puts BACKLOG.md back to HEAD when every change in it is a status token remote already holds.
func restoreFlips(root, remote string) bool {
	path, err := backlog.Find(root)
	if err != nil {
		return false
	}
	name := filepath.ToSlash(rel(root, path))
	head, err := gitRaw(root, "show", "HEAD:"+name)
	if err != nil {
		return false
	}
	merged, err := gitRaw(root, "show", remote+":"+name)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) == head {
		return false
	}
	current, before, after := backlog.Parse(string(data)), backlog.Parse(head), backlog.Parse(merged)
	reverted := string(data)
	for _, group := range current.Groups {
		for _, task := range group.Tasks {
			old, ok := before.Task(task.ID)
			if !ok {
				return false
			}
			if old.Status == task.Status {
				continue
			}
			if landed, ok := after.Task(task.ID); !ok || landed.Status != task.Status {
				return false
			}
			if reverted, err = backlog.SetStatus(reverted, task.ID, old.Status); err != nil {
				return false
			}
		}
	}
	if reverted != head {
		return false
	}
	return os.WriteFile(path, []byte(head), 0o644) == nil
}

// gitRaw runs git in root and returns its output untrimmed, for a file's exact bytes.
func gitRaw(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return string(out), err
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
