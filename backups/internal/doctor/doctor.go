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

	"komodo/internal/backlog"
	"komodo/internal/fsx"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/mount"
	"komodo/internal/plugin"
	"komodo/internal/pr"
	"komodo/internal/release"
	"komodo/internal/repo"
	"komodo/internal/toolkit"
)

// Budgets, in tokens for context; a standard's own cap is internal/harness.CapStandard.
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
	// RepoOnly turns machine-scoped problems, such as a stale global layer, into warnings.
	RepoOnly bool
	// Warn receives each note that never fails a check, such as what the forge's plan does not offer.
	Warn func(note string)
}

// Run walks every check and returns what it found.
func Run(root string, options Options) ([]Problem, error) {
	var problems []Problem
	rendered := renderInstalled(root, pinLocalDown)
	problems = append(problems, checkReferences(root)...)
	problems = append(problems, checkRoles(root)...)
	problems = append(problems, checkBuilderTier(root)...)
	problems = append(problems, checkLeaks(root)...)
	problems = append(problems, checkBudgets(root, renderActive(root))...)
	for _, problem := range checkDrift(rendered, renderInstalled(root, pinLocalUp)) {
		if options.RepoOnly && problem.Check == checkGlobal {
			if options.Warn != nil {
				options.Warn(fmt.Sprintf("%s: %s", problem.Where, problem.Detail))
			}
			continue
		}
		problems = append(problems, problem)
	}
	problems = append(problems, checkStaleSkills(rendered)...)
	problems = append(problems, checkHookBinary(root, rendered)...)
	problems = append(problems, checkProfileDrift(root)...)
	problems = append(problems, checkPromises(root)...)
	problems = append(problems, checkGitattributes(root)...)
	problems = append(problems, checkPins(root)...)
	problems = append(problems, checkPlugins(root)...)
	problems = append(problems, checkOverlay(mount.OverlayPath())...)
	problems = append(problems, checkConfig(root)...)
	problems = append(problems, checkWorkflows(root)...)
	problems = append(problems, checkLegacyBacklog(root)...)
	problems = append(problems, checkStalledBacklog(root, now())...)
	if options.Warn != nil {
		for _, note := range FlatBacklogFiles(root) {
			options.Warn(note)
		}
	}
	if !options.NoGit {
		problems = append(problems, checkAGENTSTracked(root)...)
		if options.Warn != nil {
			for _, note := range Leftovers(root) {
				options.Warn(note)
			}
		}
		found, err := checkGit(root)
		if err != nil {
			return problems, err
		}
		problems = append(problems, found...)
	}
	if options.Remote {
		defaultBranch := base(root)
		problems = append(problems, CheckRulesets(root, defaultBranch, pr.Run)...)
		problems = append(problems, CheckHeadBranches(root, pr.Run)...)
		problems = append(problems, CheckEpics(root, defaultBranch, pr.Run)...)
		problems = append(problems, CheckDefaultBacklog(root, defaultBranch, git.Run)...)
		if options.Warn != nil {
			for _, note := range ForgeNotes(root, pr.Run) {
				options.Warn(note)
			}
		}
	}
	return problems, nil
}

// HostLeftovers lists what an installed host's user settings carry from a retired setup; it never fails a check.
func HostLeftovers(root string) []string {
	var notes []string
	for _, host := range mount.Active() {
		if host.Leftovers != nil && host.Installed != nil && host.Installed(root) {
			notes = append(notes, host.Leftovers()...)
		}
	}
	return notes
}

// checkLegacyBacklog fails when a legacy backlog file sits at the repo's root or under docs/, since
// only docs/backlog/ group files are read; the fix is running the migrate command.
func checkLegacyBacklog(root string) []Problem {
	path, err := backlog.Find(root)
	if err != nil {
		return nil
	}
	return []Problem{{"legacy-backlog", rel(root, path), "still holds tasks; run `komodo migrate` and remove it"}}
}

// checkStaleSkills names each installed SKILL.md git tracks whose body differs from the binary's own
// copy, since install leaves a tracked file alone and never refreshes it on its own.
func checkStaleSkills(rendered []renderedHost) []Problem {
	var problems []Problem
	for _, host := range rendered {
		if host.Err != nil {
			continue
		}
		for _, action := range host.Plan.Actions() {
			if !action.Tracked || filepath.Base(action.Path) != "SKILL.md" {
				continue
			}
			problems = append(problems, Problem{"skills", action.Path,
				"git tracks this stale copy; untrack it (git rm --cached) and run komodo install"})
		}
	}
	return problems
}

// checkAGENTSTracked fails when AGENTS.md exists but git does not track it, since a harness worktree
// cut from a branch never receives a file git never committed.
func checkAGENTSTracked(root string) []Problem {
	path := filepath.Join(root, "AGENTS.md")
	if !exists(path) {
		return nil
	}
	if _, err := git.Run(root, "ls-files", "--error-unmatch", "--", "AGENTS.md"); err != nil {
		return []Problem{{"leftovers", "AGENTS.md", "is not tracked by git, so a harness worktree never receives it"}}
	}
	return nil
}

// checkPlugins reports each plugin manifest that did not load and a machine enable file that did not parse.
func checkPlugins(root string) []Problem {
	var problems []Problem
	enabled, err := plugin.Enabled()
	if err != nil {
		problems = append(problems, Problem{"plugins", plugin.EnabledPath(), err.Error()})
	}
	_, malformed := plugin.Load(root, enabled)
	for _, found := range malformed {
		problems = append(problems, Problem{"plugins", found.Where, found.Detail})
	}
	return problems
}

// PluginStates lists each plugin type and its plugins, none of which 1.0 runs (decision 0013); it never fails a check.
func PluginStates(root string) []string {
	enabled, _ := plugin.Enabled()
	plugins, _ := plugin.Load(root, enabled)
	var notes []string
	for _, kind := range plugin.Types {
		listed := false
		for _, loaded := range plugins {
			if loaded.Type != kind {
				continue
			}
			state := "disabled"
			if loaded.Enabled {
				state = "enabled here, but 1.0 runs no plugin"
			}
			notes = append(notes, fmt.Sprintf("plugin %s %s: %s", kind, loaded.Name, state))
			listed = true
		}
		if !listed {
			notes = append(notes, fmt.Sprintf("plugin %s: disabled, none installed", kind))
		}
	}
	return notes
}

// StrayWorktrees names each linked worktree that pins a branch with the one-line fix, and each one
// parked outside .komodo/wt by its path and tracked branch; it never fails a check.
func StrayWorktrees(root string) []string {
	worktrees, err := git.Worktrees(root)
	if err != nil {
		return nil
	}
	var notes []string
	var parked string
	for index, current := range worktrees {
		if index == 0 {
			parked = filepath.Join(current.Path, ".komodo", "wt") + string(filepath.Separator)
			continue
		}
		switch {
		case current.Branch != "":
			notes = append(notes, current.Path+" pins "+current.Branch+"; git -C "+current.Path+" switch --detach frees it")
		case !strings.HasPrefix(filepath.Clean(current.Path), parked):
			notes = append(notes, current.Path+" on branch "+current.Tracked)
		}
	}
	return notes
}

// CheckDefaultBacklog fetches origin's default branch and reports each backlog path it holds, since only
// an epic branch carries one.
func CheckDefaultBacklog(root, defaultBranch string, gitRun GitRunner) []Problem {
	ref := "origin/" + defaultBranch
	if _, err := gitRun(root, "fetch", "--quiet", "origin", defaultBranch); err != nil {
		return []Problem{{"leftovers", ref, "could not fetch: " + err.Error()}}
	}
	var problems []Problem
	for _, path := range lines(gitRun(root, "ls-tree", "-r", "--name-only", ref, "--", backlog.GroupFilesDir)) {
		problems = append(problems, Problem{"leftovers", path,
			fmt.Sprintf("is on %s; run `komodo sync` to open the cleanup pull request", ref)})
	}
	return problems
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
	BypassActors []struct {
		ActorType string `json:"actor_type"`
	} `json:"bypass_actors"`
}

// rulesetsUnoffered matches only the forge's plan-upgrade refusal, not a bare HTTP 403 a missing scope also returns.
var rulesetsUnoffered = regexp.MustCompile(`Upgrade to GitHub Pro`)

// CheckRulesets reports a default branch no bypass-free ruleset protects, or an active ruleset reaching past it,
// which blocks every group branch's push. A forge offering no rulesets is left to ForgeNotes.
func CheckRulesets(root, defaultBranch string, run pr.Runner) []Problem {
	out, err := run(root, "api", "repos/{owner}/{repo}/rulesets")
	if err != nil {
		if rulesetsUnoffered.MatchString(err.Error()) {
			return nil
		}
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
		reaches := false
		for _, ref := range item.Conditions.RefName.Include {
			reaches = reaches || ref == "~ALL"
			if ref == "~DEFAULT_BRANCH" || ref == "refs/heads/"+defaultBranch {
				reaches = true
				continue
			}
			problems = append(problems, Problem{"ruleset", item.Name,
				fmt.Sprintf("includes %s; scope it to refs/heads/%s so a group branch can be pushed", ref, defaultBranch)})
		}
		if reaches && len(item.BypassActors) > 0 {
			problems = append(problems, Problem{"ruleset", item.Name,
				fmt.Sprintf("has %d bypass actor(s); remove them so only a human merge of a pull request changes %s",
					len(item.BypassActors), defaultBranch)})
			continue
		}
		covered = covered || reaches
	}
	if !covered {
		problems = append(problems, Problem{"ruleset", defaultBranch,
			fmt.Sprintf("no active ruleset without bypass actors covers refs/heads/%s; the forge is the boundary the guard cannot be",
				defaultBranch)})
	}
	return problems
}

// CheckHeadBranches reports a forge that keeps a pull request's head branch after it merges.
// A caller the forge shows no setting to reads as unknown, not as a problem.
func CheckHeadBranches(root string, run pr.Runner) []Problem {
	out, err := run(root, "api", "repos/{owner}/{repo}")
	if err != nil {
		return []Problem{{"forge", "gh", "could not read the repo: " + err.Error()}}
	}
	var repo struct {
		DeleteBranchOnMerge *bool `json:"delete_branch_on_merge"`
	}
	if json.Unmarshal([]byte(out), &repo) != nil {
		return []Problem{{"forge", "gh", "the repo did not parse"}}
	}
	if repo.DeleteBranchOnMerge == nil || *repo.DeleteBranchOnMerge {
		return nil
	}
	return []Problem{{"forge", "delete_branch_on_merge",
		"head branches are kept after a merge; turn on automatic deletion of head branches in the repo's settings"}}
}

// ForgeNotes names what the forge's plan does not offer, rulesets and draft pull requests; it never fails a check.
func ForgeNotes(root string, run pr.Runner) []string {
	_, err := run(root, "api", "repos/{owner}/{repo}/rulesets")
	if err == nil || !rulesetsUnoffered.MatchString(err.Error()) {
		return nil
	}
	return []string{
		"the forge offers no rulesets here, so only the human merge keeps the default branch behind a pull request",
		"the forge offers no draft pull requests here, so each opens labelled status/wip",
	}
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

// resolves reports whether a referenced path exists beside the file, at the root, under komodo, or as a starter's name.
func resolves(root, dir, target string) bool {
	if strings.ContainsAny(target, "<>*") || starterName(root, filepath.Base(target)) {
		return true
	}
	for _, base := range []string{dir, root, filepath.Join(root, "komodo"), filepath.Join(root, "internal")} {
		if _, err := os.Stat(filepath.Join(base, target)); err == nil {
			return true
		}
	}
	return false
}

// starterName reports whether the starter templates define a file of this name.
func starterName(root, name string) bool {
	found := false
	_ = filepath.WalkDir(filepath.Join(root, "templates", "project"), func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == name {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
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
		// A role's lens sections carry no frontmatter and name no role; a role's body injects them.
		if strings.HasSuffix(path, ".lenses.md") {
			continue
		}
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

// checkBuilderTier reports a mode profile under komodo/profiles that puts the builder on the light tier.
func checkBuilderTier(root string) []Problem {
	tree := toolkit.FS(root)
	names, _ := fs.Glob(tree, "profiles/*.json")
	var problems []Problem
	for _, name := range names {
		data, err := fs.ReadFile(tree, name)
		if err != nil {
			continue
		}
		var parsed struct {
			Roles map[string]struct {
				Tier string `json:"tier"`
			} `json:"roles"`
		}
		if json.Unmarshal(data, &parsed) == nil && parsed.Roles["builder"].Tier == "light" {
			problems = append(problems, Problem{"profiles", path.Join("komodo", name),
				"the builder runs the light tier; a builder runs standard or heavy"})
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

// checkGitattributes reports if .gitattributes is missing or lacks eol=lf for all files.
func checkGitattributes(root string) []Problem {
	path := filepath.Join(root, ".gitattributes")
	data, err := os.ReadFile(path)
	if err != nil {
		return []Problem{{"gitattributes", ".gitattributes", "add: * text=auto eol=lf"}}
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || fields[0] != "*" {
			continue
		}
		if contains(fields[1:], "eol=lf") {
			return nil
		}
	}
	return []Problem{{"gitattributes", ".gitattributes", "add: * text=auto eol=lf"}}
}

// checkOverlay reports a machine overlay its readers refuse: bad JSON, an unknown field, or a wrong type.
func checkOverlay(path string) []Problem {
	if _, err := mount.DecodeOverlayFile(path); err != nil {
		return []Problem{{"overlay", path, "a command that reads it stops until it decodes: " + err.Error()}}
	}
	return nil
}

// checkConfig reports a present repo config its reader refuses, naming the file and what stops.
func checkConfig(root string) []Problem {
	var problems []Problem
	for _, config := range []struct {
		rel, effect string
		into        any
	}{
		{filepath.Join(".komodo", "policy.json"), "the guard skips it, so its refs and paths go unenforced", &guard.Policy{}},
		{repo.LabelsFile, "pull request labels stop until it decodes", &repo.Labels{}},
	} {
		path := filepath.Join(root, config.rel)
		if _, err := fsx.ReadStrictJSON(path, config.into); err != nil {
			problems = append(problems, Problem{"config", path, config.effect + ": " + err.Error()})
		}
	}
	return problems
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
	found, err := checkChangelog(root, lines(git.Run(root, "tag", "--list")))
	return append(problems, found...), err
}

// checkChangelog reports each drift between the changelog, the tags, and the shipped groups' versions.
func checkChangelog(root string, tags []string) ([]Problem, error) {
	changelog, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return nil, err
	}
	var shipped []string
	if parsed, err := backlog.LoadRoot(root); err == nil {
		shipped = release.ShippedVersions(parsed)
	}
	var problems []Problem
	for _, drift := range release.Check(changelog, tags, shipped) {
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
