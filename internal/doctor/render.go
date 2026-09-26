package doctor

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/install"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/toolkit"
)

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
	for _, host := range mount.Active() {
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
			// A scoped skill loads only in its role's session, so it is no part of the always-on context.
			if change.Project && !change.Scoped && filepath.Base(change.Path) == "SKILL.md" {
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
		for _, action := range host.Plan.Drift() {
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
		for _, action := range host.Plan.Drift() {
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

// checkHookBinary reports an installed host file whose guard hook names a binary that does not exist.
func checkHookBinary(root string, rendered []renderedHost) []Problem {
	var problems []Problem
	for _, host := range rendered {
		if host.Err != nil {
			continue
		}
		for _, change := range host.Plan.Changes {
			if change.Remove || len(install.HookBinaries(change.Body)) == 0 {
				continue
			}
			installed, err := os.ReadFile(change.Path)
			if err != nil {
				continue
			}
			for _, binary := range install.HookBinaries(installed) {
				resolved := binary
				if !filepath.IsAbs(resolved) {
					resolved = filepath.Join(root, resolved)
				}
				if _, err := os.Stat(resolved); err != nil {
					where := change.Path
					if rel, err := filepath.Rel(root, change.Path); err == nil && !strings.HasPrefix(rel, "..") {
						where = rel
					}
					problems = append(problems, Problem{"hook", where,
						fmt.Sprintf("the guard hook runs %s, which does not exist; run komodo install", binary)})
				}
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

// tokens is the rough token count of a byte length.
func tokens(size int) int { return (size + 3) / 4 }
